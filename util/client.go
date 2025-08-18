package util

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
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

type cachedData struct {
	Date   int64
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

func drainBody(b io.Reader) (r1, r2 io.Reader, err error) {
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, http.NoBody, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	return &buf, bytes.NewReader(buf.Bytes()), nil
}

func (c *EmbedClient) dumpRequest(req *http.Request, now time.Time) {
	uv := req.URL
	method := req.Method

	prefix := uv.Path[strings.LastIndex(uv.Path, "/")+1:]
	key := fmt.Sprintf("%v_%v_%v_REQ.txt", prefix, strings.ToUpper(method), now.Format("150405999"))
	raw, _ := httputil.DumpRequestOut(req, true)
	_ = ioutil.WriteFile(filepath.Join(c.config.dir, key), raw, 0644)
}

func (c *EmbedClient) dumpResponse(req *http.Request, resp *http.Response, now time.Time) {
	uv := req.URL
	method := req.Method

	prefix := uv.Path[strings.LastIndex(uv.Path, "/")+1:]
	key := fmt.Sprintf("%v_%v_%v_RESP.txt", prefix, strings.ToUpper(method), now.Format("150405999"))
	raw, _ := httputil.DumpResponse(resp, true)
	_ = ioutil.WriteFile(filepath.Join(c.config.dir, key), raw, 0644)
}

func (c *EmbedClient) cacheRequest(req *http.Request) *cachedData {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	if _, err := os.Stat(testCacheFile); err == nil || !os.IsNotExist(err) {
		var data cachedData
		if err = ReadFile(testCacheFile, &data); err == nil {
			return &data
		}
	}

	return nil
}

func (c *EmbedClient) cacheResponse(req *http.Request, data cachedData) {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	_ = WriteFile(testCacheFile, data)
}

func hasBody(method string) bool {
	return method == http.MethodPut || method == http.MethodPost || method == http.MethodDelete || method == http.MethodPatch
}

func (c *EmbedClient) Request(method, u string, opts ...Option) (json.RawMessage, http.Header, error) {
	c.init()

	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	// dump body reader
	var err error
	var body = options.body
	var dump io.Reader
	if options.retry > 0 && hasBody(method) {
		body, dump, err = drainBody(body)
		if err != nil {
			return nil, nil, err
		}
	}

	try := 0
try:
	if try > 0 {
		if options.randomHost != nil {
			uRL, _ := url.Parse(u)
			uRL.Host = options.randomHost(uRL.Host)
			u = uRL.String()
		}

		// dump dump reader
		if hasBody(method) {
			body, dump, err = drainBody(dump)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	request, err := http.NewRequestWithContext(options.ctx, method, u, body)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range options.header {
		request.Header.Set(k, v)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", hashUserAgent(u))
	}
	if val := request.Header.Get("Content-Length"); val != "" {
		request.ContentLength, _ = strconv.ParseInt(val, 10, 64)
	}

	if options.proxy != nil {
		transport := c.Transport.(*http.Transport)
		proxy := transport.Proxy
		transport.Proxy = options.proxy
		defer func() {
			transport.Proxy = proxy
		}()
	}

	if options.cached {
		if v := c.cacheRequest(request); v != nil {
			if options.cachedPeriod < 0 || time.Now().Unix()-v.Date < options.cachedPeriod*1000 {
				return v.Raw, v.Header, nil
			}
		}
	}

	if options.beforeRequest != nil {
		options.beforeRequest(request)
	}

	var now = time.Now()
	if options.dump {
		c.dumpRequest(request, now)
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

	if options.dump {
		c.dumpResponse(request, response, now)
	}

	if options.afterResponse != nil {
		options.afterResponse(response)
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

	raw, err := io.ReadAll(reader)
	if err != nil && try < options.retry {
		try += 1
		time.Sleep(time.Second * time.Duration(try))
		goto try
	}
	if err != nil {
		return nil, nil, err
	}

	if options.cached {
		c.cacheResponse(request, cachedData{time.Now().Unix(), raw, response.Header})
	}

	if response.StatusCode >= 400 {
		if strings.Contains(response.Header.Get("content-type"), "text/html") {
			return raw, response.Header, CodeError{method, u, response.StatusCode, ""}
		}
		return raw, response.Header, CodeError{method, u, response.StatusCode, string(raw)}
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
	request.Header.Set("User-Agent", hashUserAgent(u))

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
