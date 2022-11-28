package cloudflare

import (
	"encoding/json"
	"fmt"
	"github.com/tiechui1994/tool/log"
	"strconv"
	"strings"

	"github.com/tiechui1994/tool/util"
)

type Cloudflare struct {
	ZoneID    string
	UserID    string
	AuthKey   string
	AuthEmail string
}

const apiprefix = "https://api.cloudflare.com/client/v4"

type DnsRecord struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Proxiable bool   `json:"proxiable"`
	Proxied   bool   `json:"proxied"`
	TTL       int    `json:"ttl"`
	Locked    bool   `json:"locked"`
	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`
}

func (c *Cloudflare) DnsList(page int, _type string) (records []DnsRecord, err error) {
	if page <= 0 {
		page = 1
	}
	if _type == "" {
		_type = "A"
	}

	values := []string{
		"type=" + _type,
		"page=" + strconv.Itoa(page),
		"per_page=100",
	}

	u := apiprefix + "/zones/" + c.ZoneID + "/dns_records"
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	raw, err := util.GET(u+"?"+strings.Join(values, "&"), util.WithHeader(header))
	if err != nil {
		return records, err
	}

	var response struct {
		Success bool            `json:"success"`
		Error   json.RawMessage `json:"error"`
		Result  []DnsRecord     `json:"result"`
	}

	log.Infoln("%v", string(raw))
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return records, err
	}

	if !response.Success {
		return records, fmt.Errorf("%v", response.Error)
	}

	return response.Result, nil
}

func (c *Cloudflare) DnsUpdate(record DnsRecord) error {
	u := apiprefix + "/zones/" + c.ZoneID + "/dns_records/" + record.ID
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	body := map[string]interface{}{
		"type":    record.Type,
		"name":    record.Name,
		"content": record.Content,
		"ttl":     record.TTL,
		"proxied": record.Proxied,
	}

	raw, err := util.PUT(u, util.WithBody(body), util.WithHeader(header))
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

func (c *Cloudflare) DnsDelete(record DnsRecord) error {
	u := apiprefix + "/zones/" + c.ZoneID + "/dns_records/" + record.ID
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	_, err := util.DELETE(u, util.WithHeader(header))
	if err != nil {
		return err
	}

	return nil
}

func (c *Cloudflare) DnsCreate(_type, name, content string, ttl int, proxied bool) error {
	u := apiprefix + "/zones/" + c.ZoneID + "/dns_records"
	header := map[string]string{
		"X-Auth-Key":   c.AuthKey,
		"X-Auth-Email": c.AuthEmail,
		"Content-Type": "application/json",
	}

	body := map[string]interface{}{
		"type":    _type,
		"name":    name,
		"content": content,
		"ttl":     ttl,
		"proxied": proxied,
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
