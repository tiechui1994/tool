package vps

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	mrand "math/rand"
	"regexp"
	"strings"
	"sync"
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

// https://ide-run.goorm.io/api/plan/init
//

type Domain struct {
	Domain       string `json:"container_name"`
	DockerID     string `json:"docker_id"`
	FsPort       int    `json:"fs_port"`
	GoormptyPort int    `json:"goormpty_port"`
}

func run(c Container) (domain string, err error) {
	socket := Socket{
		Endpoint: "https://ide-run.goorm.io",
		Attr: []string{
			"docker_id=" + c.Uid,
			"project_path=" + c.ProjectPath,
			"useAgent=false",
			"agentToken=undefined",
		},
	}
	err = socket.polling()
	if err != nil {
		return domain, err
	}

	conn, _, err := socket.polling1()
	if err != nil {
		return domain, err
	}

	if conn.WriteMessage(socket_2probe) {
		err = errors.New("closed")
		return domain, err
	}

	cmd := func(cmd string, data interface{}) string {
		var ans interface{}
		switch cmd {
		case "message", "access", "leave", "join":
			bin, _ := json.Marshal(data)
			ans = []string{
				cmd, string(bin),
			}
		default:
			if data != nil {
				ans = []interface{}{
					cmd, data,
				}
			} else {
				ans = []string{cmd}
			}
		}

		bin, _ := json.Marshal(ans)
		return string(bin)
	}

	go func() {
		first := true
		cmds := []string{
			cmd("access", map[string]string{
				"channel":      "join",
				"uid":          c.Uid,
				"project_path": c.ProjectPath,
			}),
			cmd("leave", map[string]string{
				"channel": "workspace",
				"action":  "leave_workspace",
				"message": "goodbye",
			}),
			cmd("join", map[string]string{
				"channel": "workspace",
				"action":  "join_workspace",
				"message": "hello",
				"editor":  "",
			}),
			cmd("message", map[string]string{
				"channel": "friend",
				"action":  "refresh",
			}),
			cmd("message", map[string]string{
				"channel": "project",
				"action":  "refresh_message",
			}),
			cmd("message", map[string]string{
				"channel": "chat",
				"action":  "read_latest_log",
				"mode":    "one",
			}),
			cmd("message", map[string]string{
				"channel": "project",
				"action":  "check_permission",
				"mode":    "none",
			}),
			cmd("/get_container_data", map[string]interface{}{
				"name": "core",
			}),
			cmd("/portforward/init_list", map[string]string{}),
			cmd("/portforward/init_list", map[string]string{}),
		}
		for {
			msg, closed := conn.ReadMessage()
			if closed {
				break
			}
			if first {
				first = false
				conn.WriteMessage(socket_flush)
				continue
			}

			if msg == "" {
				continue
			}

			var data []interface{}
			json.Unmarshal([]byte(msg), &data)
			if len(data) == 0 {
				if len(cmds) > 0 {
					cmd := cmds[0]
					cmds = cmds[1:]
					conn.WriteMessage(cmd)
				}
				continue
			}

			switch data[0].(string) {
			case "goorm_ping":
				pong := fmt.Sprintf(`["goorm_pong",{"docker_id":"%v"}]`, c.Uid)
				conn.WriteMessage(pong)
			default:
				if len(cmds) > 0 {
					cmd := cmds[0]
					cmds = cmds[1:]
					conn.WriteMessage(cmd)
				}
			}
		}
	}()

	go socket.ping(conn)

	time.Sleep(20*time.Second)
	socket.polling2(`31:42["/portforward/init_list",{}]`)

	return domain, nil
}

func listen(c Container, domain string) (err error) {
	socket := Socket{
		Endpoint: "https://proxy.goorm.io/app/" + domain + "/9080",
		Attr: []string{
			"docker_id=" + c.Uid,
			"project_path=" + c.ProjectPath,
			"useAgent=false",
			"agentToken=undefined",
		},
	}
	err = socket.polling()
	if err != nil {
		return err
	}

	conn, _, err := socket.polling1()
	if err != nil {
		return err
	}

	if conn.WriteMessage(socket_2probe) {
		err = errors.New("closed")
		return err
	}

	go func() {
		first := true
		for {
			msg, closed := conn.ReadMessage()
			if closed {
				break
			}
			if first {
				first = false
				conn.WriteMessage(socket_flush)
			}
			log.Println("msg:", msg)
		}
	}()

	go socket.ping(conn)

	return nil
}

func exec(domain string) (conn *SocketConn, err error) {
	socket := Socket{
		Endpoint: "https://proxy.goorm.io/app/" + domain + "/7777",
		Attr: []string{
			"domain=" + domain,
		},
	}
	err = socket.polling()
	if err != nil {
		return nil, err
	}

	conn, _, err = socket.polling1()
	if err != nil {
		return nil, err
	}

	if conn.WriteMessage(socket_2probe) {
		err = errors.New("closed")
		return nil, err
	}

	go func() {
		for {
			msg, closed := conn.ReadMessage()
			if closed {
				break
			}
			log.Println("msg:", msg)
		}
	}()

	return conn, nil
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
		"t="+tencode(),
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

func (s *Socket) polling1() (conn *SocketConn, result string, err error) {
	values := []string{
		"EIO=3",
		"transport=websocket",
		"sid=" + s.sid,
	}

	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")
	c, raw, err := util.SOCKET(u, nil)
	return &SocketConn{Conn: c}, string(raw), err
}

func (s *Socket) polling2(body string) error {
	values := []string{
		"EIO=3",
		"transport=polling",
		"t="+ tencode(),
		"sid=" + s.sid,
	}
	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")

	header := map[string]string{
		"accept":       "*/*",
		"content-type": "text/plain;charset=UTF-8",
	}
	data, err := util.POST(u, body, header)
	if err != nil {
		log.Println("polling2:", err, u)
		return err
	}

	log.Println(string(data))
	return nil
}

func (s *Socket) polling3() error {
	values := []string{
		"EIO=3",
		"transport=polling",
		"t="+ tencode(),
		"sid=" + s.sid,
	}

	u := s.Endpoint + "/socket.io/?" + strings.Join(append(s.Attr, values...), "&")
	data, err := util.GET(u, nil)
	if err != nil {
		log.Println("polling3 Read:", err)
		return err
	}

	log.Println(string(data))
	return nil
}

func (s *Socket) ping(conn *SocketConn) {
	timer := time.NewTicker(time.Millisecond * time.Duration(s.interval))
	for {
		select {
		case <-timer.C:
			closed := conn.WriteMessage(socket_ping)
			if closed {
				break
			}
		}
	}
}

type SocketConn struct {
	*websocket.Conn
	sync.Mutex
}

const (
	socket_2probe = "2probe"
	socket_3probe = "3probe"
	socket_flush  = "5"

	socket_ping = "2"
	socket_pong = "3"
)

func (c *SocketConn) ReadMessage() (msg string, closed bool) {
	_, message, err := c.Conn.ReadMessage()
	if c.socketError(err) {
		return "", true
	}

	fmt.Println("read", string(message))

	switch string(message) {
	case socket_3probe, socket_pong:
		msg = ""
	default:
		msg = strings.TrimPrefix(string(message), "42")
	}

	return msg, false
}

func (c *SocketConn) WriteMessage(msg string) (closed bool) {
	switch msg {
	case socket_2probe, socket_flush, socket_ping:
	default:
		if !strings.HasPrefix(msg, "42") {
			msg = "42" + msg
		}
	}

	fmt.Println("write", string(msg))

	c.Lock()
	defer c.Unlock()
	err := c.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
	return c.socketError(err)
}

func (c *SocketConn) socketError(err error) (closed bool) {
	if err == nil {
		return false
	}

	if _, ok := err.(*websocket.CloseError); ok {
		c.Close()
		return true
	}

	if strings.Contains(err.Error(), "use of closed network") {
		c.Close()
		return true
	}

	return false
}

func (c *SocketConn) Close() {
	c.Conn.Close()
}
