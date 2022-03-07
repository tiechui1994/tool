package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	agent  string
	agents = []string{
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/534.57.2 (KHTML, like Gecko) Version/5.1.7 Safari/534.57.2",

		"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:34.0) Gecko/20100101 Firefox/34.0",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0",
	}
)

func init() {
	resolver := net.Resolver{
		PreferGo: false,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}

			conn, err := d.DialContext(ctx, network, "8.8.8.8:53")
			return conn, err
		},
	}
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 5 * time.Minute,
				Resolver:  &resolver,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 60 * time.Second,
	}
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

type CodeError int

func (err CodeError) Error() string {
	return http.StatusText(int(err))
}

func request(method, u string, body interface{}, header map[string]string) (raw json.RawMessage, err error) {
	var reader io.Reader
	if body != nil {
		switch body := body.(type) {
		case io.Reader:
			reader = body
		case string:
			reader = strings.NewReader(body)
		case []byte:
			reader = bytes.NewReader(body)
		default:
			bin, _ := json.Marshal(body)
			reader = bytes.NewReader(bin)
		}
	}

	request, _ := http.NewRequest(method, u, reader)
	request.Header.Set("user-agent", UserAgent())
	for k, v := range header {
		request.Header.Set(k, v)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return raw, err
	}

	raw, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return raw, err
	}

	if response.StatusCode >= 400 {
		return raw, CodeError(response.StatusCode)
	}

	return raw, nil
}

func Main(params map[string]interface{}) map[string]interface{} {
	var (
		body   interface{}
		header = map[string]string{
			"x-proxy-host": "www.ibm.com",
		}
		url    = "https://www.google.com"
		method = "GET"
	)
	if val, ok := params["url"]; ok {
		url, _ = val.(string)
	}
	if val, ok := params["method"]; ok {
		method, _ = val.(string)
	}
	if val, ok := params["header"]; ok {
		header, _ = val.(map[string]string)
	}
	bin, _ := json.Marshal(params["body"])
	body = strings.ReplaceAll(string(bin), "$now", time.Now().Format(time.RFC3339))

	raw, err := request(method, url, body, header)
	fmt.Println("raw", len(raw), "err", err)
	if err != nil {
		return map[string]interface{}{
			"error": err,
		}
	}

	return map[string]interface{}{
		"msg": string(raw),
	}
}
