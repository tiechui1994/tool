package weixin

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/tiechui1994/tool/util"
)

const weixin = "https://api.weixin.qq.com"

const (
	MediaImage = "image"
	MediaVideo = "video"
	MediaVoice = "voice"
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
		NewItem    []Article `json:"new_item"`
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

	ThumbMediaUrl    string `json:"thumb_media_url"`
	ShowCoverPic     int    `json:"show_cover_pic"`
	ContentSourceUrl string `json:"content_source_url"`

	Url      string `json:"url"`
	ThumbUrl string `json:"thumb_url"`
}

func MaterialList(token string, mtype string) (list interface{}, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/material/batchget_material?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"type":   mtype,
		"offset": 0,
		"count":  2,
	}

	header := map[string]string{
		"content-type": "application/json",
	}
	raw, err := util.POST(u, &body, header)
	if err != nil {
		return nil, err
	}

	if mtype == MediaNews {
		var result struct {
			TotalCount int    `json:"total_count"`
			ItemCount  int    `json:"item_count"`
			Item       []News `json:"item"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, err
		}
		return result.Item, nil

	} else {
		var result struct {
			TotalCount int     `json:"total_count"`
			ItemCount  int     `json:"item_count"`
			Item       []Media `json:"item"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, err
		}
		return result.Item, nil
	}
}

func AddPersitNews(token string, article Article) (id string, err error) {
	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/material/add_news?" + strings.Join(value, "&")
	body := map[string]interface{}{
		"articles": []Article{article},
	}

	raw, err := util.POST(u, &body, nil)
	if err != nil {
		return id, err
	}

	var result struct {
		MediaID string `json:"media_id"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return id, err
	}

	return result.MediaID, nil
}

func UploadNewsImage(token string, filename string) (url string, err error) {
	info, err := os.Stat(filename)
	if err != nil {
		return url, err
	}

	value := []string{
		"access_token=" + token,
	}
	u := weixin + "/cgi-bin/media/uploadimg?" + strings.Join(value, "&")
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	writer, err := w.CreateFormFile("media", info.Name())
	if err != nil {
		return url, err
	}
	reader, err := os.Open(filename)
	if err != nil {
		return url, err
	}
	_, err = io.Copy(writer, reader)
	if err != nil {
		return url, err
	}

	w.Close()
	header := map[string]string{
		"content-type": w.FormDataContentType(),
	}
	raw, err := util.POST(u, &body, header)
	if err != nil {
		return url, err
	}

	var result struct {
		URL string `json:"url"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return url, err
	}

	return result.URL, nil
}

func AddPersitMaterial(token string, mtype, filename string, args ...map[string]string) (url string, err error) {
	info, err := os.Stat(filename)
	if err != nil {
		return url, err
	}

	value := []string{
		"access_token=" + token,
		"type=" + mtype,
	}
	u := weixin + "/cgi-bin/material/add_material?" + strings.Join(value, "&")
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	writer, err := w.CreateFormFile("media", info.Name())
	if err != nil {
		return url, err
	}
	reader, err := os.Open(filename)
	if err != nil {
		return url, err
	}
	_, err = io.Copy(writer, reader)
	if err != nil {
		return url, err
	}

	if mtype == MediaVideo {
		bin, _ := json.Marshal(args[0])
		w.WriteField("description", string(bin))
	}

	w.Close()
	header := map[string]string{
		"content-type": w.FormDataContentType(),
	}
	raw, err := util.POST(u, &body, header)
	if err != nil {
		return url, err
	}

	var result struct {
		MediaID string `json:"media_id"`
		URL     string `json:"url"`
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return url, err
	}

	return result.URL, nil
}
