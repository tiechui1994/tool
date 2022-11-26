package util

import "testing"

func init() {
	RegisterDNS([]string{
		"114.114.114.114:53",                 // 天翼
		"223.5.5.5:53", "223.6.6.6:53",       // 阿里云
		"218.30.19.40:53", "218.30.19.50:53", // 西安
	})
}

func TestGET(t *testing.T) {
	GET("https://www.tiechui1994.tk", nil, 3)
}

func TestLogRequest(t *testing.T) {
	GET("https://www.natfrp.com/cgi/tunnel/auth", nil)
}
