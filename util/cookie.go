package util

import (
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"unicode"
	"unsafe"
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

func (s *simpleCookieJar) Cookies(req *http.Request) {
	for _, cookie := range s.privateJar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
}

func (s *simpleCookieJar) SetCookies(u *url.URL, resp *http.Response) {
	if rc := resp.Cookies(); len(rc) > 0 {
		s.privateJar.SetCookies(u, rc)
		if s.afterCookieSave != nil {
			s.afterCookieSave()
		}
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

func clean(u *url.URL, cc CustomerCookie) error {
	host, err := canonicalHost(u.Host)
	if err != nil {
		return err
	}

	switch s := cc.(type) {
	case *simpleCookieJar:
		jar := (*Jar)(unsafe.Pointer(s.privateJar))
		key := jarKey(host, jar.PsList)
		jar.Mu.Lock()
		jar.Entries[key] = map[string]entry{}
		jar.Mu.Unlock()

		if s.afterCookieSave != nil {
			s.afterCookieSave()
		}
	case *simpleCookieFun:
		jar := (*Jar)(unsafe.Pointer(s.privateJar))
		key := jarKey(host, jar.PsList)
		jar.Mu.Lock()
		jar.Entries[key] = map[string]entry{}
		jar.Mu.Unlock()

		if s.afterCookieSave != nil {
			s.afterCookieSave()
		}
	}

	return nil
}

func jarKey(host string, psl cookiejar.PublicSuffixList) string {
	if isIP(host) {
		return host
	}

	var i int
	if psl == nil {
		i = strings.LastIndex(host, ".")
		if i <= 0 {
			return host
		}
	} else {
		suffix := psl.PublicSuffix(host)
		if suffix == host {
			return host
		}
		i = len(host) - len(suffix)
		if i <= 0 || host[i-1] != '.' {
			// The provided public suffix list psl is broken.
			// Storing cookies under host is a safe stopgap.
			return host
		}
		// Only len(suffix) is used to determine the jar key from
		// here on, so it is okay if psl.PublicSuffix("www.buggy.psl")
		// returns "com" as the jar key is generated from host.
	}
	prevDot := strings.LastIndex(host[:i-1], ".")
	return host[prevDot+1:]
}

func isIP(host string) bool {
	if strings.ContainsAny(host, ":%") {
		// Probable IPv6 address.
		// Hostnames can't contain : or %, so this is definitely not a valid host.
		// Treating it as an IP is the more conservative option, and avoids the risk
		// of interpreting ::1%.www.example.com as a subdomain of www.example.com.
		return true
	}
	return net.ParseIP(host) != nil
}

func canonicalHost(host string) (string, error) {
	var err error
	if hasPort(host) {
		host, _, err = net.SplitHostPort(host)
		if err != nil {
			return "", err
		}
	}
	// Strip trailing dot from fully qualified domain names.
	host = strings.TrimSuffix(host, ".")
	if !Is(host) {
		return "", fmt.Errorf("not support not ascii host")
	}
	// We know this is ascii, no need to check.
	lower, _ := ToLower(host)
	return lower, nil
}

func hasPort(host string) bool {
	colons := strings.Count(host, ":")
	if colons == 0 {
		return false
	}
	if colons == 1 {
		return true
	}
	return host[0] == '[' && strings.Contains(host, "]:")
}

func IsPrint(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// Is returns whether s is ASCII.
func Is(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// ToLower returns the lowercase version of s if s is ASCII and printable.
func ToLower(s string) (lower string, ok bool) {
	if !IsPrint(s) {
		return "", false
	}
	return strings.ToLower(s), true
}
