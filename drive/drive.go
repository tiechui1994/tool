package drive

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

/**
doc:
https://webapps.stackexchange.com/questions/126394/cant-download-large-file-from-google-drive-as-one-single-folder-always-splits
*/

const (
	google = "https://developers.google.com"
)

var config struct {
	AccessToken  string
	RefreshToken string
	Expired      time.Time
	tokenuri     string
}

func init() {
	config.AccessToken = "ya29.a0ARrdaM-EWM4QCzV6prTezvE7DI2Ty4khhPu69LcvLzwGCgRgMmiV49LLWITP2DDO1dygi7adKCEPf_m4HBLQ5hodsqupft-cKKmBhuRZmJ3HBWsGZscwJTTRlaSAX8u1TEEeTckzn2avxrEwqcy2saI_fEKx"
	config.RefreshToken = "1//04tyFUCO2L6AICgYIARAAGAQSNwF-L9IrYYhoxfhvegjU1B3iZ8PipxJdlpZ6FO3sX2ZpEFxoGclCHAf7oF2Kl3OpLUYWCFQUnr0"
	config.Expired = time.Now().Add(3000 * time.Second)

	if _, err := os.Stat("/tmp/token"); err == nil {
		data, _ := ioutil.ReadFile("/tmp/token")
		json.Unmarshal(data, &config)
	}

	config.tokenuri = "https://oauth2.googleapis.com/token"
}

func BuildAuthorizeUri() (uri string, err error) {
	var body struct {
		Scope        []string `json:"scope"`
		ResponseType string   `json:"response_type"`
		AuthUri      string   `json:"auth_uri"`
		Prompt       string   `json:"prompt"`
		AccessType   string   `json:"access_type"`
	}

	body.Scope = []string{"https://www.googleapis.com/auth/drive.readonly"}
	body.ResponseType = "code"
	body.AuthUri = "https://accounts.google.com/o/oauth2/v2/auth"
	body.Prompt = "consent"
	body.AccessType = "offline"

	u := google + "/oauthplayground/buildAuthorizeUri"
	data, err := util.POST(u, util.WithBody(body))
	if err != nil {
		log.Errorln("BuildAuthorizeUri:", err)
		return uri, err
	}

	var result struct {
		AuthorizeUri string `json:"authorize_uri"`
		Success      bool   `json:"success"`
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return uri, err
	}

	if !result.Success {
		return uri, errors.New("BuildAuthorizeUri failed")
	}

	return result.AuthorizeUri, nil
}

func ExchangeAuthCode(code string) error {
	var body struct {
		Code     string `json:"code"`
		TokenUri string `json:"token_uri"`
	}

	body.Code = code
	body.TokenUri = config.tokenuri
	u := google + "/oauthplayground/exchangeAuthCode"

	raw, err := util.POST(u,  util.WithBody(body))
	if err != nil {
		log.Errorln("ExchangeAuthCode:", err)
		return err
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Success      bool   `json:"success"`
	}

	err = json.Unmarshal(raw, &result)
	if err != nil {
		return err
	}

	if !result.Success {
		log.Errorln("ExchangeAuthCode failed:%v", string(raw))
		return errors.New("ExchangeAuthCode failed")
	}

	config.AccessToken = result.AccessToken
	config.RefreshToken = result.RefreshToken
	config.Expired = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return nil
}

func RefreshAccessToken() error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
		TokenUri     string `json:"token_uri"`
	}

	body.RefreshToken = config.RefreshToken
	body.TokenUri = config.tokenuri
	u := google + "/oauthplayground/refreshAccessToken"

	raw, err := util.POST(u, util.WithBody(body))
	if err != nil {
		log.Errorln("RefreshAccessToken: %v", err)
		return err
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Success      bool   `json:"success"`
	}

	err = json.Unmarshal(raw, &result)
	if err != nil {
		return err
	}

	if !result.Success {
		log.Errorln("RefreshAccessToken failed:%v", string(raw))
		return errors.New("RefreshAccessToken failed")
	}

	config.AccessToken = result.AccessToken
	config.RefreshToken = result.RefreshToken
	config.Expired = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	log.Infoln("AccessToken:%v", config.AccessToken)
	log.Infoln("RefreshToken:%v", config.RefreshToken)
	log.Infoln("Expired:%v", config.Expired.Local())

	data, _ := json.Marshal(config)
	ioutil.WriteFile("/tmp/token", data, 0666)
	return nil
}

func Token(code string) error {
	if code != "" {
		err := ExchangeAuthCode(code)
		if err != nil {
			return err
		}
	}

	err := RefreshAccessToken()
	if err != nil {
		return err
	}

	go func() {
		tciker := time.NewTicker(3000 * time.Second)
		for {
			select {
			case <-tciker.C:
				RefreshAccessToken()
			}
		}
	}()

	return nil
}

func Download(dist string, file File) {
	timer := time.NewTimer(5 * time.Second)
	for {
		select {
		case <-timer.C:
			timer.Reset(30 * time.Minute)
			cmd := fmt.Sprintf(`curl -C - \
					-H 'Authorization: Bearer %v' \
					-o %v -L 'https://www.googleapis.com/drive/v3/files/%v?alt=media&acknowledgeAbuse=True'`,
				config.AccessToken, dist, file.ID)
			cm := exec.Command("bash", "-c", cmd)
			cm.Stdin = os.Stdin
			cm.Stdout = os.Stdout
			cm.Stderr = os.Stderr
			cm.Run()
			return
		}
	}
}

const (
	folder = "application/vnd.google-apps.folder"
)

type File struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Parents      []string `json:"parents"`
	MimeType     string   `json:"mimeType"`
	WebViewLink  string   `json:"webViewLink"`
	CreatedTime  string   `json:"createdTime"`
	ModifiedTime string   `json:"modifiedTime"`
	Shared       bool     `json:"shared"`
}

func Files() (files []File, err error) {
	values := []string{
		"pageSize=1000",
		"fields=files(id,name,mimeType,parents,webViewLink,createdTime,modifiedTime,shared)",
	}
	u := "https://www.googleapis.com/drive/v3/files?" + strings.Join(values, "&")
	header := map[string]string{
		"Authorization": "Bearer " + config.AccessToken,
	}
	raw, err := util.GET(u, util.WithHeader(header))
	if err != nil {
		log.Infoln("%v", string(raw))
		return nil, err
	}

	var result struct {
		Files []File `json:"files"`
	}

	err = json.Unmarshal(raw, &result)
	return result.Files, err
}

func Exec() {
	files, err := Files()
	if err != nil {
		os.Exit(1)
	}

	tpl1 := `
{{ range $idx, $ele := . }}
{{ $x := "f" }}
{{- if (eq .MimeType "application/vnd.google-apps.folder") -}} 
	{{- $x = "d" -}}
{{- end -}}
{{ printf "%-2d %-4s %-28s %s" $idx $x .ModifiedTime .Name -}}
{{ end }}
`
	tpl1 = strings.Trim(tpl1, "\n")
	list, err := template.New("").Parse(tpl1)
	if err != nil {
		os.Exit(1)
	}

	tpl2 := `
operate:
{{- range $idx, $ele := . }}
  {{ printf "%-2d %s" $idx $ele -}}
{{ end }}
`
	tpl2 = strings.Trim(tpl2, "\n")
	op, err := template.New("").Parse(tpl2)
	if err != nil {
		os.Exit(1)
	}
	ops := []string{
		"list files",
		"download file",
		"exit",
	}

	var buf bytes.Buffer
	funcs := []func(){
		0: func() {
			err = list.Execute(&buf, files)
			if err != nil {
				os.Exit(1)
			}
			fmt.Println(buf.String())
		},
		1: func() {
		again:
			var idx int
			fmt.Printf("Please Select Download File:")
			fmt.Scanf("%d", &idx)
			if idx < 0 || idx >= len(files) {
				fmt.Println("please select download file. eg: 0")
				goto again
			}
			Download("./"+files[idx].Name, files[idx])
		},
		2: func() {
			os.Exit(0)
		},
	}

retry:
	var idx int
	buf.Reset()
	// op
	err = op.Execute(&buf, ops)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(buf.String())
	fmt.Printf("Please Select Opertion:")
	fmt.Scanf("%d", &idx)
	if idx < 0 || idx >= len(ops) {
		fmt.Println("input fortmat error. eg: 0")
		goto retry
	}

	// operate
	funcs[idx]()
	goto retry
}

