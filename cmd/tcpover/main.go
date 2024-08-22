package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/tiechui1994/tool/cmd/tcpover/over"
)

var debug bool

func main() {
	runAsConnector := flag.Bool("c", false, "as connector")
	runAsAgent := flag.Bool("a", false, "as agent")
	runAsServer := flag.Bool("s", false, "as server")

	listenAddr := flag.String("l", "", "Listen address [SC]")
	serverEndpoint := flag.String("e", "", "Server endpoint. [C]")
	name := flag.String("name", "", "name [SC]")
	remoteName := flag.String("remoteName", "", "remoteName. [C]")
	remoteAddr := flag.String("remoteAddr", "", "remoteAddr. [C]")

	flag.Parse()

	if !*runAsServer && !*runAsConnector && !*runAsAgent {
		log.Fatalln("must be run as one mode")
	}

	if *runAsServer && *listenAddr == "" {
		log.Fatalln("server must set listen addr")
	}

	if *runAsConnector && (*serverEndpoint == "" || *remoteName == "" || *remoteAddr == "") {
		log.Fatalln("agent must set server endpoint and remoteName, remoteAddr")
	}

	if *runAsAgent && (*serverEndpoint == "" || *name == "" || *remoteName == "") {
		log.Fatalln("agent must set server endpoint and remoteName, remoteName")
	}

	if *runAsServer {
		s := over.NewServer()
		if err := http.ListenAndServe(*listenAddr, s); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if *runAsConnector {
		c := over.NewClient(*serverEndpoint, nil)
		if err := c.Std(*remoteName, *remoteAddr); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if *runAsAgent {
		c := over.NewClient(*serverEndpoint, nil)
		if err := c.ServeAgent(*name, *listenAddr, *remoteName); err != nil {
			log.Fatalln(err)
		}
		return
	}
}
