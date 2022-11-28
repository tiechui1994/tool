package gitee

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"time"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

/*
gitee 同步 git 项目
*/

const (
	endpoint = "https://gitee.com"
)

var (
	sleep     time.Duration
	cookie    string
	token     string
	grouppath string
)

func InitParams(c string, s time.Duration) {
	cookie = c
	sleep = s
}

// CSRF-Token
func CsrfToken() (err error) {
	u := endpoint
	data, _ := util.GET(u, util.WithHeader(map[string]string{"Cookie": cookie}))
	re := regexp.MustCompile(`meta name="csrf-token" content="(.*?)"`)
	tokens := re.FindStringSubmatch(string(data))
	if len(tokens) == 2 {
		token = tokens[1]
		return nil
	}

	return errors.New("invalid cookie")
}

// groupath
func Resources() (err error) {
	u := endpoint + "/api/v3/internal/my_resources"

	data, err := util.GET(u, util.WithHeader(map[string]string{"X-CSRF-Token": token}))
	if err != nil {
		log.Errorln("my resources:%v", err)
		return err
	}

	var result struct {
		GroupPath string `json:"groups_path"`
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return err
	}

	grouppath = result.GroupPath
	if grouppath == "" {
		return errors.New("invalid groupath")
	}

	return nil
}

// sync
func ForceSync(project string) (err error) {
	values := make(url.Values)
	values.Set("user_sync_code", "")
	values.Set("password_sync_code", "")
	values.Set("sync_wiki", "true")
	values.Set("authenticity_token", token)

	u := endpoint + "/" + grouppath + "/" + project + "/force_sync_project"
	data, err := util.POST(u,
		util.WithBody(bytes.NewBufferString(values.Encode())),
		util.WithHeader(map[string]string{"X-CSRF-Token": token}))
	if len(data) == 0 {
		log.Infoln("sync [%v] .... ", project)
		time.Sleep(sleep * time.Second)
		return ForceSync(project)
	}

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return err
	}

	if result.Status == 1 {
		log.Infoln("[%v] 同步成功", project)
	} else {
		log.Errorln("[%v] 同步失败", project)
	}

	return nil
}

func Mark() (err error) {
	body := `{"scope":"infos"}`
	u := endpoint + "/notifications/mark"
	header := map[string]string{
		"X-CSRF-Token": token,
		"Content-Type": "application/json;charset=UTF-8",
	}
	data, err := util.PUT(u,
		util.WithBody(bytes.NewBufferString(body)),
		util.WithHeader(header))
	if err != nil {
		return err
	}

	var result struct {
		Count int `json:"count"`
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return err
	}

	log.Infoln("成功将 [%d] 条消息标记已读", result.Count)
	return nil
}
