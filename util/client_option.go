package util

import (
	"context"
	"crypto/tls"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

type clientConfig struct {
	lock sync.Mutex

	proxy          func(*http.Request) (*url.URL, error)
	additionHeader map[string]string

	once       sync.Once // set once dns
	dns        []string
	dnsTimeout time.Duration

	dialerTimeout   time.Duration
	dialerKeepAlive time.Duration
	connTimeout     time.Duration
	connLongTimeout time.Duration

	cookieJar CustomerCookie
	cookieFun CustomerCookie
	dir       string // file jar dir

	beforeRequest func(r *http.Request)
	afterResponse func(w *http.Response)
}

type ClientOption interface {
	apply(opt *clientConfig)
}

// Func
type funcClientOption struct {
	call func(*clientConfig)
}

func (fun *funcClientOption) apply(config *clientConfig) {
	fun.call(config)
}

func newFuncClientOption(f func(*clientConfig)) *funcClientOption {
	return &funcClientOption{call: f}
}

func WithClientProxy(proxy func(*http.Request) (*url.URL, error)) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		config.proxy = proxy
	})
}

func WithClientDNS(dns []string) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		config.once.Do(func() {
			var value []string
			for _, v := range dns {
				if strings.HasSuffix(v, ":53") {
					value = append(value, v)
				}
			}
			if len(value) > 0 {
				config.dns = value
			}
		})
	})
}

func WithDNSTimeout(timeout time.Duration) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if timeout < time.Second {
			timeout = time.Second
		}
		config.dnsTimeout = timeout
	})
}

func WithDialerTimeout(timeout time.Duration) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if timeout < time.Second {
			timeout = time.Second
		}
		config.dialerTimeout = timeout
	})
}

func WithConnTimeout(timeout, longTimeout time.Duration) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if timeout < time.Second {
			timeout = time.Second
		}
		if timeout > longTimeout {
			longTimeout = timeout
		}

		config.connTimeout = timeout
		config.connLongTimeout = longTimeout
	})
}

func WithClientCookieJar(name string) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if config.cookieFun != nil {
			panic("cookieFun exist, not support cookie jar")
		}

		if config.cookieJar != nil {
			return
		}

		var privateJar *cookiejar.Jar
		if loadJar := unSerialize(name); loadJar != nil {
			privateJar = loadJar
		} else {
			privateJar, _ = cookiejar.New(nil)
		}
		config.cookieJar = &simpleCookieJar{
			name:       name,
			privateJar: privateJar,
			afterCookieSave: func() {
				serialize(privateJar, name)
			},
		}
	})
}

func WithClientCookieFun(name string) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if config.cookieJar != nil {
			panic("cookieJar exist, not support cookie fun")
		}

		if config.cookieFun != nil {
			return
		}

		var privateJar *cookiejar.Jar
		if loadJar := unSerialize(name); loadJar != nil {
			privateJar = loadJar
		} else {
			privateJar, _ = cookiejar.New(nil)
		}
		config.cookieFun = &simpleCookieFun{
			name:       name,
			privateJar: privateJar,
			afterCookieSave: func() {
				serialize(privateJar, name)
			},
		}
	})
}

func WithInitClientCookie(name, cookie, endpoint string) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if config.cookieJar != nil {
			panic("cookieJar exist, not support cookie fun")
		}

		uv, err := url.Parse(endpoint)
		if err != nil {
			panic("invalid endpoint: " + endpoint + " " + err.Error())
		}

		var cookies []*http.Cookie
		tokens := strings.Split(cookie, ";")
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			kv := strings.SplitN(token, "=", 2)
			if len(kv) == 2 && len(strings.TrimSpace(kv[0])) > 0 && len(strings.TrimSpace(kv[1])) > 0 {
				cookies = append(cookies, &http.Cookie{
					Name:     strings.TrimSpace(kv[0]),
					Value:    strings.TrimSpace(kv[1]),
					Path:     "/",
					HttpOnly: true,
					Secure:   true,
				})
			}
		}

		if config.cookieFun != nil {
			cf := config.cookieFun.(*simpleCookieFun)
			cf.privateJar.SetCookies(uv, cookies)
			if cf.afterCookieSave != nil {
				cf.afterCookieSave()
			}
			return
		}

		jar, _ := cookiejar.New(nil)
		jar.SetCookies(uv, cookies)
		cookieFun := &simpleCookieFun{
			name:       name,
			privateJar: jar,
			afterCookieSave: func() {
				serialize(jar, name)
			},
		}
		config.cookieFun = cookieFun
		cookieFun.afterCookieSave()
	})
}

func WithAddiotionHeader(header map[string]string) ClientOption {
	if header == nil {
		header = make(map[string]string)
	}
	return newFuncClientOption(func(config *clientConfig) {
		config.additionHeader = header
	})
}

func WithClientBeforeRequest(f func(*http.Request)) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		config.beforeRequest = f
	})
}

func WithClientAfterResponse(f func(*http.Response)) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		config.afterResponse = f
	})
}

type EmbedClient struct {
	transport *customerTransport
	once      sync.Once
	config    *clientConfig
}

func NewClient(opts ...ClientOption) *EmbedClient {
	options := &clientConfig{
		dir:   globalConfig.dir,
		dns:   globalConfig.dns,
		proxy: globalConfig.proxy,

		dialerTimeout:   globalConfig.dialerTimeout,
		dialerKeepAlive: globalConfig.dialerKeepAlive,
		dnsTimeout:      globalConfig.dnsTimeout,
		connTimeout:     globalConfig.connTimeout,
		connLongTimeout: globalConfig.connLongTimeout,
	}

	for _, opt := range opts {
		opt.apply(options)
	}
	return &EmbedClient{config: options}
}

func (c *EmbedClient) init() {
	c.once.Do(func() {
		resolver := &net.Resolver{
			PreferGo: true, // 表示使用 Go 的 DNS
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: c.config.dnsTimeout}
				dns := c.config.dns[int(rand.Int31n(int32(len(c.config.dns))))]
				conn, err := d.DialContext(ctx, network, dns)
				return conn, err
			},
		}

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if val, ok := ctx.Value(proxyKey).(*proxy); ok && val.proxyDail != nil {
					if val.proxyDail != nil {
						return val.proxyDail(ctx, network, addr)
					}
				}

				var resolve *net.Resolver
				if len(c.config.dns) > 0 {
					resolve = resolver
				}
				d := net.Dialer{
					Resolver:  resolve,
					Timeout:   c.config.dialerTimeout,
					KeepAlive: c.config.dialerKeepAlive,
				}
				conn, err := d.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				return newTimeoutConn(conn, c.config.connTimeout, c.config.connLongTimeout), nil
			},
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			Proxy: func(req *http.Request) (*url.URL, error) {
				if val, ok := req.Context().Value(proxyKey).(*proxy); ok && val.proxy != nil {
					if val.proxy != nil {
						return val.proxy(req)
					}
				}

				if c.config.proxy != nil {
					return c.config.proxy(req)
				}
				return http.ProxyFromEnvironment(req)
			},
		}

		_ = http2.ConfigureTransport(transport)

		c.transport = &customerTransport{
			Transport: transport,
			config:    c.config,
		}
	})
}

func (c *EmbedClient) GetCookie(url *url.URL, name string) *http.Cookie {
	if c.config.cookieFun == nil && c.config.cookieJar == nil {
		return nil
	}

	var jar *cookiejar.Jar
	if c.config.cookieFun != nil {
		jar = c.config.cookieFun.(*simpleCookieFun).privateJar
	} else if c.config.cookieJar != nil {
		jar = c.config.cookieJar.(*simpleCookieJar).privateJar
	} else {
		panic("no cookieJar")
	}

	cookies := jar.Cookies(url)
	for _, c := range cookies {
		if c != nil && c.Name == name {
			return c
		}
	}

	return nil
}

func (c *EmbedClient) GetCookies(url *url.URL) []*http.Cookie {
	if c.config.cookieFun == nil && c.config.cookieJar == nil {
		return nil
	}

	var jar *cookiejar.Jar
	if c.config.cookieFun != nil {
		jar = c.config.cookieFun.(*simpleCookieFun).privateJar
	} else if c.config.cookieJar != nil {
		jar = c.config.cookieJar.(*simpleCookieJar).privateJar
	} else {
		panic("no cookieJar")
	}

	return jar.Cookies(url)
}

func (c *EmbedClient) SetCookie(u *url.URL, name, value string) {
	if c.config.cookieFun == nil && c.config.cookieJar == nil {
		return
	}

	var jar *cookiejar.Jar
	if c.config.cookieFun != nil {
		jar = c.config.cookieFun.(*simpleCookieFun).privateJar
	} else if c.config.cookieJar != nil {
		jar = c.config.cookieJar.(*simpleCookieJar).privateJar
	} else {
		panic("no cookieJar")
	}

	jar.SetCookies(u, []*http.Cookie{
		{
			Name:     sanitizeCookieName(name),
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
		},
	})
}

func (c *EmbedClient) CleanCookie(u *url.URL) error {
	if c.config.cookieFun == nil && c.config.cookieJar == nil {
		return nil
	}

	if c.config.cookieFun != nil {
		return clean(u, c.config.cookieFun)
	} else if c.config.cookieJar != nil {
		return clean(u, c.config.cookieJar)
	} else {
		panic("no cookieJar")
	}
}

type contextKey string

const (
	// 是否使用代理的控制 Key
	proxyKey contextKey = "proxy"
	// 请求级 Callback 钩子的 Key
	callbackKey contextKey = "callback"
)

type customerTransport struct {
	Transport http.RoundTripper
	config    *clientConfig
}

type callback struct {
	beforeTransport func(req *http.Request)
	afterTransport  func(req *http.Request, resp *http.Response)
}

type proxy struct {
	proxy     func(*http.Request) (*url.URL, error)
	proxyDail func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (c *customerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// cookie, 全局范围
	var cookie CustomerCookie
	if c.config.cookieFun != nil {
		cookie = c.config.cookieFun
	} else if c.config.cookieJar != nil {
		cookie = c.config.cookieJar
	}
	if cookie != nil {
		cookie.Cookies(r)
	}

	// beforeRequest 全局的
	if c.config.beforeRequest != nil {
		c.config.beforeRequest(r)
	}

	// headers, 全局范围
	if c.config.additionHeader != nil {
		for k, v := range c.config.additionHeader {
			r.Header.Set(k, v)
		}
	}

	// 请求级别
	var call *callback
	if cb, ok := r.Context().Value(callbackKey).(*callback); ok {
		call = cb
	}

	if call != nil && call.beforeTransport != nil {
		call.beforeTransport(r)
	}
	resp, err := c.Transport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if cookie != nil && resp != nil {
		cookie.SetCookies(r.URL, resp)
	}
	if call != nil && call.afterTransport != nil {
		call.afterTransport(r, resp)
	}

	// afterResponse 全局的
	if c.config.afterResponse != nil {
		c.config.afterResponse(resp)
	}

	return resp, nil
}
