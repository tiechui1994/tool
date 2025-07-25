package util

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

type CustomerCookie interface {
	Cookies(req *http.Request)
	SetCookies(u *url.URL, resp *http.Response)
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

func (s *simpleCookieFun) Cookies(req *http.Request) {
	values := make([]string, 0)
	for _, cookie := range s.privateJar.Cookies(req.URL) {
		s := fmt.Sprintf("%s=%s", sanitizeCookieName(cookie.Name), cookie.Value)
		values = append(values, s)
	}
	req.Header.Set("Cookie", strings.Join(values, "; "))
}

func (s *simpleCookieFun) SetCookies(u *url.URL, resp *http.Response) {
	if rc := resp.Cookies(); len(rc) > 0 {
		s.privateJar.SetCookies(u, rc)
		if s.afterCookieSave != nil {
			s.afterCookieSave()
		}
	}
}
