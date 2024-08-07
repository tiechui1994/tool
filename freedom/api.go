package freedom

import (
	"fmt"
	"github.com/tiechui1994/tool/util"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
)

const (
	endpoint = "https://my.freenom.com"
)

type Freedom struct {
	userName string
	password string
	client   *util.EmbedClient
}

func New(userName, password string) *Freedom {
	client := util.NewClient(util.WithClientCookieJar(userName))

	freedom := &Freedom{
		userName: userName,
		password: password,
		client:   client,
	}

	return freedom
}

func (f *Freedom) needAccessToken() {
	u := endpoint + "/clientarea.php"
	_, err := f.client.GET(u, util.WithHeader(f.header()), util.WithTest())
	var need bool
	if err == nil {
		need = false
	} else if v, ok := err.(*util.CodeError); ok {
		need = v.Code != 200
	}

	if !need {
		return
	}

	u = "https://aws-waf-solver.llf.app/token"
	header := f.header(map[string]string{
		"Accept":        "application/json",
		"Authorization": "https://github.com/luolongfei/freenom",
	})
	delete(header, "Accept-Encoding")
	raw, err := f.client.GET(u, util.WithHeader(header), util.WithDebug(), util.WithProxy(func(request *http.Request) (*url.URL, error) {
		fmt.Println("===", request.URL)
		return url.Parse("http://192.168.1.1:7890")
	}))
	fmt.Println(string(raw), err)
}

func (f *Freedom) header(others ...map[string]string) map[string]string {
	others = append(others, map[string]string{})
	header := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
		"Accept-Encoding": "gzip, deflate, br",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}
	for k, v := range others[0] {
		header[k] = v
	}
	return header
}

func (f *Freedom) Login() error {
	u := endpoint + "/clientarea.php"
	raw, err := f.client.GET(u, util.WithHeader(f.header()), util.WithDebug())
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}
	regex := regexp.MustCompile(`<input type="hidden" name="token" value="(.*?)"`)
	if !regex.Match(raw) {
		return fmt.Errorf("no valid token")
	}

	tokens := regex.FindAllStringSubmatch(string(raw), 1)
	token := tokens[0][1]
	u = endpoint + "/dologin.php"
	header := f.header(map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      "https://my.freenom.com/clientarea.php",
	})
	body := url.Values{
		"token":    []string{token},
		"username": []string{f.userName},
		"password": []string{f.password},
	}

	raw, err = f.client.POST(u, util.WithHeader(header), util.WithBody(body.Encode()), util.WithDebug())
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	uv, _ := url.Parse(endpoint)
	if f.client.GetCookie(uv, "WHMCSZH5eHTGhfvzP") != nil {
		return nil
	}

	return fmt.Errorf("login no cookie")
}

func (f *Freedom) Domain() error {
	u := endpoint + "/domains.php?a=renewals"
	headers := f.header(map[string]string{
		"Referer": "https://my.freenom.com/clientarea.php",
	})

	raw, err := f.client.GET(u, util.WithHeader(headers), util.WithRetry(2), util.WithDebug())
	if err != nil {
		return err
	}

	_ = ioutil.WriteFile("./www.html", raw, 0666)
	regex := regexp.MustCompile(`<li.*?Logout.*?<\/li>`)
	if !regex.Match(raw) {
		//return fmt.Errorf("invalid")
	}

	fmt.Println(string(raw))

	return nil
}
