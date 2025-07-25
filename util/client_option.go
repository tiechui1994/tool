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
	"unsafe"
)

type clientConfig struct {
	proxy func(*http.Request) (*url.URL, error)

	once       sync.Once // set once dns
	dns        []string
	dnsTimeout time.Duration

	dialerTimeout   time.Duration
	dialerKeepAlive time.Duration
	connTimeout     time.Duration
	connLongTimeout time.Duration

	cookieJar http.CookieJar
	cookieFun CustomerCookie
	dir       string        // file jar dir
	sync      chan struct{} // sync file jar
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
			privateJar = (*cookiejar.Jar)(unsafe.Pointer(loadJar))
		} else {
			privateJar, _ = cookiejar.New(nil)
		}
		config.sync = make(chan struct{})
		config.cookieJar = &simpleCookieJar{
			name:       name,
			privateJar: privateJar,
			afterCookieSave: func() {
				config.sync <- struct{}{}
			},
		}

		go func() {
			timer := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-timer.C:
					serialize(privateJar, name)
				case <-config.sync:
					serialize(privateJar, name)
				}
			}
		}()
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
			privateJar = (*cookiejar.Jar)(unsafe.Pointer(loadJar))
		} else {
			privateJar, _ = cookiejar.New(nil)
		}

		config.sync = make(chan struct{})
		config.cookieFun = &simpleCookieFun{
			name:       name,
			privateJar: privateJar,
			afterCookieSave: func() {
				config.sync <- struct{}{}
			},
		}
		go func() {
			timer := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-timer.C:
					serialize(privateJar, name)
				case <-config.sync:
					serialize(privateJar, name)
				}
			}
		}()
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
			return
		}

		jar, _ := cookiejar.New(nil)
		jar.SetCookies(uv, cookies)

		config.sync = make(chan struct{})
		config.cookieFun = &simpleCookieFun{
			name:       name,
			privateJar: jar,
			afterCookieSave: func() {
				config.sync <- struct{}{}
			},
		}

		go func() {
			timer := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-timer.C:
					serialize(jar, name)
				case <-config.sync:
					serialize(jar, name)
				}
			}
		}()
	})
}

type EmbedClient struct {
	*http.Client
	once   sync.Once
	config *clientConfig
}

func NewClient(opts ...ClientOption) *EmbedClient {
	options := &clientConfig{
		dir:   globalClient.config.dir,
		dns:   globalClient.config.dns,
		proxy: globalClient.config.proxy,

		dialerTimeout:   globalClient.config.dialerTimeout,
		dialerKeepAlive: globalClient.config.dialerKeepAlive,
		dnsTimeout:      globalClient.config.dnsTimeout,
		connTimeout:     globalClient.config.connTimeout,
		connLongTimeout: globalClient.config.connLongTimeout,
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
				d := net.Dialer{
					Timeout: c.config.dnsTimeout,
				}
				dns := c.config.dns[int(rand.Int31n(int32(len(c.config.dns))))]
				conn, err := d.DialContext(ctx, network, dns)
				return conn, err
			},
		}

		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					d := net.Dialer{
						Resolver:  resolver,
						Timeout:   c.config.dialerTimeout,
						KeepAlive: c.config.dialerKeepAlive,
					}
				retry:
					conn, err := d.DialContext(ctx, network, addr)
					if err != nil {
						if val, ok := err.(*net.OpError); ok &&
							strings.Contains(val.Err.Error(), "no suitable address found") {
							goto retry
						}

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
					if c.config.proxy != nil {
						return c.config.proxy(req)
					}
					return http.ProxyFromEnvironment(req)
				},
			},
		}

		if c.config.cookieJar != nil {
			client.Jar = c.config.cookieJar
		} else if c.config.cookieFun != nil {
			client.Transport = &customerTransport{
				Transport:      client.Transport,
				CustomerCookie: c.config.cookieFun,
			}
		}

		c.Client = client
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

type customerTransport struct {
	Transport      http.RoundTripper
	CustomerCookie CustomerCookie
}

func (c *customerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if c.CustomerCookie != nil {
		c.CustomerCookie.Cookies(r)
	}

	resp, err := c.Transport.RoundTrip(r)
	if err != nil {
		return nil, err
	}

	if c.CustomerCookie != nil && resp != nil {
		c.CustomerCookie.SetCookies(r.URL, resp)
	}

	return resp, nil
}
