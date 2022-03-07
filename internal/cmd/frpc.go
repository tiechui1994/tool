package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

var token string

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

		var ip string
	again:
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

		for i := range nodes {
			if Auth(nodes[i], ip) == nil {
				log.Infoln("auth (type:%v, name:%v, remote:%v) suceess", nodes[i].Type,
					nodes[i].Name, nodes[i].Remote)
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
		"cookie":     "PHPSESSID=" + token + ";",
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
	return response.Data, err
}

func Auth(node Node, ip string) error {
	body := []string{
		"id=" + strconv.Itoa(node.ID),
		"ip=" + ip,
	}

	header := map[string]string{
		"content-type": "application/x-www-form-urlencoded",
		"accept":       "application/json",
		"cookie":       "PHPSESSID=" + token + ";",
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
	return err
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
