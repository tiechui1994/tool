package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"

	"github.com/tiechui1994/tool/util"
)

/**
百度语音转换测试API, 支持中文,英文
网站: https://developer.baidu.com/vcast
**/

// https://developer.baidu.com/vcast 登录百度账号, 获取 Cookie 信息
var COOKIE = ""

func ConvertText(text string, filename string) (err error) {
	values := make(url.Values)
	values.Set("title", text)
	values.Set("content", ".")
	values.Set("sex", "0")    // 0 非情感女声, 1 非情感男声 3 情感男声  4 情感女声
	values.Set("speed", "6")  // 声速: 0-10
	values.Set("volumn", "8") // 声音大小
	values.Set("pit", "8")
	values.Set("method", "TRADIONAL")

	u := "https://developer.baidu.com/vcast/getVcastInfo"
	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
		"Accept":       "application/json, text/javascript, */*; q=0.01",
		"Cookie":       COOKIE,
	}

	data, err := util.POST(u, util.WithBody(values.Encode()), util.WithHeader(header))
	if err != nil {
		return err
	}

	log.Printf("uploadtext: %s", data)
	var res struct {
		BosUrl string `json:"bosUrl"`
		Status string `json:"status"`
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return err
	}

	if res.Status != "success" {
		log.Printf("failed: %v", string(data))
		return fmt.Errorf("")
	}

	return Download(res.BosUrl, filename)
}

func Download(u string, filename string) (err error) {
	data, err := util.GET(u)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename+".mp3", data, 0666)
}
