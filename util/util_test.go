package util

import "testing"

func TestGET(t *testing.T) {
	DEBUG = true
	GET("https://www.baidu.com", nil)
}
