package util

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tiechui1994/tool/log"
)

type CodeError int

func (err CodeError) Error() string {
	return http.StatusText(int(err))
}

func Request(method, u string, body interface{}, header map[string]string) (json.RawMessage, http.Header, error) {
	var reader io.Reader
	if body != nil {
		switch body := body.(type) {
		case io.Reader:
			reader = body
		case string:
			reader = strings.NewReader(body)
		case []byte:
			reader = bytes.NewReader(body)
		default:
			bin, _ := json.Marshal(body)
			log.Infoln("body:%s", string(bin))
			reader = bytes.NewReader(bin)
		}
	}

	request, _ := http.NewRequest(method, u, reader)
	for k, v := range header {
		request.Header.Set(k, v)
	}

	request.Header.Set("user-agent", UserAgent())

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if len(requestInterceptor) > 0 {
		for _, f := range requestInterceptor {
			f(request)
		}
	}

	raw, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}

	if len(responseInterceptor) > 0 {
		for _, f := range responseInterceptor {
			f(response)
		}
	}

	if response.StatusCode >= 400 {
		log.Errorln("path:%v code:%v data:%v", request.URL.Path, response.StatusCode, string(raw))
		return raw, response.Header, CodeError(response.StatusCode)
	}

	return raw, response.Header, err
}

func POST(u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	raw, _, err = Request("POST", u, body, header)
	return raw, err
}

func PUT(u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	raw, _, err = Request("PUT", u, body, header)
	return raw, err
}

func GET(u string, header map[string]string) (raw json.RawMessage, err error) {
	raw, _, err = Request("GET", u, nil, header)
	return raw, err
}

func DELETE(u string, header map[string]string) (raw json.RawMessage, err error) {
	raw, _, err = Request("DELETE", u, nil, header)
	return raw, err
}

func SOCKET(u string, header map[string]string) (conn *websocket.Conn, raw json.RawMessage, err error) {
	dailer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		NetDialContext:    (&net.Dialer{}).DialContext,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: true,
		Jar:               client.Jar,
	}

	head := make(http.Header)
	for key, val := range header {
		head.Set(key, val)
	}
	head.Set("user-agent", UserAgent())

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

func File(u, method string, body io.Reader, header map[string]string) (io io.Reader, err error) {
	request, _ := http.NewRequest(method, u, body)
	if header != nil {
		for k, v := range header {
			request.Header.Set(k, v)
		}
	}
	request.Header.Set("user-agent", UserAgent())

	response, err := client.Do(request)
	if err != nil {
		return io, err
	}

	if response.StatusCode != 200 {
		return io, CodeError(response.StatusCode)
	}

	return response.Body, err
}
