package util

import (
	"context"
	"crypto/tls"
	"fmt"
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

type cookieFunc interface {
	LoadCookie(req *http.Request)
	SaveCookie(url *url.URL, resp *http.Response)
}

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
	cookieFun cookieFunc
	dir       string        // file jar dir
	sync      chan struct{} // sync file jar
}

type simpleCookieJar struct {
	name            string
	privateJar      *cookiejar.Jar
	afterCookieSave func()
}

func (s *simpleCookieJar) Cookies(u *url.URL) []*http.Cookie {
	fmt.Println("Cookies", u.String())
	return s.privateJar.Cookies(u)
}

func (s *simpleCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.privateJar.SetCookies(u, cookies)
	if s.afterCookieSave != nil {
		s.afterCookieSave()
	}
}

type simpleCookieFun struct {
	name            string
	privateJar      *cookiejar.Jar
	afterCookieSave func()
}

var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

func sanitizeCookieName(n string) string {
	return cookieNameSanitizer.Replace(n)
}

func (s *simpleCookieFun) LoadCookie(req *http.Request) {
	values := make([]string, 0)
	for _, cookie := range s.privateJar.Cookies(req.URL) {
		s := fmt.Sprintf("%s=%s", sanitizeCookieName(cookie.Name), cookie.Value)
		values = append(values, s)
	}
	req.Header.Set("Cookie", strings.Join(values, "; "))
}

func (s *simpleCookieFun) SaveCookie(url *url.URL, resp *http.Response) {
	if rc := resp.Cookies(); len(rc) > 0 {
		s.privateJar.SetCookies(url, rc)
		if s.afterCookieSave != nil {
			s.afterCookieSave()
		}
	}
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
			panic("cookieFun")
		}

		config.sync = make(chan struct{})
		cj := &simpleCookieJar{name: name}
		cj.afterCookieSave = func() {
			config.sync <- struct{}{}
		}
		loadJar := unSerialize(name)
		if loadJar != nil {
			cj.privateJar = (*cookiejar.Jar)(unsafe.Pointer(loadJar))
		} else {
			cj.privateJar, _ = cookiejar.New(nil)
		}
		config.cookieJar = cj

		go func() {
			timer := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-timer.C:
					serialize(cj.privateJar, name)
				case <-config.sync:
					serialize(cj.privateJar, name)
				}
			}
		}()
	})
}

func WithClientCookieFun(name string) ClientOption {
	return newFuncClientOption(func(config *clientConfig) {
		if config.cookieJar != nil {
			panic("cookieJar")
		}

		config.sync = make(chan struct{})
		cf := &simpleCookieFun{name: name}
		loadJar := unSerialize(name)
		if loadJar != nil {
			cf.privateJar = (*cookiejar.Jar)(unsafe.Pointer(loadJar))
		} else {
			cf.privateJar, _ = cookiejar.New(nil)
		}
		cf.afterCookieSave = func() {
			config.sync <- struct{}{}
		}
		config.cookieFun = cf

		go func() {
			timer := time.NewTicker(5 * time.Second)
			for {
				select {
				case <-timer.C:
					serialize(cf.privateJar, name)
				case <-config.sync:
					serialize(cf.privateJar, name)
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
