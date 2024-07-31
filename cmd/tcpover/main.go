package main

import (
	"flag"
	"log"
	"net/http"
)

var debug bool

func main() {
	runAsConnector := flag.Bool("c", false, "as connector")
	runAsAgent := flag.Bool("a", false, "as agent")
	runAsServer := flag.Bool("s", false, "as server")
	connectLocal := flag.Bool("ca", false, "connect local agent mode")
	listenAddr := flag.String("l", "", "Listen address [SC]")
	serverEndpoint := flag.String("e", "", "Server endpoint. [C]")
	uid := flag.String("d", "", "The destination uid to [C]")
	flag.Parse()

	if !*runAsServer && !*runAsConnector && !*runAsAgent {
		log.Fatalln("must be run as one mode")
	}

	if *runAsServer && *listenAddr == "" {
		log.Fatalln("server must set listen addr")
	}

	if *runAsConnector && (*serverEndpoint == "" || *uid == "") {
		log.Fatalln("connector must set server endpoint and destination")
	}

	if *runAsAgent && (*serverEndpoint == "" || *uid == "") {
		log.Fatalln("agent must set server endpoint and destination and listen addr")
	}

	if *runAsServer {
		s := NewServer()
		if err := http.ListenAndServe(*listenAddr, s); err != nil {
			log.Fatalln(err)
		}
		return
	}

	if *runAsConnector {
		c := NewClient(*serverEndpoint)
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
		c := NewClient(*serverEndpoint)
		if err := c.Serve(*uid); err != nil {
			log.Fatalln(err)
		}
		return
	}
}
