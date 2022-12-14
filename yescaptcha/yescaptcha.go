package yescaptcha

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

const (
	host   = "https://api.yescaptcha.com"
	cnhost = "https://china.yescaptcha.com"
)

type Task string

const (
	ImageToTextTask                    Task = "ImageToTextTask"
	NoCaptchaTaskProxyless             Task = "NoCaptchaTaskProxyless"
	ReCaptchaV2Classification          Task = "ReCaptchaV2Classification"
	RecaptchaV2EnterpriseTaskProxyless Task = "RecaptchaV2EnterpriseTaskProxyless"
	RecaptchaV3TaskProxyless           Task = "RecaptchaV3TaskProxyless"
	RecaptchaV3EnterpriseTask          Task = "RecaptchaV3EnterpriseTask"
	HCaptchaTaskProxyless              Task = "HCaptchaTaskProxyless"
	HCaptchaClassification             Task = "HCaptchaClassification"
	FunCaptchaClassification           Task = "FunCaptchaClassification"
	FuncaptchaTaskProxyless            Task = "FuncaptchaTaskProxyless"
)

type Captcha struct {
	host      string
	clientKey string
	header    map[string]string
}

func NewCaptcha(clientKey, region string) *Captcha {
	c := Captcha{
		host:      host,
		clientKey: clientKey,
		header: map[string]string{
			"content-type": "application/json",
		},
	}

	if strings.Contains(strings.ToLower(region), "china") ||
		strings.Contains(strings.ToLower(region), "cn") {
		c.host = cnhost
	}

	return &c
}

func (c *Captcha) GetBalance() (balance int, err error) {
	u := c.host + "/getBalance"
	raw, err := util.POST(u, util.WithHeader(c.header), util.WithBody(map[string]string{
		"clientKey": c.clientKey,
	}), util.WithRetry(2))
	if err != nil {
		return balance, err
	}

	var result struct {
		Balance int    `json:"balance"`
		Code    int    `json:"errorId"`
		Msg     string `json:"errorDescription"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return balance, err
	}

	if result.Code != 0 {
		return balance, fmt.Errorf("GetBalance Failed: %q", result.Msg)
	}

	return result.Balance, nil
}

func (c *Captcha) CreateTask(task Task, body string) (solution interface{}, err error) {
	u := c.host + "/createTask"
	raw, err := util.POST(u, util.WithHeader(c.header), util.WithBody(map[string]interface{}{
		"clientKey": c.clientKey,
		"task": map[string]interface{}{
			"type": task,
			"body": body,
		},
	}))
	if err != nil {
		return solution, err
	}

	var result struct {
		TaskId string `json:"taskId"`
		Code   int    `json:"errorId"`
		Msg    string `json:"errorDescription"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return solution, err
	}

	if result.Code != 0 {
		return solution, fmt.Errorf("CreateTask Failed: %v", result.Msg)
	}

	taskResultQuery := func(taskId string) (solution interface{}, status string, err error) {
		u = c.host + "/getTaskResult"
		raw, err = util.POST(u, util.WithHeader(c.header), util.WithBody(map[string]interface{}{
			"clientKey": c.clientKey,
			"taskId":    taskId,
		}), util.WithRetry(2))
		if err != nil {
			return solution, status, err
		}

		var result struct {
			Status   string                 `json:"status"`
			Solution map[string]interface{} `json:"solution"`
			Code     int                    `json:"errorId"`
			Msg      string                 `json:"errorDescription"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return solution, status, err
		}
		if result.Code != 0 {
			return solution, status, fmt.Errorf("GetTaskResult Failed: %v", result.Msg)
		}

		return result.Solution, result.Status, nil
	}

	ctx, _ := context.WithTimeout(context.Background(), 600*time.Second)
	ticker := time.NewTicker(5 * time.Second)
	try := 0
	for {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			var status string
			solution, status, err = taskResultQuery(result.TaskId)
			if err != nil && try < 3 {
				try += 1
				log.Errorln("QueryTask Failed: %v", err)
				continue
			}
			if err != nil {
				return solution, err
			}
			if status == "ready" {
				return solution, nil
			}
		}
	}
}
