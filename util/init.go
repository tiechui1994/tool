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
	// edge
	"Mozilla/5.0 (X11; Ubuntu 20.04 LTS; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.2903.112",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0 OneOutlook/1.2024.1028.400",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 11_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.6497.170 Safari/537.36 Edg/130.0.2675.82",

	// chrome
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/535.11 (KHTML, like Gecko) Ubuntu/24.04.1 Chrome/131.0.6778.200 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 15_1_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.205 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 12; SM-T867V Build/SP1A.210812.016) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.260 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",

	// firefox
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_2; rv:139.363.934) Gecko/20100101 Firefox/139.363.934",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0 OpenWave/93.4.3744.31",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:139.0) adbeat.com/policy Gecko/20100101 Firefox/139.0",

	// safari
	"Mozilla/5.0 (Macintosh; ARM Mac OS X 15_4_0) AppleWebKit/621.1.15.11.10 (KHTML, like Gecko) Version/18.4 Safari/621.1.15.11",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Safari/605.1.40",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 15_6_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/605.1.15 PrivaBrowser-iOS/3.20/normal",

	// android
	"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36,gzip(gfe)",
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
	config.dnsTimeout = 10 * time.Second
	config.dialerTimeout = 15 * time.Second
	config.dialerKeepAlive = 5 * time.Minute
	config.connTimeout = 15 * time.Second
	config.connLongTimeout = 30 * time.Second

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

func RegisterDNSTimeout(timeout time.Duration) {
	WithDNSTimeout(timeout).apply(globalClient.config)
}

func RegisterDialerTimeout(timeout time.Duration) {
	WithDialerTimeout(timeout).apply(globalClient.config)
}

func RegisterConnTimeout(timeout, longTimeout time.Duration) {
	WithConnTimeout(timeout, longTimeout).apply(globalClient.config)
}

func RegisterCookieJar(name string) {
	WithClientCookieJar(name).apply(globalClient.config)
}

func RegisterCookieFun(name string) {
	WithClientCookieFun(name).apply(globalClient.config)
}

func RegisterInitCookie(name, cookie, endpoint string) {
	WithInitClientCookie(name, cookie, endpoint).apply(globalClient.config)
}

func Dir() string {
	return globalClient.config.dir
}

func GetCookie(url *url.URL, name string) *http.Cookie {
	return globalClient.GetCookie(url, name)
}

func SetCookie(url *url.URL, name, value string) {
	globalClient.SetCookie(url, name, value)
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

func hashUserAgent(u string) string {
	urL, err := url.Parse(u)
	if err == nil {
		u = urL.Hostname()
	}

	rnd := Fnv(u) % uint64(len(agents))
	return agents[int(rnd)]
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
