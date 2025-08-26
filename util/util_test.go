package util

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func init() {
	RegisterDNS([]string{
		"114.114.114.114:53",           // 天翼
		"223.5.5.5:53", "223.6.6.6:53", // 阿里云
		"218.30.19.40:53", "218.30.19.50:53", // 西安
	})
}

func TestGET(t *testing.T) {
	t.Log(POST("https://api.quinn.eu.xx", WithRetry(2)))
}

func TestLogRequest(t *testing.T) {
	RegisterDNSTimeout(time.Second)
	RegisterConnTimeout(time.Second, 2*time.Second)
	_, err := GET("https://www.baidu.com")
	t.Log(err)
}

func TestCookie(t *testing.T) {
	RegisterCookieFun("cookie")
	var token string
	withAfterResponse := WithAfterResponse(func(w *http.Response) {
		t := w.Header.Get("x-csrf-token")
		if t != "" {
			token = t
		}
	})
	withBeforeRequest := WithBeforeRequest(func(r *http.Request) {
		if token != "" {
			r.Header.Set("x-csrf-token", token)
		}
	})

	_, err := GET("https://stream.streamlit.app", withBeforeRequest, withAfterResponse)
	t.Log(err)
	_, err = GET("https://stream.streamlit.app/api/v2/app/status", WithHeader(map[string]string{
		"origin": "https://stream.streamlit.app",
	}), withBeforeRequest, withAfterResponse)
	t.Log(err)

	raw, err := POST("https://stream.streamlit.app/api/v2/app/resume", WithHeader(map[string]string{
		"x-streamlit-machine-id": "11fc256f-c40b-467c-8754-a946bc4c6b2b",
		"origin":                 "https://stream.streamlit.app",
	}), withBeforeRequest, withAfterResponse)
	t.Log(err)
	t.Log(string(raw))
}

func TestCookieClean(t *testing.T) {
	RegisterCookieJar("ai")

	u, _ := url.Parse("https://aihub-run.gitcode.com/api/txt")
	t.Log(GetCookies(u))

	ClearCookie(u)
	time.Sleep(time.Second)
	t.Log(GetCookies(u))
}
