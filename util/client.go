package util

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type testDataStu struct {
	Raw    json.RawMessage
	Header http.Header
}

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
	} else {
		return fmt.Sprintf("%v %q : (status:%v, message:%v)", err.Method, err.URL,
			http.StatusText(err.Code), err.Message)
	}
}

func Request(method, u string, opts ...Option) (json.RawMessage, http.Header, error) {
	return globalClient.Request(method, u, opts...)
}

func POST(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.POST(u, opts...)
}

func PUT(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.PUT(u, opts...)
}

func GET(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.GET(u, opts...)
}

func DELETE(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.DELETE(u, opts...)
}

func File(u, method string, opts ...Option) (io io.Reader, err error) {
	return globalClient.File(u, method, opts...)
}

func (c *EmbedClient) debugRequest(req *http.Request, now time.Time) {
	uv := req.URL
	method := req.Method

	prefix := uv.Path[strings.LastIndex(uv.Path, "/")+1:]
	key := fmt.Sprintf("%v_%v_%v_REQ.txt", prefix, strings.ToUpper(method), now.Format("150405999"))
	bytes, _ := httputil.DumpRequestOut(req, true)
	_ = ioutil.WriteFile(filepath.Join(c.config.dir, key), bytes, 0644)
}

func (c *EmbedClient) debugResponse(req *http.Request, resp *http.Response, now time.Time) {
	uv := req.URL
	method := req.Method

	prefix := uv.Path[strings.LastIndex(uv.Path, "/")+1:]
	key := fmt.Sprintf("%v_%v_%v_RESP.txt", prefix, strings.ToUpper(method), now.Format("150405999"))
	bytes, _ := httputil.DumpResponse(resp, true)
	_ = ioutil.WriteFile(filepath.Join(c.config.dir, key), bytes, 0644)
}

func (c *EmbedClient) testRequest(req *http.Request) *testDataStu {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	if _, err := os.Stat(testCacheFile); err == nil || !os.IsNotExist(err) {
		var stu testDataStu
		if err = ReadFile(testCacheFile, &stu); err == nil {
			return &stu
		}
	}

	return nil
}

func (c *EmbedClient) testResponse(req *http.Request, data testDataStu) {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	_ = WriteFile(testCacheFile, data)
}

func (c *EmbedClient) Request(method, u string, opts ...Option) (json.RawMessage, http.Header, error) {
	c.init()

	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	try := 0
try:
	if try > 0 && options.randomHost != nil {
		uRL, _ := url.Parse(u)
		uRL.Host = options.randomHost(uRL.Host)
		u = uRL.String()
	}
	request, err := http.NewRequestWithContext(context.Background(), method, u, options.body)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range options.header {
		request.Header.Set(k, v)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", UserAgent())
	}
	if val := request.Header.Get("Content-Length"); val != "" {
		request.ContentLength, _ = strconv.ParseInt(val, 10, 64)
	}
	if c.config.cookieFun != nil {
		c.config.cookieFun.LoadCookie(request)
	}

	if options.proxy != nil {
		transport := c.Transport.(*http.Transport)
		proxy := transport.Proxy
		transport.Proxy = options.proxy
		defer func() {
			transport.Proxy = proxy
		}()
	}

	if options.test {
		if v := c.testRequest(request); v != nil {
			return v.Raw, v.Header, nil
		}
	}

	now := time.Now()
	if options.debug {
		c.debugRequest(request, now)
	}

	response, err := c.Do(request)
	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, nil, err
	}

	if options.beforeRequest != nil {
		options.beforeRequest(request)
	}

	if options.debug {
		c.debugResponse(request, response, now)
	}

	var reader io.Reader
	encoding := response.Header.Get("Content-Encoding")
	switch strings.ToLower(encoding) {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return nil, nil, err
		}
	case "deflate":
		reader = flate.NewReader(reader)
	default:
		reader = response.Body
	}
	raw, err := ioutil.ReadAll(reader)

	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, nil, err
	}

	if options.afterResponse != nil {
		options.afterResponse(response)
	}

	if response.StatusCode >= 400 {
		if try < options.retry {
			try += 1
			time.Sleep(time.Second * time.Duration(try))
			goto try
		}
		if strings.Contains(response.Header.Get("content-type"), "text/html") {
			return raw, response.Header, CodeError{method, u, response.StatusCode, ""}
		}
		return raw, response.Header, CodeError{method, u, response.StatusCode, string(raw)}
	}

	if c.config.cookieFun != nil {
		c.config.cookieFun.SaveCookie(request.URL, response)
	}

	if options.test {
		c.testResponse(request, testDataStu{raw, response.Header})
	}

	return raw, response.Header, err
}

func (c *EmbedClient) POST(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodPost, u, opts...)
	return raw, err
}

func (c *EmbedClient) PUT(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodPut, u, opts...)
	return raw, err
}

func (c *EmbedClient) GET(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodGet, u, opts...)
	return raw, err
}

func (c *EmbedClient) DELETE(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodDelete, u, opts...)
	return raw, err
}

func (c *EmbedClient) File(u, method string, opts ...Option) (io io.Reader, err error) {
	c.init()

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

	response, err := c.Do(request)
	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return io, CodeError{method, u, response.StatusCode, ""}
	}

	return response.Body, err
}
