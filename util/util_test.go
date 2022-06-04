package util

import "testing"

func TestGET(t *testing.T) {
	GET("https://blog.tiechui1994.tk", nil)
}

func TestLogRequest(t *testing.T) {
	GET("https://www.natfrp.com/cgi/tunnel/auth", nil)
}

