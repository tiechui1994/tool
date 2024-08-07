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
	runAsProxy := flag.Bool("p", false, "as proxy")

	connectLocal := flag.Bool("ca", false, "connect local agent mode")
	listenAddr := flag.String("l", "", "Listen address [SC]")
	serverEndpoint := flag.String("e", "", "Server endpoint. [C]")
	uid := flag.String("d", "", "The destination uid to [C]")
	flag.Parse()

	if !*runAsServer && !*runAsConnector && !*runAsAgent && !*runAsProxy {
		log.Fatalln("must be run as one mode")
	}

	if *runAsServer && *listenAddr == "" {
		log.Fatalln("server must set listen addr")
	}

	if *runAsConnector && (*serverEndpoint == "" || *uid == "") {
		log.Fatalln("connector must set server endpoint and destination")
	}

	if *runAsAgent && (*serverEndpoint == "" || *uid == "") {
		log.Fatalln("agent must set server endpoint and destination")
	}

	if *runAsProxy && (*serverEndpoint == "" || *listenAddr == "") {
		log.Fatalln("agent must set server endpoint  and listen addr")
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
		if *connectLocal {
			if err := c.Tcp(*uid); err != nil {
				log.Fatalln(err)
			}
		} else {
			if err := c.Std(*uid); err != nil {
				log.Fatalln(err)
			}
		}
		return
	}

	if *runAsAgent {
		c := over.NewClient(*serverEndpoint, nil)
		if err := c.ServeAgent(*uid); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if *runAsProxy {
		c := over.NewClient(*serverEndpoint, nil)
		if err := c.ServeProxy(*listenAddr); err != nil {
			log.Fatalln(err)
		}
		return
	}
}
