package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

var first bool
var token string
var next time.Time

func random(length int) string {
	str := "01234567890abcdefghijklmnopqrstuvwxyz"

	var ans []byte
	val := rand.Int63()
	for len(ans) < length {
		if val == 0 {
			val = rand.Int63()
		}
		v := val & 63
		val = val >> 7
		if v < 36 {
			ans = append(ans, str[v])
		}
	}

	return string(ans)
}

func init() {
	rand.Seed(time.Now().Unix())
	first = true
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if strings.HasPrefix(req.URL.String(), "https://www.natfrp.com/cgi/user/login") {
			x := random(160)
			req.Header.Set("Cookie", "PHPSESSID="+x+";")
			log.Infoln("set login cookie: %v", x)
		}
		log.Infoln("url: %v", req.URL.String())
		for _, req := range via {
			log.Infoln("-> url: %v", req.URL.String())
		}
		return nil
	}

	util.LogRequest(func(i *http.Request) {
		if i.URL.Host == "www.natfrp.com" || i.URL.Host == "openid.13a.com" {
			log.Infoln("request: %v, cookie:%v", i.URL.Path, i.Cookies())
		}
	})
	util.LogResponse(func(i *http.Response) {
		if i.Request.URL.Host == "www.natfrp.com" || i.Request.Host == "openid.13a.com" {
			log.Infoln("response: %v, cookie:%v", i.Request.URL.Path, i.Cookies())
		}
	})
}

var ipsites = map[string]func(string) string{
	"https://ip.cn/api/index?ip=&type=0": func(s string) string {
		var data struct {
			Address string `json:"address"`
			IP      string `json:"ip"`
		}

		json.Unmarshal([]byte(s), &data)
		return data.IP
	},                                            // 中国
	"https://inet-ip.info/ip": strings.TrimSpace, // 日本
	"https://icanhazip.com":   strings.TrimSpace, // 美国
}

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "token,t",
			Usage:       "frp token",
			Required:    true,
			Destination: &token,
		},
	}

	app.Action = func(c *cli.Context) error {
		if token == "" {
			log.Errorln("token must be set")
			return fmt.Errorf("invalid params")
		}

		go Server()

		var ip string
	again:
		err := LoginCheck()
		if err != nil {
			log.Errorln("login check failed: %v", err)
			return err
		}

		for key, value := range ipsites {
			raw, err := util.GET(key, nil)
			if err != nil {
				continue
			}

			newip := value(string(raw))
			if newip == ip {
				time.Sleep(time.Minute)
				goto again
			}

			ip = newip
			log.Infoln("curent ip: %v", ip)
			break
		}

		nodes, err := Nodes()
		if err != nil {
			log.Errorln("get nodes failed: %v", err)
			return err
		}

		u, _ := url.Parse("https://www.natfrp.com/")
		cookie := util.GetCookie(u, "PHPSESSID")

		log.Infoln("nodes: %v, %v", nodes, cookie.Value)

		for i := range nodes {
			err = Auth(nodes[i], ip)
			if err == nil {
				log.Infoln("auth (type:%v, name:%v, remote:%v) suceess", nodes[i].Type,
					nodes[i].Name, nodes[i].Remote)
			} else {
				log.Errorln("auth (type:%v, name:%v, remote:%v) failed: %v", nodes[i].Type,
					nodes[i].Name, nodes[i].Remote, err)
			}
		}

		time.Sleep(time.Minute)
		goto again
	}

	app.Run(os.Args)
}

type Node struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Remote string `json:"remote"`
	Type   string `json:"type"`
}

func Nodes() ([]Node, error) {
	header := map[string]string{
		"accept":     "application/json",
		"referer":    "https://www.natfrp.com/tunnel/",
		"origin":     "https://www.natfrp.com",
		"user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36",
	}
	raw, err := util.GET("https://www.natfrp.com/cgi/tunnel/list", header)
	if err != nil {
		return nil, err
	}

	var response struct {
		Code int    `json:"code"`
		Data []Node `json:"data"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, err
	}
	if response.Code != 200 {
		return nil, util.CodeError(response.Code)
	}

	return response.Data, err
}

func LoginCheck() error {
	if next.IsZero() || time.Now().After(next) {
		next = time.Now().Add(60*time.Minute + time.Duration(rand.Int31n(300))*time.Second)
	} else {
		return nil
	}

	header := map[string]string{
		"accept":     "text/html",
		"referer":    "https://www.natfrp.com/user/login",
		"user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36",
	}

	if first {
		header["cookie"] = "PHPSESSID=" + token + ";"
	}
	raw, err := util.GET("https://www.natfrp.com/cgi/user/login", header)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`path\s*=\s*'(/authorize.*)'`)
	if re.MatchString(string(raw)) {
		tokens := re.FindAllStringSubmatch(string(raw), 1)
		u := "https://openid.13a.com/oauth" + tokens[0][1]
		header := map[string]string{
			"accept":     "text/html",
			"referer":    "https://www.natfrp.com/",
			"user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36",
		}
		if first {
			header["cookie"] = "PHPSESSID=" + token + ";"
			first = !first
		}
		raw, err = util.GET(u, header)
		if err != nil {
			return err
		}
	}

	return nil
}

func Auth(node Node, ip string) error {
	body := []string{
		"id=" + strconv.Itoa(node.ID),
		"ip=" + ip,
	}

	header := map[string]string{
		"content-type": "application/x-www-form-urlencoded",
		"accept":       "application/json",
		"referer":      "https://www.natfrp.com/tunnel/",
		"origin":       "https://www.natfrp.com",
		"user-agent":   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36",
	}
	raw, err := util.POST("https://www.natfrp.com/cgi/tunnel/auth", strings.Join(body, "&"), header)
	if err != nil {
		return err
	}

	var response struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return err
	}
	if response.Code != 200 {
		fmt.Println(string(raw))
		return util.CodeError(response.Code)
	}

	return nil
}

func SignHandler(w http.ResponseWriter, r *http.Request) {
	header := map[string]string{
		"accept":     "application/json, text/plain, */*",
		"referer":    "https://www.natfrp.com/user/",
		"origin":     "https://www.natfrp.com",
		"user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36",
	}

	var raw []byte
	var err error
	switch r.Method {
	case http.MethodGet:
		raw, err = util.GET("https://www.natfrp.com/cgi/user/sign?gt", header)
		if err != nil {
			io.WriteString(w, `{"code":401, "data":{}}`)
			log.Errorln("get code: %v", err)
			return
		}
	case http.MethodPost:
		header["content-type"] = "application/x-www-form-urlencoded"
		raw, err = util.POST("https://www.natfrp.com/cgi/user/sign", r.Body, header)
		if err != nil {
			w.WriteHeader(401)
			log.Errorln("get code: %v", err)
			return
		}
	}

	w.Write(raw)
}

func Server() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ajax/sign", SignHandler)
	server := http.Server{
		Addr:    ":1234",
		Handler: mux,
	}
	server.ListenAndServe()
}
