package speech

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tiechui1994/tool/util"
)

type FileInfo struct {
	Icon     string `json:"icon"`
	ID       string `json:"id"`
	Name     string `json:"name_all"`
	Share    string `json:"share"`
	Download string `json:"download"`
}

var (
	hosts = []string{
		"lanzouy.com",
		"lanzouw.com",
		"lanzoui.com",
		"lanzoux.com",
		"lanzouo.com",
		"lanzoul.com",
	}
)

func FetchLanZouInfo(shareURL, pwd string) ([]FileInfo, error) {
	rURL := regexp.MustCompile(`^(https?://[a-zA-Z0-9-]*?\.?lanzou[a-z].com)/`)
	urls := rURL.FindAllStringSubmatch(shareURL, 1)
	if len(urls) == 0 || len(urls[0]) < 2 {
		return nil, fmt.Errorf("invalid share url")
	}
	endpoint := urls[0][1]

	// random host
	host := func(cur string) (val string) {
		defer func() {
			uRL, _ := url.Parse(shareURL)
			uRL.Host = val
			shareURL = uRL.String()
			endpoint = "https://" + val
		}()

		name := cur[:strings.Index(cur, ".")]
		for i, v := range hosts {
			if strings.HasSuffix(cur, v) && i < len(hosts)-1 {
				return name + "." + hosts[i+1]
			}
		}

		return name + "." + hosts[0]
	}

	raw, err := util.GET(shareURL, util.WithRetry(3), util.WithRandomHost(host))
	if err != nil {
		return nil, fmt.Errorf("get share failed: %w", err)
	}

	// check
	rLink := regexp.MustCompile(`id="passwddiv"`)
	if rLink.MatchString(string(raw)) {
		return fileDirectURL(raw, shareURL, pwd, endpoint)
	}

	return fileInDirectURL(raw, shareURL, pwd, endpoint)
}

// 直接
func fileDirectURL(raw []byte, shareURL, pwd, endpoint string) ([]FileInfo, error) {
	rNote1 := regexp.MustCompile(`(?s:<!--.*?-->)`)
	rNote2 := regexp.MustCompile(`(//.*)`)
	raw = rNote1.ReplaceAll(raw, []byte(""))
	raw = rNote2.ReplaceAll(raw, []byte(""))

	// ajax
	r := regexp.MustCompile(`(?s:ajax\((\{.*?dataType\s*:\s*'json'|"json"\s*)\,)`)
	values := r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return nil, fmt.Errorf("javascript ajax regex failed")
	}

	// data
	values[0][1] = strings.ReplaceAll(values[0][1], `'`, `"`)
	r = regexp.MustCompile(`data\s*:\s*(".*?")`)
	values = r.FindAllStringSubmatch(values[0][1], 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return nil, fmt.Errorf("ajax post data regex failed")
	}

	form, _ := strconv.Unquote(values[0][1])
	u := endpoint + "/ajaxm.php"
	body := form + pwd
	log.Printf("request ajaxm url:%v body: %v", u, body)
	raw, err := util.POST(u, util.WithBody(body), util.WithRetry(3),
		util.WithHeader(map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         endpoint,
			"Referer":        shareURL,
			"Content-Length": fmt.Sprintf("%v", len(body)),
		}))
	if err != nil {
		return nil, fmt.Errorf("request ajaxm failed: %w", err)
	}

	if gjson.GetBytes(raw, "zt").Int() != 1 {
		return nil, fmt.Errorf("request ajaxm error: %v", gjson.GetBytes(raw, "inf").String())
	}

	var response struct {
		Info string `json:"inf"`
		Code int    `json:"zt"`
		Dom  string `json:"dom"`
		Url  string `json:"url"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, fmt.Errorf("decode ajaxm data failed: %w", err)
	}

	result := []FileInfo{
		{
			Name:     response.Info,
			Share:    shareURL,
			Download: response.Dom + "/file/" + response.Url,
		},
	}

	return result, nil
}

// 间接
func fileInDirectURL(raw []byte, shareURL, pwd, endpoint string) ([]FileInfo, error) {
	// ajax
	r := regexp.MustCompile(`(?s:ajax\((\{.*?\})\,)`)
	values := r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return nil, fmt.Errorf("javascript ajax regex failed")
	}

	// data
	r = regexp.MustCompile(`(?s:data\s*\:\s*(\{.*?\}))`)
	values = r.FindAllStringSubmatch(values[0][1], 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return nil, fmt.Errorf("ajax post data regex failed")
	}

	// key, value
	values[0][1] = strings.ReplaceAll(values[0][1], `'`, `"`)
	r = regexp.MustCompile(`(".*?"|\w)\s*\:\s*(.*?)(\s*,|\s*\})`)
	values = r.FindAllStringSubmatch(values[0][1], -1)
	if len(values) == 0 {
		return nil, fmt.Errorf("post data key:value regex failed")
	}

	rValueIsNumOrStr := regexp.MustCompile(`^(".*"|\d+)$`)
	originKV := make(map[string]json.RawMessage)
	search := make(map[string]*regexp.Regexp)
	for _, v := range values {
		key, value := v[1], v[2]
		// handle key
		if strings.Contains(key, `"`) {
			key, _ = strconv.Unquote(key)
		}

		// handle value
		if key == "pwd" {
			originKV[key] = []byte(fmt.Sprintf(`"%v"`, pwd))
			continue
		}
		if rValueIsNumOrStr.MatchString(value) {
			originKV[key] = []byte(value)
			continue
		}

		search[key] = regexp.MustCompile(fmt.Sprintf(`(const|let|var)?\s*%v\s*=\s*(\d|'.*?'|".*?")`, value))
	}

	for i := range search {
		values := search[i].FindAllStringSubmatch(string(raw), 1)
		if len(values) > 0 && len(values[0]) >= 3 {
			originKV[i] = []byte(strings.ReplaceAll(values[0][2], `'`, `"`))
		}
	}

	raw, err := json.Marshal(originKV)
	if err != nil {
		return nil, fmt.Errorf("marshal origin ajax data failed: %v", err)
	}

	var kv map[string]interface{}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	err = d.Decode(&kv)
	if err != nil {
		return nil, fmt.Errorf("convert ajax data to form data failed: %v", err)
	}
	form := make(url.Values)
	for k, v := range kv {
		form.Set(k, fmt.Sprintf("%v", v))
	}

	u := endpoint + "/filemoreajax.php"
	body := form.Encode()
	log.Printf("request filemoreajax url: %v, body: %v", u, body)
	raw, err = util.POST(u,
		util.WithBody(body),
		util.WithRetry(3),
		util.WithHeader(map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         endpoint,
			"Referer":        shareURL,
			"Content-Length": fmt.Sprintf("%v", len(body)),
		}))
	if err != nil {
		return nil, fmt.Errorf("request filemoreajax failed: %w", err)
	}

	if gjson.GetBytes(raw, "zt").Int() != 1 {
		return nil, fmt.Errorf("request filemoreajax error: %v", gjson.GetBytes(raw, "inf").String())
	}

	var response struct {
		Info string     `json:"inf"`
		Code int        `json:"zt"`
		Text []FileInfo `json:"text"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, fmt.Errorf("decode filemoreajax data failed: %w", err)
	}

	time.Sleep(time.Millisecond * 1500)

	for i, v := range response.Text {
		response.Text[i].Share = endpoint + "/" + v.ID
		response.Text[i].Download, err = fetchFileURL(response.Text[i].Share)
		if err != nil {
			log.Printf("fetch url %v failed: %v",
				response.Text[i].Share, err)
		}
		time.Sleep(time.Millisecond * 2000)
	}

	return response.Text, nil
}

func fetchFileURL(shareURL string) (string, error) {
	rURL := regexp.MustCompile(`^(https?://[a-zA-Z0-9-]*?\.?lanzou[a-z].com)/`)
	urls := rURL.FindAllStringSubmatch(shareURL, 1)
	if len(urls) == 0 || len(urls[0]) < 2 {
		return "", fmt.Errorf("invalid share url")
	}
	endpoint := urls[0][1]

	// random host
	host := func(cur string) (val string) {
		defer func() {
			uRL, _ := url.Parse(shareURL)
			uRL.Host = val
			shareURL = uRL.String()
			endpoint = "https://" + val
		}()

		name := cur[:strings.Index(cur, ".")]
		for i, v := range hosts {
			if strings.HasSuffix(cur, v) && i < len(hosts)-1 {
				return name + "." + hosts[i+1]
			}
		}

		return name + "." + hosts[0]
	}

	raw, err := util.GET(shareURL, util.WithRetry(3), util.WithRandomHost(host))
	if err != nil {
		return "", fmt.Errorf("get code file url failed: %w", err)
	}

	rNote1 := regexp.MustCompile(`(?s:<!--.*?-->)`)
	rNote2 := regexp.MustCompile(`(//.*)`)
	raw = rNote1.ReplaceAll(raw, []byte(""))
	raw = rNote2.ReplaceAll(raw, []byte(""))

	// iframe
	r := regexp.MustCompile(`<iframe.*?\s*src="(.*?)"`)
	values := r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return "", fmt.Errorf("iframe source regex failed")
	}

	time.Sleep(time.Second)

	fn := endpoint + values[0][1]
	log.Printf("fn url: %v", fn)
	raw, err = util.GET(fn, util.WithRetry(2), util.WithHeader(map[string]string{"Referer": shareURL}))
	if err != nil {
		return "", fmt.Errorf("get iframe failed: %w", err)
	}
	raw = rNote1.ReplaceAll(raw, []byte(""))
	raw = rNote2.ReplaceAll(raw, []byte(""))

	// ajax
	r = regexp.MustCompile(`(?s:ajax\((\{.*?\})\,)`)
	values = r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return "", fmt.Errorf("javascript ajax regex failed")
	}

	// data
	r = regexp.MustCompile(`(?s:data\s*\:\s*(\{.*?\}))`)
	values = r.FindAllStringSubmatch(values[0][1], 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return "", fmt.Errorf("ajax post data regex failed")
	}

	// key, value
	values[0][1] = strings.ReplaceAll(values[0][1], `'`, `"`)
	log.Printf("ajax regex data: %v", values[0][1])
	r = regexp.MustCompile(`(".*?"|\w)\s*\:\s*(.*?)(\s*,|\s*\})`)
	values = r.FindAllStringSubmatch(values[0][1], -1)
	if len(values) == 0 {
		return "", fmt.Errorf("post data key:value regex failed")
	}

	rValueIsNumOrStr := regexp.MustCompile(`^(".*"|\d+)$`)
	originKV := make(map[string]json.RawMessage)
	search := make(map[string]*regexp.Regexp)
	for _, v := range values {
		key, value := v[1], v[2]
		if strings.Contains(key, `"`) {
			key, _ = strconv.Unquote(key)
		}

		if rValueIsNumOrStr.MatchString(value) {
			originKV[key] = []byte(value)
			continue
		}

		search[key] = regexp.MustCompile(fmt.Sprintf(`(const|let|var)?\s*%v\s*=\s*(\d|'.*?'|".*?")`, value))
	}

	for i := range search {
		values := search[i].FindAllStringSubmatch(string(raw), 1)
		if len(values) > 0 && len(values[0]) >= 3 {
			originKV[i] = []byte(strings.ReplaceAll(values[0][2], `'`, `"`))
		}
	}

	raw, err = json.Marshal(originKV)
	if err != nil {
		return "", fmt.Errorf("marshal origin data: %w", err)
	}
	var kv map[string]interface{}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	err = d.Decode(&kv)
	if err != nil {
		return "", fmt.Errorf("convert origin data to form data failed: %w", err)
	}

	form := make(url.Values)
	for k, v := range kv {
		form.Set(k, fmt.Sprintf("%v", v))
	}

	time.Sleep(time.Second)

	u := endpoint + "/ajaxm.php"
	body := form.Encode()
	log.Printf("request ajaxm url:%v body: %v", u, body)
	raw, err = util.POST(u, util.WithBody(body), util.WithRetry(3),
		util.WithHeader(map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         endpoint,
			"Referer":        fn,
			"Content-Length": fmt.Sprintf("%v", len(body)),
		}))
	if err != nil {
		return "", fmt.Errorf("request ajaxm failed: %w", err)
	}

	if gjson.GetBytes(raw, "zt").Int() != 1 {
		return "", fmt.Errorf("request ajaxm error: %v", gjson.GetBytes(raw, "inf").String())
	}

	var response struct {
		Code int    `json:"zt"`
		Dom  string `json:"dom"`
		Url  string `json:"url"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return "", fmt.Errorf("decode ajaxm data failed: %w", err)
	}

	return response.Dom + "/file/" + response.Url, nil
}

func LanZouRealURL(download string) (string, error) {
	raw, header, err := util.Request("GET", download)
	if err != nil {
		return "", fmt.Errorf("get download url failed: %w", err)
	}

	rNote1 := regexp.MustCompile(`(?s:<!--.*?-->)`)
	rNote2 := regexp.MustCompile(`(//.*)`)
	raw = rNote1.ReplaceAll(raw, []byte(""))
	raw = rNote2.ReplaceAll(raw, []byte(""))

	// 正常流量
	if !strings.Contains(string(raw), "网络异常") {
		uRL := header.Get("Location")
		log.Printf("url: %v", uRL)
		return uRL, nil
	}

	// 异常流量
	// ajax
	r := regexp.MustCompile(`(?s:ajax\((\{.*?\})\,)`)
	values := r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return "", fmt.Errorf("javascript ajax regex failed")
	}

	// data
	r = regexp.MustCompile(`(?s:data\s*\:\s*(\{.*?\}))`)
	values = r.FindAllStringSubmatch(values[0][1], 1)
	if len(values) == 0 || len(values[0]) < 1 {
		return "", fmt.Errorf("ajax post data regex failed")
	}

	// key, value
	values[0][1] = strings.ReplaceAll(values[0][1], `'`, `"`)
	log.Printf("ajax regex data: %v", values[0][1])
	r = regexp.MustCompile(`(".*?"|\w)\s*\:\s*(.*?)(\s*,|\s*\})`)
	values = r.FindAllStringSubmatch(values[0][1], -1)
	if len(values) == 0 {
		return "", fmt.Errorf("post data key:value regex failed")
	}

	rValueIsNumOrStr := regexp.MustCompile(`^(".*"|\d+)$`)
	originKV := make(map[string]json.RawMessage)
	for _, v := range values {
		key, value := v[1], v[2]
		if strings.Contains(key, `"`) {
			key, _ = strconv.Unquote(key)
		}

		if key == "el" {
			originKV[key] = []byte("2")
			continue
		}

		if rValueIsNumOrStr.MatchString(value) {
			originKV[key] = []byte(value)
			continue
		}
	}

	raw, err = json.Marshal(originKV)
	if err != nil {
		return "", fmt.Errorf("marshal origin data: %w", err)
	}
	var kv map[string]interface{}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	err = d.Decode(&kv)
	if err != nil {
		return "", fmt.Errorf("convert origin data to form data failed: %w", err)
	}

	form := make(url.Values)
	for k, v := range kv {
		form.Set(k, fmt.Sprintf("%v", v))
	}

	time.Sleep(3 * time.Second)

	u := "https://developer.lanzoug.com/file/ajax.php"
	body := form.Encode()
	host := func(cur string) (val string) {
		name := cur[:strings.Index(cur, ".")]
		for i, v := range hosts {
			if strings.HasSuffix(cur, v) && i < len(hosts)-1 {
				return name + "." + hosts[i+1]
			}
		}

		return name + "." + hosts[0]
	}

	log.Printf("request ajax url:%v body: %v", u, body)
	raw, err = util.POST(u, util.WithBody(body), util.WithRetry(4),
		util.WithHeader(map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Content-Length": fmt.Sprintf("%v", len(body)),
		}), util.WithRandomHost(host))
	if err != nil {
		return "", fmt.Errorf("request ajax failed: %w", err)
	}

	var response struct {
		Code int    `json:"zt"`
		URL  string `json:"url"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return "", fmt.Errorf("request ajax failed: %w", err)
	}

	if response.Code != 1 {
		return "", fmt.Errorf("request ajax error: %v", response.URL)
	}

	return response.URL, nil
}
