package util

import (
	"encoding/gob"
	"encoding/json"
	"math/rand"
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
	agent string
)

var globalClient *EmbedClient

func init() {
	rand.Seed(time.Now().UnixNano())
	config := new(clientConfig)
	config.dns = []string{
		"8.8.8.8:53", "8.8.4.4:53",
		"114.114.114.114:53",
		"223.5.5.5:53", "223.6.6.6:53",
		"112.124.47.27:53", "114.215.126.16:53",
		"208.67.222.222:53", "208.67.220.220:53",
	}
	config.dnsTimeout = 10* time.Second
	config.dialerTimeout = 15*time.Second
	config.dialerKeepAlive = 5 * time.Minute
	config.connTimeout = 15*time.Second
	config.connLongTimeout = 30*time.Second

	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	config.dir = filepath.Join(home, ".config/tool")
	_ = os.MkdirAll(config.dir, 0775)

	globalClient = &EmbedClient{config: config}
}

func RegisterDNS(dns []string) {
	WithClientDNS(dns).apply(globalClient.config)
}

func RegisterProxy(proxy func(*http.Request) (*url.URL, error)) {
	WithClientProxy(proxy).apply(globalClient.config)
}

func RegisterDNSTimeout(timeout time.Duration)  {
	WithDNSTimeout(timeout).apply(globalClient.config)
}

func RegisterDialerTimeout(timeout time.Duration)  {
	WithDialerTimeout(timeout).apply(globalClient.config)
}

func RegisterConnTimeout(timeout, longTimeout time.Duration)  {
	WithConnTimeout(timeout, longTimeout).apply(globalClient.config)
}

func RegisterCookieJar(name string) {
	WithClientCookieJar(name).apply(globalClient.config)
}

func RegisterCookieFun(name string) {
	WithClientCookieFun(name).apply(globalClient.config)
}

func Dir() string {
	return globalClient.config.dir
}

func GetCookie(url *url.URL, name string) *http.Cookie {
	return globalClient.GetCookie(url, name)
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

func WriteFile(filepath string, data interface{}) error {
	fd, err := os.Create(filepath)
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(fd)
	return encoder.Encode(data)
}

func ReadFile(filepath string, data interface{}) error {
	fd, err := os.Open(filepath)
	if err != nil {
		return err
	}
	decoder := gob.NewDecoder(fd)
	return decoder.Decode(data)
}

func serialize(jar *cookiejar.Jar, name string) {
	oldPath := filepath.Join(Dir(), name+".backup")
	fd, _ := os.Create(oldPath)
	_ = json.NewEncoder(fd).Encode((*Jar)(unsafe.Pointer(jar)))
	_ = fd.Sync()
	_ = os.Rename(oldPath, filepath.Join(Dir(), name))
}

func unSerialize(name string) *Jar {
	var jar Jar
	fd, _ := os.Open(filepath.Join(Dir(), name))
	err := json.NewDecoder(fd).Decode(&jar)
	if err != nil {
		return nil
	}
	return &jar
}
