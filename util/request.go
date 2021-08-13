package util

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type CodeError int

func (err CodeError) Error() string {
	return http.StatusText(int(err))
}

func request(method, u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	var reader io.Reader
	if body != nil {
		switch body.(type) {
		case io.Reader:
			reader = body.(io.Reader)
		case string:
			reader = strings.NewReader(body.(string))
		case []byte:
			reader = bytes.NewReader(body.([]byte))
		default:
			bin, _ := json.Marshal(body)
			reader = bytes.NewReader(bin)
		}
	}

	request, _ := http.NewRequest(method, u, reader)
	if header != nil {
		for k, v := range header {
			request.Header.Set(k, v)
		}
	}

	request.Header.Set("user-agent", UserAgent())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return raw, err
	}

	if Debug {
		log.Println(method, request.URL.Path, request.Cookies())
	}

	raw, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return raw, err
	}

	if Debug && len(raw) > 0 {
		log.Println(method, request.URL.Path, response.Cookies())
	}

	if response.StatusCode >= 400 {
		return raw, CodeError(response.StatusCode)
	}

	return raw, nil
}

func POST(u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	return request("POST", u, body, header)
}

func PUT(u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	return request("PUT", u, body, header)
}

func GET(u string, header map[string]string) (raw json.RawMessage, err error) {
	return request("GET", u, nil, header)
}

func DELETE(u string, header map[string]string) (raw json.RawMessage, err error) {
	return request("DELETE", u, nil, header)
}

func SOCKET(u string, header map[string]string) (conn *websocket.Conn, raw json.RawMessage, err error) {
	dailer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		NetDialContext:    (&net.Dialer{}).DialContext,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: true,
	}

	head := make(http.Header)
	for key, val := range header {
		head.Set(key, val)
	}
	head.Set("user-agent", UserAgent())

	uu, _ := url.Parse(u)
	if cookies := jar.Cookies(uu); len(cookies) > 0 {
		cookie := make([]string, 0, len(cookies))
		for _, v := range cookies {
			cookie = append(cookie, v.String())
		}
		head.Set("cookie", strings.Join(cookie, "; "))
	}

	if strings.HasPrefix(u, "https") {
		u = "wss" + u[5:]
	} else {
		u = "ws" + u[4:]
	}
	conn, response, err := dailer.Dial(u, head)
	if err != nil {
		return nil, nil, err
	}

	raw, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return conn, nil, err
	}

	if response.StatusCode >= 400 {
		return conn, raw, CodeError(response.StatusCode)
	}

	return conn, raw, nil
}

func File(u, method string, body io.Reader, header map[string]string, path string) (err error) {
	fd, err := os.Create(path)
	if err != nil {
		return err
	}

	request, _ := http.NewRequest(method, u, body)
	if header != nil {
		for k, v := range header {
			request.Header.Set(k, v)
		}
	}
	request.Header.Set("user-agent", UserAgent())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return CodeError(response.StatusCode)
	}

	buf := make([]byte, 8192)
	_, err = io.CopyBuffer(fd, response.Body, buf)
	return err
}
