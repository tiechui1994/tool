package util

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type httpOptions struct {
	test          bool
	debug         bool
	header        map[string]string
	body          io.Reader
	retry         int
	ctx           context.Context
	beforeRequest func(r *http.Request)
	afterResponse func(w *http.Response)
	randomHost    func(string) string
	proxy         func(*http.Request) (*url.URL, error)
}

func (opt *httpOptions) Clone() *httpOptions {
	v := new(httpOptions)
	*v = *opt

	v.ctx = context.Background()
	v.body = nil
	v.header = nil
	return v
}

type Option interface {
	apply(opt *httpOptions)
}

func defaultOptions() *httpOptions {
	return &httpOptions{
		ctx:    context.Background(),
		header: make(map[string]string),
	}
}

// Empty
type emptyHttpOption struct{}

func (emptyHttpOption) apply(*httpOptions) {}

// Func
type funcHttpOption struct {
	call func(*httpOptions)
}

func (fdo *funcHttpOption) apply(do *httpOptions) {
	fdo.call(do)
}

func newFuncDialOption(f func(*httpOptions)) *funcHttpOption {
	return &funcHttpOption{call: f}
}

func WithHeader(header map[string]string) Option {
	return newFuncDialOption(func(o *httpOptions) {
		o.header = header
	})
}

func WithBody(body interface{}) Option {
	return newFuncDialOption(func(o *httpOptions) {
		switch body := body.(type) {
		case io.Reader:
			o.body = body
		case string:
			o.body = strings.NewReader(body)
		case []byte:
			o.body = bytes.NewReader(body)
		default:
			bin, _ := json.Marshal(body)
			o.body = bytes.NewReader(bin)
		}
	})
}

func WithTest() Option {
	return newFuncDialOption(func(options *httpOptions) {
		options.test = true
	})
}

func WithRetry(retry uint) Option {
	return newFuncDialOption(func(o *httpOptions) {
		o.retry = int(retry)
	})
}

func WithContext(ctx context.Context) Option {
	return newFuncDialOption(func(o *httpOptions) {
		o.ctx = ctx
	})
}

func WithDebug() Option {
	return newFuncDialOption(func(o *httpOptions) {
		o.debug = true
	})
}

func WithBeforeRequest(f func(r *http.Request)) Option {
	return newFuncDialOption(func(opt *httpOptions) {
		opt.beforeRequest = f
	})
}

func WithAfterResponse(f func(w *http.Response)) Option {
	return newFuncDialOption(func(opt *httpOptions) {
		opt.afterResponse = f
	})
}

func WithRandomHost(f func(string) string) Option {
	return newFuncDialOption(func(opt *httpOptions) {
		opt.randomHost = f
	})
}

func WithProxy(f func(*http.Request) (*url.URL, error)) Option {
	return newFuncDialOption(func(opt *httpOptions) {
		opt.proxy = f
	})
}
