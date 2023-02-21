package weixin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

func randomBoundary() string {
	var buf [8]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		panic(err)
	}
	return "------------------------" + hex.EncodeToString(buf[:])
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// Multipart request has the following structure:
//  POST /upload HTTP/1.1
//  Other-Headers: ...
//  Content-Type: multipart/form-data; boundary=$boundary
//  \r\n
//  --$boundary\r\n      <-request body starts here
//  Content-Disposition: form-data; name="field1"; filename="xyz.img"\r\n
//  Content-Type: application/octet-stream\r\n
//  Content-Length: 4\r\n
//  \r\n
//  $content\r\n
//  --$boundary\r\n
//  Content-Disposition: form-data; name="field2"; filename="pwd.img"\r\n
//  ...
//  --$boundary--\r\n
func uploadFile(fields map[string]interface{}) (reader io.Reader, contentType string, totalSize int, err error) {
	boundary := randomBoundary()
	contentType = fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	parts := make([]io.Reader, 0)
	CRLF := "\r\n"

	fieldBoundary := "--" + boundary + CRLF

	for k, v := range fields {
		parts = append(parts, strings.NewReader(fieldBoundary))
		totalSize += len(fieldBoundary)
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case string:
			header := fmt.Sprintf(`Content-Disposition: form-data; name="%s"`, escapeQuotes(k))
			parts = append(
				parts,
				strings.NewReader(header+CRLF+CRLF),
				strings.NewReader(val),
				strings.NewReader(CRLF),
			)
			totalSize += len(header) + 2*len(CRLF) + len(val) + len(CRLF)
			continue
		case fs.File:
			stat, _ := val.Stat()
			header := strings.Join([]string{
				fmt.Sprintf(`Content-Disposition: form-data; name="%s"; filename="%s"`, escapeQuotes(k), escapeQuotes(stat.Name())),
				fmt.Sprintf(`Content-Type: %s`, "application/octet-stream"),
			}, CRLF)
			parts = append(
				parts,
				strings.NewReader(header+CRLF+CRLF),
				val,
				strings.NewReader(CRLF),
			)
			totalSize += len(header) + 2*len(CRLF) + int(stat.Size()) + len(CRLF)
			continue
		}
	}

	finishBoundary := "--" + boundary + "--" + CRLF
	parts = append(parts, strings.NewReader(finishBoundary))
	totalSize += len(finishBoundary)

	return io.MultiReader(parts...), contentType, totalSize, nil
}
