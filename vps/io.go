package vps

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	mrand "math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/tiechui1994/tool/util"
)

func tencode() string {
	var (
		s, u = 64, 0
	)

	a := map[string]int{}
	i := strings.Split("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_", "")
	for ; u < s; u++ {
		a[i[u]] = u
	}

	n := func(t int) string {
		var e string
		e = i[t%s] + e
		t = int(t / s)
		for t > 0 {
			e = i[t%s] + e
			t = int(t / s)
		}
		return e
	}

	return n(int(time.Now().UnixNano() / 1e6))
}

func tdecode(t string) int {
	var (
		s, u = 64, 0
	)

	a := map[string]int{}
	i := strings.Split("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_", "")
	for ; u < s; u++ {
		a[i[u]] = u
	}

	var e = 0
	for u = 0; u < len(t); u++ {
		e = e*s + a[string(t[u])]
	}

	return e
}

func random(length int) string {
	bs := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	result := make([]byte, 0, length)
	r := mrand.New(mrand.NewSource(time.Now().UnixNano())) // 产生随机数实例
	for i := 0; i < length; i++ {
		result = append(result, bs[r.Intn(len(bs))]) // 获取随机
	}
	return string(result)
}

func login(email, pw string) {
	body := map[string]interface{}{
		"email":          email,
		"keepLogin":      false,
		"pw":             pw,
		"return_url":     "https://ide.goorm.io/my/dashboard",
		"signupLanguage": "zh",
	}
	header := map[string]string{
		"accept":       "application/json",
		"content-type": "application/json;charset=UTF-8",
	}

	u := "https://accounts.goorm.io/api/login"
	raw, err := util.POST(u, body, header)
	log.Println(string(raw), err)
	util.SyncCookieJar()
}

type Container struct {
	Uid   string `json:"uid"`
	Stack struct {
		ID string `json:"id"`
		OS string `json:"osId"`
	} `json:"stack"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	ProjectPath    string    `json:"projectPath"`
	LastAccessDate time.Time `json:"lastAccessDate"`
}

func containers() (list []Container, err error) {
	u := "https://ide.goorm.io/api/users/my/containers/all"
	header := map[string]string{
		"accept": "application/json",
	}

	raw, err := util.GET(u, header)
	if err != nil {
		return list, err
	}

	err = json.Unmarshal(raw, &list)

	return list, err
}

type AddrInfo struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

func addr(c Container) (addr AddrInfo, err error) {
	values := []string{
		"docker_id=" + c.Uid,
		"project_path=" + c.ProjectPath,
	}
	u := "https://ide-run.goorm.io/container/ls_addr?" + strings.Join(values, "&")
	header := map[string]string{
		"accept": "application/json",
	}

	raw, err := util.GET(u, header)
	if err != nil {
		return addr, err
	}

	err = json.Unmarshal(raw, &addr)

	return addr, err
}

func run(c Container) {
	socket := Socket{
		Endpoint: "https://ide-run.goorm.io",
		Attr: []string{
			"docker_id=" + c.Uid,
			"project_path=" + c.ProjectPath,
			"useAgent=false",
			"agentToken=undefined",
		},
	}
	socket.polling()
}

type Socket struct {
	Endpoint string
	Attr     []string
	sid      string
	timeout  int
	interval int
}

// /socket.io/?
// EIO=3&
// transport=polling&
// t=Nlbxcde
func (s *Socket) polling() error {
	values := []string{
		"EIO=3",
		"transport=polling",
		"t=", tencode(),
	}
	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")
	raw, err := util.GET(u, nil)
	fmt.Println("hex", hex.EncodeToString(raw))
	fmt.Println("str", string(raw))
	raw = regexp.MustCompile(`{.*}`).Find(raw)

	var result struct {
		Sid          string   `json:"sid"`
		Upgrades     []string `json:"upgrades"`
		PingInterval int      `json:"pingInterval"`
		PingTimeout  int      `json:"pingTimeout"`
	}

	err = json.Unmarshal(raw, &result)
	if err != nil {
		return err
	}

	s.sid = result.Sid
	s.interval = result.PingInterval
	s.timeout = result.PingTimeout

	log.Printf("polling: %v", string(raw))
	return nil
}

func (s *Socket) polling1() (result string, err error) {
	values := []string{
		"EIO=3",
		"transport=polling",
		"t=", tencode(),
		"sid=" + s.sid,
	}
	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")
	data, err := util.GET(u, nil)

	return string(data), nil
}

func (s *Socket) polling2(body string) error {
	values := []string{
		"EIO=3",
		"transport=polling",
		"t=", tencode(),
		"sid=" + s.sid,
	}
	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")
	data, err := util.POST(u, body, nil)
	if err != nil {
		log.Println("polling2 Read:", err)
		return err
	}

	log.Println(string(data))
	return nil
}
