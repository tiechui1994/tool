package http

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/auth"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/bufio"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/cache"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/inbound"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/socks5"
)

type Listener struct {
	listener net.Listener
	addr     string
	closed   bool
}

func (l *Listener) RawAddress() string {
	return l.addr
}

func (l *Listener) Address() string {
	return l.listener.Addr().String()
}

func (l *Listener) Close() error {
	l.closed = true
	return l.listener.Close()
}

func New(addr string, in chan<- ctx.ConnContext) (ctx.Listener, error) {
	return NewWithAuthenticate(addr, in, true)
}

func NewWithAuthenticate(addr string, in chan<- ctx.ConnContext, authenticate bool) (ctx.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	var c *cache.LruCache
	if authenticate {
		c = cache.New(cache.WithAge(30))
	}

	hl := &Listener{
		listener: l,
		addr:     addr,
	}
	go func() {
		for {
			conn, err := hl.listener.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}
			go HandleConn(conn, in, c)
		}
	}()

	return hl, nil
}

func HandleConn(c net.Conn, in chan<- ctx.ConnContext, cache *cache.LruCache) {
	client := newClient(c.RemoteAddr(), c.LocalAddr(), in)
	defer client.CloseIdleConnections()
	conn := bufio.NewBufferedConn(c)

	keepAlive := true
	trusted := cache == nil // disable authenticate if cache is nil

	for keepAlive {
		request, err := http.ReadRequest(conn.Reader())
		if err != nil {
			break
		}

		request.RemoteAddr = conn.RemoteAddr().String()
		keepAlive = strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive"

		var resp *http.Response
		if !trusted {
			resp = authenticate(request, cache)
			trusted = resp == nil
		}

		if trusted {
			if request.Method == http.MethodConnect {
				// Manual writing to support CONNECT for http 1.0 (workaround for uplay client)
				_, err = fmt.Fprintf(conn, "HTTP/%d.%d %03d %s\r\n\r\n",
					request.ProtoMajor, request.ProtoMinor, http.StatusOK, "Connection established")
				if err != nil {
					break
				}

				in <- inbound.NewHTTPS(request, conn)

				return // hijack connection
			}

			host := request.Header.Get("Host")
			if host != "" {
				request.Host = host
			}

			request.RequestURI = ""

			if isUpgradeRequest(request) {
				handleUpgrade(conn, request)
				return // hijack connection
			}

			removeHopByHopHeaders(request.Header)
			removeExtraHTTPHostPort(request)

			if request.URL.Scheme == "" || request.URL.Host == "" {
				resp = responseWith(request, http.StatusBadRequest)
			} else {
				resp, err = client.Do(request)
				if err != nil {
					resp = responseWith(request, http.StatusBadGateway)
				}
			}

			removeHopByHopHeaders(resp.Header)
		}

		if keepAlive {
			resp.Header.Set("Proxy-Connection", "keep-alive")
			resp.Header.Set("Connection", "keep-alive")
			resp.Header.Set("Keep-Alive", "timeout=4")
		}

		resp.Close = !keepAlive
		err = resp.Write(conn)
		if err != nil {
			break // close connection
		}
	}

	conn.Close()
}

func authenticate(request *http.Request, cache *cache.LruCache) *http.Response {
	authenticator := auth.NewAuthenticator(nil)
	if authenticator != nil {
		credential := parseBasicProxyAuthorization(request)
		if credential == "" {
			resp := responseWith(request, http.StatusProxyAuthRequired)
			resp.Header.Set("Proxy-Authenticate", "Basic")
			return resp
		}

		authed, exist := cache.Get(credential)
		if !exist {
			user, pass, err := decodeBasicProxyAuthorization(credential)
			authed = err == nil && authenticator.Verify(user, pass)
			cache.Set(credential, authed)
		}
		if !authed.(bool) {
			return responseWith(request, http.StatusForbidden)
		}
	}

	return nil
}

func responseWith(request *http.Request, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Header:     http.Header{},
	}
}

func newClient(source net.Addr, originTarget net.Addr, in chan<- ctx.ConnContext) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			Dial: func(network, address string) (net.Conn, error) {
				if network != "tcp" && network != "tcp4" && network != "tcp6" {
					return nil, errors.New("unsupported network " + network)
				}

				dstAddr := socks5.ParseAddr(address)
				if dstAddr == nil {
					return nil, socks5.ErrAddressNotSupported
				}

				left, right := net.Pipe()
				in <- inbound.NewHTTP(dstAddr, source, originTarget, right)
				return left, nil
			},
		},

		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
