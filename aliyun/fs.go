package aliyun

import (
	"fmt"
	"log"
	"net/url"
	"regexp"

	"github.com/tiechui1994/tool/util"
)

func AutoLogin() {
	u, _ := url.Parse(www + "/")
	session := util.GetCookie(u, "TEAMBITION_SESSIONID")
	if session != nil {
		return
	}

	clientid, token, pubkey, err := LoginParams()
	if err != nil {
		return
	}

	remail := regexp.MustCompile(`^[A-Za-z0-9]+([_\\.][A-Za-z0-9]+)*@([A-Za-z0-9\-]+\.)+[A-Za-z]{2,6}$`)
	rphone := regexp.MustCompile(`^1[3-9]\\d{9}$`)
retry:
	var username string
	var password string
	fmt.Printf("Input Email/Phone:")
	fmt.Scanf("%s", &username)
	fmt.Printf("Input Password:")
	fmt.Scanf("%s", &password)

	if username == "" || password == "" {
		goto retry
	}

	if remail.MatchString(username) {
		_, err = Login(clientid, pubkey, token, username, "", password)
	} else if rphone.MatchString(username) {
		_, err = Login(clientid, pubkey, token, "", username, password)
	} else {
		goto retry
	}

	if err != nil {
		fmt.Println(err.Error())
		goto retry
	}

	fmt.Println("登录成功!!!")
}

func Select() {

}

func main() {
	AutoLogin()

	_, _, _, err := GetCacheData()
	if err != nil {
		log.Println(err)
		return
	}

	p, err := NewProject("data", "5f6707e0f0aab521364694ee")
	if err != nil {
		log.Println(err)
		return
	}

	_ = p
}
