package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/over/mux"
)

func init() {
	fd, _ := os.Create("./www.log")
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	log.SetOutput(fd)
}

func main() {
	go func() {
		l, err := net.Listen("tcp", ":9999")
		if err != nil {
			log.Fatalf("Listen: %v", err)
		}
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}
			ctx, _ := context.WithCancel(context.Background())
			worker, err := mux.NewServerWorker(ctx, mux.NewDispatcher(), &mux.Link{
				Reader: conn,
				Writer: conn,
			})
			if err != nil {
				log.Fatalf("worker: %v, %v", worker, err)
			}
		}
	}()

	time.Sleep(time.Second)
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	link := &mux.Link{Reader: conn, Writer: conn}
	client := mux.NewClientWorker(link)

	l, err := net.Listen("tcp", ":2222")
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("%v", err)
			continue
		}
		ctx := context.WithValue(context.Background(), "destination", mux.Destination{
			Network: mux.TargetNetworkTCP,
			Address: "127.0.0.1:22",
		})
		client.Dispatch(ctx, &mux.Link{Reader: conn, Writer: conn})
	}
}
