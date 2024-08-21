package main

import (
	"context"
	"log"
	"net"

	"github.com/tiechui1994/tool/cmd/tcpover/mux"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
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
}
