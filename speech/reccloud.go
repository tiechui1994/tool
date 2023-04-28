package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/tiechui1994/tool/util"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func uploadFile(fields map[string]string) (reader io.Reader, contentType string, totalSize int) {
	boundary := randomBoundary()
	contentType = fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	parts := make([]io.Reader, 0)
	CRLF := "\r\n"

	fieldBoundary := "--" + boundary + CRLF

	for k, v := range fields {
		parts = append(parts, strings.NewReader(fieldBoundary))
		totalSize += len(fieldBoundary)

		header := fmt.Sprintf(`Content-Disposition: form-data; name="%s"`, escapeQuotes(k))
		parts = append(
			parts,
			strings.NewReader(header+CRLF+CRLF),
			strings.NewReader(v),
			strings.NewReader(CRLF),
		)
		totalSize += len(header) + 2*len(CRLF) + len(v) + len(CRLF)
	}

	finishBoundary := "--" + boundary + "--" + CRLF
	parts = append(parts, strings.NewReader(finishBoundary))
	totalSize += len(finishBoundary)

	return io.MultiReader(parts...), contentType, totalSize
}

func HMACSha1(key, method, md5, _type, date string, ossHeader []string, resource string) string {
	values := []string{
		method, md5, _type, date,
	}
	values = append(values, ossHeader...)
	values = append(values, resource)

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(strings.Join(values, "\n")))
	res := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return res
}

func Upload(src string) {
	fd, err := os.Open(src)
	if err != nil {
		return
	}

	_, name := filepath.Split(fd.Name())
	reader, contentType, totalSize := uploadFile(map[string]string{
		"task_type":   "201",
		"filenames[]": name,
	})

	header := map[string]string{
		"content-type":   contentType,
		"content-length": fmt.Sprintf("%v", totalSize),
		"origin":         "https://reccloud.cn",
		"x-api-key":      "wxonf9nu5elogwxzp",
	}

	u := "https://aw.aoscdn.com/tech/authorizations/oss"
	raw, err := util.POST(u, util.WithBody(reader), util.WithHeader(header))
	if err != nil {
		return
	}

	var response struct {
		Status int `json:"status"`
		Data   struct {
			Accelerate string
			Bucket     string
			Region     string
			Endpoint   string
			Callback   struct {
				Url  string `json:"url"`
				Type string `json:"type"`
				Body string `json:"body"`
			} `json:"callback"`
			Credential struct {
				AccessKeyID     string `json:"access_key_id"`
				AccessKeySecret string `json:"access_key_secret"`
				Expiration      int    `json:"expiration"`
				SecurityToken   string `json:"security_token"`
			} `json:"credential"`
			Objects map[string]string `json:"objects"`
		} `json:"data"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return
	}

	for _, task := range response.Data.Objects {
		endpoint := fmt.Sprintf("https://%v.%v/%v", response.Data.Bucket,
			response.Data.Endpoint, task)
		date := time.Now().Format("Mon, 02 Jan 2006 15:04:05 GMT")
		ossHeader := []string{
			"x-oss-date:" + date,
			"x-oss-security-token:" + response.Data.Credential.SecurityToken,
			"x-oss-user-agent: aliyun-sdk-js/6.17.1 Chrome 112.0.0.0 on Windows 10 64-bit",
		}

		header := map[string]string{
			"origin":               "https://reccloud.cn",
			"x-oss-date":           date,
			"x-oss-security-token": response.Data.Credential.SecurityToken,
			"authorization": "OSS " + response.Data.Credential.AccessKeyID + ":" +
				HMACSha1(response.Data.Credential.AccessKeySecret,
					"PUT", "11", "audio/mpeg", date, ossHeader,
					"/"+response.Data.Endpoint),
		}
		raw, err = util.POST(endpoint+"?uploads=", util.WithBody(reader), util.WithHeader(header))
		if err != nil {
			return
		}

		var uploads struct {
			InitiateMultipartUploadResult struct {
				UploadId string `xml:"UploadId"`
				Key      string `xml:"Key"`
				Bucket   string `xml:"Bucket"`
			} `xml:"InitiateMultipartUploadResult"`
		}
		err = xml.Unmarshal(raw, &uploads)
		if err != nil {
			return
		}

		//

		util.POST(endpoint + "endpoint")
	}
}
