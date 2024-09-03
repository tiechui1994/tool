package bencode

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)


func FileTorrentCompute(file *os.File) (infoHash, torrentBase64 string, err error) {
	var chunkSize = int64(5013504)
	var pieces []byte

	stats, _ := file.Stat()
	fileSize, fileName := stats.Size(), stats.Name()

	offset := int64(0)
	for offset < fileSize  {
		size := chunkSize
		if offset + chunkSize > fileSize {
			size = fileSize - offset
		}

		hash := sha1.New()
		n, err := io.CopyN(hash, file, size)
		if n != size || (err != nil && err != io.EOF) {
			fmt.Println(err == io.EOF, n, size)
			return "", "", fmt.Errorf("chunk size compute failed: %v", err)
		}

		pieces = append(pieces, hash.Sum(nil)...)
		offset += size
	}

	var announce = "wss://wormhole.app/websocket"
	var info = map[string]interface{}{
		"length": fileSize,
		"name": fileName,
		"nonce": "bc938e22ca47931d0529cffc2ab9f861",
		"piece length": chunkSize,
		"pieces": pieces,
		"private": 1,
	}
	var torrent = map[string]interface{}{
		"announce": announce,
		"announce-list": [][]string{
			{announce},
		},
		"created by":    "WebTorrent/0108",
		"creation date": time.Now().Unix(),
		"info":          info,
		"private":       1,
		"url-list":      []string{},
	}

	var buf bytes.Buffer
	err = Marshal(&buf, torrent)
	if err != nil {
		return "", "", err
	}
	torrentBase64 = base64.RawStdEncoding.EncodeToString(buf.Bytes())

	buf.Reset()
	err = Marshal(&buf, torrent)
	if err != nil {
		return "", "", err
	}
	hash := sha1.New()
	_, _ = hash.Write(buf.Bytes())
	infoHash = hex.EncodeToString(hash.Sum(nil))

	return infoHash, torrentBase64, nil
}
