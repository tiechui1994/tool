package idea

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

// http://idea.94goo.com/
// http://idea.do198.com/
// http://vrg123.com/loadcode/ 4565

// go build -o idea -ldflags '-w -s' idea.go

func WriteCode(code, file string) {
	if fd, err := os.Open(file); err == nil {
		data, err := ioutil.ReadAll(fd)
		if err != nil {
			log.Errorln(err.Error())
			return
		}

		// ignore: Account and License Server
		urlpre := []byte{0xff, 0xff}
		for _, b := range []byte("URL") {
			urlpre = append(urlpre, []byte{b, 0x00}...)
		}

		accpre := []byte{0xff, 0xff}
		for _, b := range []byte("JetProfile") {
			accpre = append(accpre, []byte{b, 0x00}...)
		}

		if strings.HasPrefix(string(data), string(urlpre)) ||
			strings.HasPrefix(string(data), string(accpre)) {
			return
		}
	}

	log.Infoln("Start write code to file: %v ...", file)
	fd, err := os.Create(file)
	if err != nil {
		log.Errorln("%v", err)
		return
	}
	defer fd.Close()

	code = "<certificate-key>\n" + code
	data := []byte{0xff, 0xff}
	for _, b := range []byte(code) {
		data = append(data, []byte{b, 0x00}...)
	}
	fd.Write(data)
	log.Infoln("Success write %v", file)
}

func GetCode1() string {
	u := "http://idea.94goo.com"
	data, err := util.GET(u, nil)
	if err != nil {
		log.Infoln("idea.94goo.com: %v", err)
		return ""
	}

	re := regexp.MustCompile(`<input type="hidden" class="new-key" value="(.*)">`)
	tokens := re.FindAllStringSubmatch(string(data), 1)
	if len(tokens) == 1 {
		return tokens[0][1]
	}

	return ""
}

func GetCode2() (token, username, password string) {
	data, err := util.GET("http://vrg123.com/", nil)
	if err != nil {
		log.Infoln("%v", err)
		return token, username, password
	}

	str := strings.Replace(string(data), "\n", "", -1)

	re := regexp.MustCompile(`<textarea.*?>(.*)</textarea>`)
	tokens := re.FindAllStringSubmatch(str, 1)
	if len(tokens) == 1 && len(tokens[0]) == 2 {
		token = tokens[0][1]
	}

	reusername := regexp.MustCompile(`<input\s*class="username"\s*value="(.*?)".*?>`)
	repassword := regexp.MustCompile(`<input\s*class="password"\s*value="(.*?)".*?>`)
	utokens := reusername.FindAllStringSubmatch(str, 1)
	ptokens := repassword.FindAllStringSubmatch(str, 1)
	if len(utokens) == 1 && len(utokens[0]) == 2 {
		username = utokens[0][1]
	}
	if len(ptokens) == 1 && len(ptokens[0]) == 2 {
		password = ptokens[0][1]
	}

	return token, username, password
}

func ValidCode(code string) bool {
	tokens := strings.Split(code, "-")
	// licenseid-metadata-sign-cert
	if len(tokens) != 4 {
		return false
	}

	var meta struct {
		LicenseId string `json:"licenseId"`
		Products  []struct {
			Code     string `json:"code"`
			PaidUpTo string `json:"paidUpTo"`
			Extended bool   `json:"extended"`
		} `json:"products"`
	}

	data, err := base64.StdEncoding.DecodeString(tokens[1])
	if err != nil {
		return false
	}

	err = json.Unmarshal(data, &meta)
	if err != nil {
		return false
	}

	log.Debugln(meta.Products[0].PaidUpTo)
	log.Debugln(code)

	return meta.LicenseId == tokens[0] && len(meta.Products) > 0 &&
		meta.Products[0].PaidUpTo >= time.Now().Format("2006-01-02")
}

func SearchFile(dir string) []string {
	var paths []string
	re := regexp.MustCompile(`\.(goland|clion|pycharm|intellijidea)[0-9]{4}\.[0-9]`)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := strings.ToLower(info.Name())
			if re.MatchString(name) {
				key := re.FindAllStringSubmatch(name, 1)[0][1]
				switch key {
				case "goland", "clion", "pycharm":
					paths = append(paths, path+"/config/"+key+".key")
				case "intellijidea":
					paths = append(paths, path+"/config/idea.key")
				}
			}
		}

		return nil
	})

	return paths
}
