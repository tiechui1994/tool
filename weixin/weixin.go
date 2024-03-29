package weixin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tiechui1994/tool/util"
)

const weixin = "https://api.weixin.qq.com"

const (
	MediaImage = "image"
	MediaVideo = "video"
	MediaVoice = "voice"
	MediaThumb = "thumb"
	MediaNews  = "news"
)

type Media struct {
	MediaID    string `json:"media_id"`
	Name       string `json:"name"`
	UpdateTime int    `json:"update_time"`
	Url        string `json:"url"`
}

type News struct {
	MediaID string `json:"media_id"`
	Content struct {
		NewItem    []Article `json:"news_item"`
		CreateTime int       `json:"create_time"`
		UpdateTime int       `json:"update_time"`
	}
	UpdateTime int `json:"update_time"`
}

type Article struct {
	Title   string `json:"title"`
	Author  string `json:"author"`
	Content string `json:"content"`
	Digest  string `json:"digest"`

	ContentSourceUrl   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	NeedOpenComment    int    `json:"need_open_comment"`
	OnlyFansCanComment int    `json:"only_fans_can_comment"`

	Url string `json:"url"`
}

type wxerror struct {
	Code int    `json:"errcode"`
	Msg  string `json:"errmsg"`
}

func (e wxerror) Error() string {
	return fmt.Sprintf("code:[%v], msg:%v", e.Code, e.Msg)
}

func (e wxerror) isError() bool {
	return e.Code != 0
}

func PersistMaterialList(token, mtype string, offset, count int) (list interface{}, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/material/batchget_material?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"type":   mtype,
		"offset": offset,
		"count":  count,
	}
	header := map[string]string{
		"content-type": "application/json",
	}
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return nil, err
	}

	if mtype == MediaNews {
		var result struct {
			wxerror
			TotalCount int    `json:"total_count"`
			ItemCount  int    `json:"item_count"`
			Item       []News `json:"item"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, err
		}

		if result.isError() {
			err = result.wxerror
			return
		}

		return result.Item, nil
	} else {
		var result struct {
			wxerror
			TotalCount int     `json:"total_count"`
			ItemCount  int     `json:"item_count"`
			Item       []Media `json:"item"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, err
		}

		if result.isError() {
			err = result.wxerror
			return
		}

		return result.Item, nil
	}
}

func UploadImage(token, filename string) (url string, err error) {
	fd, err := os.Open(filename)
	if err != nil {
		return url, err
	}

	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/media/uploadimg?" + strings.Join(value, "&")

	reader, contentType, contentLen, err := uploadFile(map[string]interface{}{
		"media": fd,
	})
	if err != nil {
		return url, err
	}

	header := map[string]string{
		"Content-Type":   contentType,
		"Content-Length": fmt.Sprintf("%v", contentLen),
	}
	raw, err := util.POST(u, util.WithBody(reader), util.WithHeader(header))
	if err != nil {
		return url, err
	}

	var result struct {
		wxerror
		URL string `json:"url"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return url, err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.URL, nil
}

func AddPersistMaterial(token, mtype, filename string, args ...map[string]string) (url, id string, err error) {
	fd, err := os.Open(filename)
	if err != nil {
		return url, id, err
	}

	value := []string{
		"access_token=" + token,
		"type=" + mtype,
	}
	u := weixin + "/cgi-bin/material/add_material?" + strings.Join(value, "&")

	fields := map[string]interface{}{
		"media": fd,
	}
	if mtype == MediaVideo {
		bin, _ := json.Marshal(args[0])
		fields["description"] = string(bin)
	}

	reader, contentType, contentLen, err := uploadFile(fields)
	if err != nil {
		return url, id, err
	}

	header := map[string]string{
		"Content-Type":   contentType,
		"Content-Length": fmt.Sprintf("%v", contentLen),
	}
	raw, err := util.POST(u, util.WithBody(reader), util.WithHeader(header))
	if err != nil {
		return url, id, err
	}

	var result struct {
		wxerror
		MediaID string `json:"media_id"`
		URL     string `json:"url"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return url, id, err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.URL, result.MediaID, nil
}

type TokenInfo struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func Token(appid, secret string) (token TokenInfo, err error) {
	value := []string{
		"grant_type=client_credential",
		"appid=" + appid,
		"secret=" + secret,
	}
	u := weixin + "/cgi-bin/token?" + strings.Join(value, "&")
	raw, err := util.GET(u)
	if err != nil {
		return
	}

	var result struct {
		wxerror
		TokenInfo
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.TokenInfo, nil
}

func AddDraft(token string, article Article) (id string, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/draft/add?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"articles": []Article{article},
	}

	raw, err := util.POST(u, util.WithBody(body))
	if err != nil {
		return id, err
	}

	var result struct {
		wxerror
		MediaID string `json:"media_id"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return id, err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.MediaID, nil
}

func GetDraft(token, mediaid string) (artice []Article, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/draft/get?" + strings.Join(value, "&")
	body := map[string]string{
		"media_id": mediaid,
	}
	raw, err := util.POST(u, util.WithBody(body))
	if err != nil {
		return artice, err
	}

	var result struct {
		wxerror
		NewsItem []Article `json:"news_item"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return artice, err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.NewsItem, nil
}

func UpdateDraft(token, mediaid string, index int, article Article) (err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/draft/update?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"media_id": mediaid,
		"index":    index,
		"articles": article,
	}

	raw, err := util.POST(u, util.WithBody(body))
	if err != nil {
		return err
	}

	var result struct {
		wxerror
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return nil
}

func DraftList(token string, offset, count int) (list []News, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/draft/batchget?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"offset":     offset,
		"count":      count,
		"no_content": 1,
	}
	header := map[string]string{
		"content-type": "application/json",
	}
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return nil, err
	}
	var result struct {
		wxerror
		TotalCount int    `json:"total_count"`
		ItemCount  int    `json:"item_count"`
		Items []News `json:"item"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return result.Items, err
	}

	if result.isError() {
		err = result.wxerror
		return
	}

	return result.Items, err
}
