package main

import (
	"encoding/base64"
	"encoding/hex"
	"flag"
	"log"
	"os"

	"github.com/tiechui1994/tool/bencode"
	"github.com/tiechui1994/tool/util"
)

func main() {
	name := flag.String("n", "", "upload name")
	flag.Parse()

	file, err := os.Open(*name)
	if err != nil {
		log.Fatalf("Open: %v", err)
	}

	infoHash, torrentBase64, err := bencode.FileTorrentCompute(file)
	if err != nil {
		log.Fatalf("FileTorrentCompute: %v", err)
	}

	torrentFile, err := base64.RawStdEncoding.DecodeString(torrentBase64)

	header := map[string]string{
		"infoHash": infoHash,
		"torrentFile": hex.EncodeToString(torrentFile),
	}

	util.POST("", util.WithHeader(header), util.WithBody())

}