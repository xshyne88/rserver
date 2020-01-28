package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/rs/xid"
	pb "github.com/xshyne88/rserver/proto"
	"google.golang.org/grpc"
)

func inputCommand(input chan<- string) {
	for {
		var command string
		_, err := fmt.Scanf("%s", &command)
		if err != nil {
			panic(err)
		}
		input <- command
	}
}

func main() {
	log.Println("Server started.")
	log.Println("---------------")
	ln, err := net.Listen("tcp", ":3003")
	if err != nil {
		log.Fatalf("error listening %s", err)
	}
	defer ln.Close()

	for {
		log.Println("waiting for incoming TCP Connections")
		incoming, err := ln.Accept()
		if err != nil {
			log.Fatalf("error accepting connection %s", err)
		}

		incomingConn, err := yamux.Client(incoming, yamux.DefaultConfig())
		if err != nil {
			log.Fatalf("couldn't create yamux %s", err)
		}
		log.Println("TCP Connection Success starting grpcServer")
		log.Println("---------------")
		var conn *grpc.ClientConn
		conn, err = grpc.Dial(":7777", grpc.WithInsecure(),
			grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
				return incomingConn.Open()
			}),
		)
		if err != nil {
			log.Fatalf("did not connect: %s", err)
		}

		input := make(chan string)
		go inputCommand(input)
		go handleConn(conn, input)
	}
}

func handleConn(conn *grpc.ClientConn, input <-chan string) {
	defer conn.Close()
	client := pb.NewCommandIssuerClient(conn)
	stream, err := client.SendCommands(context.Background())
	checkError(err, "could not start CommandIssuer:SendCommands")

	ctx := stream.Context()
	done := make(chan bool)

	for {
		select {
		case i := <-input:
			req := pb.CommandRequest{Id: xid.New().String(), CommandType: i}
			if err := stream.Send(&req); err != nil {
				log.Fatalf("can not send %v", err)
			}
		default:
		}
	}

	go func() {
		for i := 0; i < 15; i++ {
			time.Sleep(time.Millisecond * 1200)
			req := pb.CommandRequest{Id: xid.New().String(), CommandType: getRandomCommand()}
			if err := stream.Send(&req); err != nil {
				log.Fatalf("can not send %v", err)
			}
			log.Printf("%s command sent", req.CommandType)
		}
		if err := stream.CloseSend(); err != nil {
			log.Println(err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			log.Println(err)
		}
		close(done)
	}()

	<-done
	log.Println("exiting")
}

func checkError(err error, resp string) {
	if err != nil {
		log.Fatal(resp, err)
	}
}

func getRandomCommand() string {
	num := rand.Intn(3)

	switch num {
	case 1:
		return "Restart"
	case 2:
		return "Shut Down"
	default:
		return "Start Up"
	}
}
