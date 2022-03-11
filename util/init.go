package util

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

var agents = []string{
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36",
	"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/534.57.2 (KHTML, like Gecko) Version/5.1.7 Safari/534.57.2",

	"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:34.0) Gecko/20100101 Firefox/34.0",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0",
}

type entry struct {
	Name       string    `json:"name"`
	Value      string    `json:"value"`
	Domain     string    `json:"domain"`
	Path       string    `json:"path"`
	SameSite   string    `json:"samesite"`
	Secure     bool      `json:"secure"`
	HttpOnly   bool      `json:"httponly"`
	Persistent bool      `json:"persistent"`
	HostOnly   bool      `json:"host_only"`
	Expires    time.Time `json:"expires"`
	Creation   time.Time `json:"creation"`
	LastAccess time.Time `json:"lastaccess"`
	SeqNum     uint64    `json:"seqnum"`
}

type Jar struct {
	PsList cookiejar.PublicSuffixList `json:"pslist"`

	// mu locks the remaining fields.
	Mu sync.Mutex `json:"mu"`

	// entries is a set of entries, keyed by their eTLD+1 and subkeyed by
	// their name/domain/path.
	Entries map[string]map[string]entry `json:"entries"`

	// nextSeqNum is the next sequence number assigned to a new cookie
	// created SetCookies.
	NextSeqNum uint64 `json:"nextseqnum"`
}

var (
	requestInterceptor  []func(*http.Request)
	responseInterceptor []func(*http.Response)

	jar     http.CookieJar
	jarsync chan struct{}
	agent   string
	confdir string
	cookie  = "cookie"

	dnss = []string{
		"8.8.8.8:53", "8.8.4.4:53",
		"114.114.114.114:53",
		"223.5.5.5:53", "223.6.6.6:53",
		"112.124.47.27:53", "114.215.126.16:53",
		"208.67.222.222:53", "208.67.220.220:53",
	}
)

func init() {
	rand.Seed(time.Now().UnixNano())
	jar, _ = cookiejar.New(nil)

	resolver := net.Resolver{
		PreferGo: false,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}

			conn, err := d.DialContext(ctx, network, dnss[int(rand.Int31n(int32(len(dnss))))])
			return conn, err
		},
	}
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{
					Resolver:  &resolver,
					Timeout:   30 * time.Second,
					KeepAlive: 5 * time.Minute,
				}
				conn, err := d.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				return newTimeoutConn(conn, 60*time.Second, 300*time.Second), nil
			},
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       50 * time.Second,
			ResponseHeaderTimeout: time.Second * 60,
		},
		Jar: jar,
	}
}

func RegisterLocalJar() {
	jarsync = make(chan struct{})
	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}

	confdir = filepath.Join(home, ".config/tool")
	os.MkdirAll(confdir, 0775)

	localjar := unserialize()
	if localjar != nil {
		jar = (*cookiejar.Jar)(unsafe.Pointer(localjar))
		http.DefaultClient.Jar = jar
	}

	go func() {
		timer := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-timer.C:
				cookjar := jar.(*cookiejar.Jar)
				serialize(cookjar)
			case <-jarsync:
				cookjar := jar.(*cookiejar.Jar)
				serialize(cookjar)
			}
		}
	}()
}

func GetCookie(url *url.URL, name string) *http.Cookie {
	for _, c := range jar.Cookies(url) {
		if c != nil && c.Name == name {
			return c
		}
	}

	return nil
}

func UserAgent(args ...int) string {
	if agent != "" {
		return agent
	}

	rnd := int(time.Now().Unix()) % len(agents)
	args = append(args, rnd)
	if args[0] < len(agent) && args[0] >= 0 {
		agent = agents[args[0]]
	} else {
		agent = agents[rnd]
	}

	return agent
}

func SyncCookieJar() {
	if jarsync != nil {
		jarsync <- struct{}{}
	}
}

func ConfDir() string {
	return confdir
}

func LogRequest(f func(*http.Request)) {
	requestInterceptor = append(requestInterceptor, f)
}

func LogResponse(f func(*http.Response)) {
	responseInterceptor = append(responseInterceptor, f)
}

func WriteFile(filepath string, data interface{}) error {
	fd, err := os.Create(filepath)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(fd)
	return encoder.Encode(data)
}

func ReadFile(filepath string, data interface{}) error {
	fd, err := os.Open(filepath)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(fd)
	decoder.UseNumber()
	return decoder.Decode(data)
}

func serialize(jar *cookiejar.Jar) {
	oldpath := filepath.Join(confdir, "."+cookie+".json")
	localjar := (*Jar)(unsafe.Pointer(jar))
	fd, _ := os.Create(oldpath)
	json.NewEncoder(fd).Encode(localjar)
	fd.Sync()

	os.Rename(oldpath, filepath.Join(confdir, cookie+".json"))
}

func unserialize() *Jar {
	var localjar Jar
	fd, _ := os.Open(filepath.Join(confdir, cookie+".json"))
	err := json.NewDecoder(fd).Decode(&localjar)
	if err != nil {
		return nil
	}

	return &localjar
}
