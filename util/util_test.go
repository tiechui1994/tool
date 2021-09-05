package util

import "testing"

func TestGET(t *testing.T) {
	DEBUG = true
	GET("https://blog.tiechui1994.tk", nil)
}
