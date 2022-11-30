package util

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CodeError struct {
	Method  string
	URL     string
	Code    int
	Message string
}

func (err CodeError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("%v %q : status: %v", err.Method, err.URL,
			http.StatusText(err.Code))
	} else  {
		return fmt.Sprintf("%v %q : (status:%v, message:%v)", err.Method, err.URL,
			http.StatusText(err.Code), err.Message)
	}
}

func Request(method, u string, opts ...Option) (json.RawMessage, http.Header, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	try := 0
try:
	request, _ := http.NewRequestWithContext(options.ctx, method, u, options.body)
	for k, v := range options.header {
		request.Header.Set(k, v)
	}

	request.Header.Set("user-agent", UserAgent())

	response, err := client.Do(request)
	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, nil, err
	}

	if options.BeforeRequest != nil {
		options.BeforeRequest(request)
	}

	raw, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}

	if options.AfterResponse != nil {
		options.AfterResponse(response)
	}

	if response.StatusCode >= 400 {
		return raw, response.Header, CodeError{method,u, response.StatusCode, string(raw)}
	}

	return raw, response.Header, err
}

func POST(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = Request(http.MethodPost, u, opts...)
	return raw, err
}

func PUT(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = Request(http.MethodPut, u, opts...)
	return raw, err
}

func GET(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = Request(http.MethodGet, u, opts...)
	return raw, err
}

func DELETE(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = Request(http.MethodDelete, u, opts...)
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
		return conn, raw, CodeError{http.MethodGet,u, response.StatusCode,""}
	}

	return conn, raw, nil
}

func File(u, method string, opts ...Option) (io io.Reader, err error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	try := 0
try:
	request, _ := http.NewRequestWithContext(options.ctx, method, u, options.body)
	for k, v := range options.header {
		request.Header.Set(k, v)
	}
	request.Header.Set("user-agent", UserAgent())

	response, err := client.Do(request)
	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return io,  CodeError{method,u, response.StatusCode,""}
	}

	return response.Body, err
}
