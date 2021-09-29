package util

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tiechui1994/tool/log"
)

type CodeError int

func (err CodeError) Error() string {
	return http.StatusText(int(err))
}

func request(method, u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
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
			reader = bytes.NewReader(bin)
		}
	}

	request, _ := http.NewRequest(method, u, reader)
	for k, v := range header {
		request.Header.Set(k, v)
	}

	request.Header.Set("user-agent", UserAgent())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return raw, err
	}

	if logprefix {
		log.Infoln("%v %v %v", method, request.URL.Path, request.Cookies())
	}

	raw, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return raw, err
	}

	if logsufix && len(raw) > 0 {
		log.Infoln("%v %v %v", method, request.URL.Path, response.Cookies())
	}

	if response.StatusCode >= 400 {
		log.Errorln("path:%v code:%v data:%v", request.URL.Path, response.StatusCode, string(raw))
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

func File(u, method string, body io.Reader, header map[string]string) (io io.Reader, err error) {
	request, _ := http.NewRequest(method, u, body)
	if header != nil {
		for k, v := range header {
			request.Header.Set(k, v)
		}
	}
	request.Header.Set("user-agent", UserAgent())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return io, err
	}

	if response.StatusCode != 200 {
		return io, CodeError(response.StatusCode)
	}

	return response.Body, err
}
