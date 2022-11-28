package clash

import (
	"encoding/json"
	"strings"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

const (
	endpoint = "http://127.0.0.1:9090"
)

var (
	defaultProvider = "🔰 选择节点"
)

func DefaultProvider(args ...string) string {
	if len(args) > 0 {
		defaultProvider = args[0]
	}

	return defaultProvider
}

func SpeedTest(proxy string) (delay int, err error) {
	values := []string{
		"timeout=5000",
		"url=http://www.gstatic.com/generate_204",
	}
	u := endpoint + "/proxies/" + proxy + "/delay?" + strings.Join(values, "&")
	raw, err := util.GET(u)
	if err != nil {
		log.Errorln("proxy: [%v], error: %v", proxy, err)
		return delay, err
	}

	var result struct {
		Delay int `json:"delay"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		log.Errorln("decode: [%v], error: %v", proxy, err)
		return delay, err
	}

	return result.Delay, nil
}

type Provider struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	VehicleType string `json:"vehicleType"`
	Proxies     []struct {
		History []struct {
			Delay int    `json:"delay"`
			Time  string `json:"time"`
		} `json:"history"`
		Name string `json:"name"`
		Type string `json:"type"`
		UDP  bool   `json:"udp"`
	} `json:"proxies"`
}

func Providers() (val Provider, err error) {
	u := endpoint + "/providers/proxies"
	raw, err := util.GET(u)
	if err != nil {
		log.Errorln("error: %v", err)
		return val, err
	}

	var result struct {
		Providers map[string]Provider `json:"providers"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		log.Errorln("decode error: %v", err)
		return val, err
	}
	return result.Providers[defaultProvider], nil
}

type Proxy struct {
	All  []string `json:"all"`
	Name string   `json:"name"`
	Now  string   `json:"now"`
	Type string   `json:"type"`
}

func Proxys() (val []Proxy, err error) {
	u := endpoint + "/proxies"
	raw, err := util.GET(u)
	if err != nil {
		log.Errorln("error: %v", err)
		return val, err
	}

	var result struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		log.Errorln("decode error: %v", err)
		return val, err
	}

	for _, v := range result.Proxies {
		if v.Type == "Selector" {
			val = append(val, v)
		}
	}

	return val, nil
}

func SetProxy(provider, proxy string) {
	u := endpoint + "/proxies/" + provider
	_, err := util.PUT(u,
		util.WithBody(map[string]string{
			"name": proxy,
		}))
	if err != nil {
		log.Errorln("error: %v", err)
	}
}
