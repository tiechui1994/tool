package cloudflare

import (
	"encoding/json"
	"fmt"
	"github.com/tiechui1994/tool/util"
	"strings"
)

type Target struct {
	Target     string `json:"target"`
	Constraint struct {
		Operator string `json:"operator"`
		Value    string `json:"value"`
	} `json:"constraint"`
}

type ActionRedirect struct {
	Url        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

type Action struct {
	ID    string      `json:"id" yaml:"-"`
	Value interface{} `json:"value"`
}

type PageRule struct {
	ID         string   `json:"id"`
	Targets    []Target `json:"targets"`
	Actions    []Action `json:"actions"`
	Status     string   `json:"status"`
	Priority   int      `json:"priority"`
	CreatedOn  string   `json:"created_on"`
	ModifiedOn string   `json:"modified_on"`
}

func (c *Cloudflare) PageRulesList() (records []PageRule, err error) {
	u := apiprefix + "/zones/" + c.ZoneID + "/pagerules"
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	raw, err := util.GET(u, util.WithHeader(header))
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return c.PageRulesList()
		}
		return records, err
	}

	var response struct {
		Success bool            `json:"success"`
		Error   json.RawMessage `json:"error"`
		Result  []PageRule      `json:"result"`
	}

	err = json.Unmarshal(raw, &response)
	if err != nil {
		return records, err
	}

	if !response.Success {
		return records, fmt.Errorf("%v", response.Error)
	}

	return response.Result, nil
}

func (c *Cloudflare) PageRulesUpdate(rule PageRule) (err error) {
	u := apiprefix + "/zones/" + c.ZoneID + "/pagerules/" + rule.ID
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	body := map[string]interface{}{
		"targets": rule.Targets,
		"actions": rule.Actions,
		"status":  rule.Status,
	}
	raw, err := util.PUT(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return c.PageRulesUpdate(rule)
		}
		return err
	}

	var response struct {
		Success bool            `json:"success"`
		Error   json.RawMessage `json:"error"`
	}

	err = json.Unmarshal(raw, &response)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("%v", response.Error)
	}

	return nil
}

func (c *Cloudflare) PageRulesDelete(rule PageRule) (err error) {
	u := apiprefix + "/zones/" + c.ZoneID + "/pagerules/" + rule.ID
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	_, err = util.DELETE(u, util.WithHeader(header))
	if err != nil {
		return err
	}

	return nil
}

func (c *Cloudflare) PageRulesCreate(rule PageRule) (err error) {
	u := apiprefix + "/zones/" + c.ZoneID + "/pagerules"
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}
	body := map[string]interface{}{
		"targets": rule.Targets,
		"actions": rule.Actions,
		"status":  "active",
	}
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return err
	}

	var response struct {
		Success bool            `json:"success"`
		Error   json.RawMessage `json:"error"`
	}

	err = json.Unmarshal(raw, &response)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("%v", response.Error)
	}

	return nil
}
