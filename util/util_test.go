package util

import (
	"context"
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
	t.Log(POST("https://static.broadlink.eu.org", WithRetry(2), WithDump(), WithProxy(func(r *http.Request) (*url.URL, error) {
		return url.Parse("https://84.239.14.158:9002")
	})))
}

func TestLogRequest(t *testing.T) {
	RegisterDNSTimeout(time.Second)
	RegisterConnTimeout(time.Second, 2*time.Second)
	_, err := GET("https://www.baidu.com")
	t.Log(err)
}

func TestCookie(t *testing.T) {
	c := NewClient(WithClientCookieFun("local"))
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

	_, err := c.GET("https://stream.streamlit.app", withBeforeRequest, withAfterResponse)
	t.Log(err)
	_, err = c.GET("https://stream.streamlit.app/api/v2/app/status", WithHeader(map[string]string{
		"origin": "https://stream.streamlit.app",
	}), withBeforeRequest, withAfterResponse)
	t.Log(err)

	raw, err := c.POST("https://stream.streamlit.app/api/v2/app/resume", WithHeader(map[string]string{
		"x-streamlit-machine-id": "11fc256f-c40b-467c-8754-a946bc4c6b2b",
		"origin":                 "https://stream.streamlit.app",
	}), withBeforeRequest, withAfterResponse)
	t.Log(err)
	t.Log(string(raw))
}

func TestClearCookie(t *testing.T) {
	c := NewClient(WithClientCookieJar("local"))
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			http.SetCookie(writer, &http.Cookie{
				Name:  "SESSION",
				Value: time.Now().Format("2006-01-02T15:04:05.99999999"),
			})
		})
		server := http.Server{
			Addr:    ":8080",
			Handler: mux,
		}
		server.ListenAndServe()
	}()

	time.Sleep(500 * time.Millisecond)
	_, _ = c.GET("http://127.0.0.1:8080/")

	_, _ = c.GET("http://127.0.0.1:8080/")

	u, _ := url.Parse("http://127.0.0.1:8080/")
	t.Log("GetCookies 1:", c.GetCookies(u))
	t.Logf("CleanCookie: %v", c.CleanCookie(u))
	time.Sleep(time.Second)
	t.Log("GetCookies 1:", c.GetCookies(u))

	_, _ = c.GET("http://127.0.0.1:8080/")
	t.Log("GetCookies 3:", c.GetCookies(u))
}

func TestContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	raw, err := GET("https://cloud.quinn.eu.org", WithContext(ctx), WithTimeout(1000*time.Millisecond), WithRetry(2))
	t.Log(err, string(raw))
}
