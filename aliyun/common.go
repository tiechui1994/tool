package aliyun

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/tiechui1994/tool/util"
)

func GetCacheData() (roles []Role, orgs []Org, err error) {
	var result struct {
		Roles []Role
		Org   []Org
	}

	key := filepath.Join(util.ConfDir, "teambition.json")
	if util.ReadFile(key, &result) == nil {
		return result.Roles, result.Org, nil
	}

	roles, err = Roles()
	if err != nil {
		log.Println(err)
		return
	}

	if len(roles) == 0 {
		err = errors.New("no roles")
		return
	}

	var (
		lock sync.Mutex
		wg   sync.WaitGroup
	)
	for _, role := range roles {
		wg.Add(1)
		go func(orgid string) {
			defer wg.Done()
			org, err := Orgs(roles[0].OrganizationId)
			if err != nil {
				log.Println("fetch org err:", orgid, err)
				return
			}
			org.Projects, err = Projects(orgid)
			if err != nil {
				log.Println("fetch project err:", orgid, err)
				return
			}

			lock.Lock()
			orgs = append(orgs, org)
			lock.Unlock()
		}(role.OrganizationId)
	}

	wg.Wait()

	result.Roles = roles
	result.Org = orgs
	util.WriteFile(key, result)
	return
}

func AutoLogin() {
	u, _ := url.Parse(www + "/")
	cookie := util.GetCookie(u, "TEAMBITION_SESSIONID")
	if cookie != nil {
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
	util.SyncCookieJar()
}
