package util

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	}
	return fmt.Sprintf("%v %q : (status:%v, message:%v)", err.Method, err.URL,
		http.StatusText(err.Code), err.Message)
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

func PATCH(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.PATCH(u, opts...)
}

func HEAD(u string, opts ...Option) (raw json.RawMessage, err error) {
	return globalClient.HEAD(u, opts...)
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
	_ = os.WriteFile(filepath.Join(c.config.dir, key), raw, 0644)
}

func (c *EmbedClient) dumpResponse(req *http.Request, resp *http.Response, now time.Time) {
	uv := req.URL
	method := req.Method

	prefix := uv.Path[strings.LastIndex(uv.Path, "/")+1:]
	key := fmt.Sprintf("%v_%v_%v_RESP.txt", prefix, strings.ToUpper(method), now.Format("150405999"))
	raw, _ := httputil.DumpResponse(resp, true)
	_ = os.WriteFile(filepath.Join(c.config.dir, key), raw, 0644)
}

func (c *EmbedClient) cacheRequest(req *http.Request, opts *httpOptions) (json.RawMessage, http.Header, error) {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	var cache cachedData
	if err := readFile(testCacheFile, &cache); err != nil {
		return nil, nil, err
	}
	if opts.cachedPeriod < 0 || time.Now().Unix()-cache.Date < opts.cachedPeriod*1000 {
		return cache.Raw, cache.Header, nil
	}

	return nil, nil, fmt.Errorf("cache expired")
}

func (c *EmbedClient) cacheResponse(req *http.Request, data cachedData) {
	key := hex.EncodeToString(MD5(req.Method + "_" + req.URL.String()))
	testCacheFile := filepath.Join(c.config.dir, key)
	_ = writeFile(testCacheFile, data)
}

func (c *EmbedClient) buildRequest(u, method string, body io.Reader, opts *httpOptions) (*http.Request, error) {
	ctx := opts.ctx
	if opts.timeout > 0 {
		ctx, _ = context.WithTimeout(ctx, opts.timeout)
	}
	if opts.dump {
		now := time.Now()
		ctx = context.WithValue(ctx, callbackKey, &callback{
			beforeTransport: func(req *http.Request) {
				if opts.dump {
					c.dumpRequest(req, now)
				}
			},
			afterTransport: func(req *http.Request, resp *http.Response) {
				if opts.dump {
					c.dumpResponse(req, resp, now)
				}
			},
		})
	}
	if opts.proxy != nil || opts.proxyDail != nil {
		ctx = context.WithValue(ctx, proxyKey, &proxy{
			proxy:     opts.proxy,
			proxyDail: opts.proxyDail,
		})
	}

	request, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}

	for k, v := range opts.header {
		request.Header.Set(k, v)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", hashUserAgent(u))
	}
	if val := request.Header.Get("Content-Length"); val != "" {
		request.ContentLength, _ = strconv.ParseInt(val, 10, 64)
	}

	return request, nil
}

func hasBody(method string) bool {
	return method == http.MethodPut || method == http.MethodPost || method == http.MethodDelete || method == http.MethodPatch
}

func (c *EmbedClient) randomHost(u string, opts *httpOptions) (string, error) {
	uL, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	if opts.randomHost != nil {
		uL.Host = opts.randomHost(uL.Host)
	}
	return uL.String(), nil
}

func (c *EmbedClient) httpClient(opts *httpOptions) *http.Client {
	return &http.Client{Transport: c.transport}
}

func (c *EmbedClient) readBody(response *http.Response) (json.RawMessage, http.Header, error) {
	defer response.Body.Close()

	var reader io.Reader
	var gzipReader *gzip.Reader
	var flatReader io.ReadCloser
	encoding := response.Header.Get("Content-Encoding")
	switch strings.ToLower(encoding) {
	case "gzip":
		var err error
		gzipReader, err = gzip.NewReader(response.Body)
		if err != nil {
			return nil, nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	case "deflate":
		flatReader = flate.NewReader(response.Body)
		defer flatReader.Close()
		reader = flatReader
	default:
		reader = response.Body
	}

	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	return raw, response.Header, nil
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
	for try <= options.retry {
		u, err = c.randomHost(u, options)
		if err != nil {
			return nil, nil, err
		}

		// dump dump reader
		if try > 0 && hasBody(method) {
			body, dump, err = drainBody(dump)
			if err != nil {
				return nil, nil, err
			}
		}

		request, err := c.buildRequest(u, method, body, options)
		if err != nil {
			return nil, nil, err
		}

		if options.cached {
			raw, headers, err := c.cacheRequest(request, options)
			if err == nil {
				return raw, headers, nil
			}
		}

		if options.beforeRequest != nil {
			options.beforeRequest(request)
		}

		response, err := c.httpClient(options).Do(request)
		if err != nil {
			if try < options.retry && IsRetryable(err) {
				try++
				time.Sleep(time.Second * time.Duration(try))
				continue
			}
			return nil, nil, err
		}

		if options.afterResponse != nil {
			options.afterResponse(response)
		}

		raw, headers, err := c.readBody(response)
		if err != nil {
			if try < options.retry && IsRetryable(err) {
				try++
				time.Sleep(time.Second * time.Duration(try))
				continue
			}
			return nil, nil, err
		}

		if options.cached {
			c.cacheResponse(request, cachedData{time.Now().Unix(), raw, headers})
		}

		if response.StatusCode >= http.StatusBadRequest {
			if try < options.retry && IsRetryable(CodeError{method, u, response.StatusCode, ""}) {
				try++
				time.Sleep(time.Second * time.Duration(try))
				continue
			}
			if strings.HasPrefix(headers.Get("content-type"), "text/html") {
				return raw, headers, CodeError{method, u, response.StatusCode, ""}
			}
			return raw, headers, CodeError{method, u, response.StatusCode, string(raw)}
		}

		return raw, headers, nil
	}

	return nil, nil, fmt.Errorf("max retries exceeded")
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

func (c *EmbedClient) PATCH(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodPatch, u, opts...)
	return raw, err
}

func (c *EmbedClient) HEAD(u string, opts ...Option) (raw json.RawMessage, err error) {
	raw, _, err = c.Request(http.MethodHead, u, opts...)
	return raw, err
}

func (c *EmbedClient) File(u, method string, opts ...Option) (io io.Reader, err error) {
	c.init()

	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	try := 0
	for try <= options.retry {
		request, err := c.buildRequest(u, method, options.body, options)
		if err != nil {
			return nil, err
		}

		response, err := c.httpClient(options).Do(request)
		if err != nil {
			if try < options.retry && IsRetryable(err) {
				try++
				time.Sleep(time.Second * time.Duration(try))
				continue
			}
			return nil, err
		}

		if response.StatusCode >= http.StatusBadRequest {
			err = CodeError{method, u, response.StatusCode, ""}
			response.Body.Close()
			if try < options.retry && IsRetryable(err) {
				try++
				time.Sleep(time.Second * time.Duration(try))
				continue
			}
			return nil, err
		}

		return response.Body, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if val, ok := err.(CodeError); ok {
		return val.Code == http.StatusBadGateway || val.Code == http.StatusServiceUnavailable ||
			val.Code == http.StatusGatewayTimeout
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	var x509Err x509.UnknownAuthorityError
	if errors.As(err, &x509Err) {
		return false
	}

	var netError net.Error
	if errors.As(err, &netError) {
		if netError.Timeout() {
			return true
		}
	}

	// 系统调用错误
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EHOSTDOWN) || errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETDOWN) || errors.Is(err, syscall.ENETRESET) || errors.Is(err, syscall.ENETUNREACH) {
		return true
	}

	// 连接发送前关闭
	if errors.Is(err, net.ErrClosed) {
		return true
	}

	return false
}
