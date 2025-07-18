package util

import (
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

func sanitizeCookieValue(v string) string {
	v = sanitizeOrWarn("Cookie.Value", validCookieValueByte, v)
	if len(v) == 0 {
		return v
	}
	//note: fix some website bugs.
	//if strings.ContainsAny(v, " ,") {
	//	return `"` + v + `"`
	//}
	v = strings.ReplaceAll(v, " ,", ",")
	return v
}
func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}
func sanitizeOrWarn(fieldName string, valid func(byte) bool, v string) string {
	ok := true
	for i := 0; i < len(v); i++ {
		if valid(v[i]) {
			continue
		}
		log.Printf("net/http: invalid byte %q in %s; dropping invalid bytes", v[i], fieldName)
		ok = false
		break
	}
	if ok {
		return v
	}
	buf := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		if b := v[i]; valid(b) {
			buf = append(buf, b)
		}
	}
	return string(buf)
}

var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

func sanitizeCookieName(n string) string {
	return cookieNameSanitizer.Replace(n)
}

type simpleCookieJar struct {
	name            string
	privateJar      *cookiejar.Jar
	afterCookieSave func()
}

func (s *simpleCookieJar) Cookies(u *url.URL) []*http.Cookie {
	return s.privateJar.Cookies(u)
}

func (s *simpleCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.privateJar.SetCookies(u, cookies)
	if s.afterCookieSave != nil {
		s.afterCookieSave()
	}
}

type simpleCookieFun struct {
	name            string
	privateJar      *cookiejar.Jar
	afterCookieSave func()
}

func (s *simpleCookieFun) Cookies(url *url.URL) []*http.Cookie {
	cookies := s.privateJar.Cookies(url)
	for i, val := range cookies {
		val.Value = sanitizeCookieValue(val.Value)
		cookies[i] = val
	}
	return cookies
}

func (s *simpleCookieFun) SetCookies(url *url.URL, cookies []*http.Cookie) {
	s.privateJar.SetCookies(url, cookies)
	if s.afterCookieSave != nil {
		s.afterCookieSave()
	}
}
