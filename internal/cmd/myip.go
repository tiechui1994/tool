package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiechui1994/tool/util"
)

func echo(s string) string {
	return strings.TrimSpace(s)
}

func ipcn(s string) string {
	var data struct {
		Address string `json:"address"`
		IP      string `json:"ip"`
	}

	json.Unmarshal([]byte(s), &data)
	return data.IP
}

var sites = map[string]func(string) string{
	"https://ip.cn/api/index?ip=&type=0": ipcn, // 中国
	"https://inet-ip.info/ip":            echo, // 日本
	"https://icanhazip.com":              echo, // 美国
}

func main() {
	for key, value := range sites {
		raw, err := util.GET(key, nil)
		if err != nil {
			continue
		}
		fmt.Printf("%v", value(string(raw)))
		break
	}
}
