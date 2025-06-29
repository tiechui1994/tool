package util

import (
	"testing"
	"time"
)

func init() {
	RegisterDNS([]string{
		"114.114.114.114:53",                 // 天翼
		"223.5.5.5:53", "223.6.6.6:53",       // 阿里云
		"218.30.19.40:53", "218.30.19.50:53", // 西安
	})
}

func TestGET(t *testing.T) {
	POST("https://api.quinn.eu.xx", WithRetry(2))
}

func TestLogRequest(t *testing.T) {
	RegisterDNSTimeout(time.Second)
	RegisterConnTimeout(time.Second, 2*time.Second)
	_, err := GET("https://www.baidu.com")
	t.Log(err)
}
