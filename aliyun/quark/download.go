package quark

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tiechui1994/tool/util"
)

type DriverQuark struct {
	api     string
	referer string
	ua      string
	pr      string
	client  *util.EmbedClient
}

func NewQuark(cookie string) (*DriverQuark, error) {
	client := util.NewClient(util.WithClientInitCookieJar("Quark", "https://drive.quark.cn", cookie))
	q := &DriverQuark{
		client:  client,
		pr:      "ucpro",
		api:     "https://drive.quark.cn/1/clouddrive",
		referer: "https://pan.quark.cn",
		ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) quark-cloud-drive/2.5.20 Chrome/100.0.4896.160 Electron/18.3.5.4-b478491100 Safari/537.36 Channel/pckk_other_ch",
	}

	return q, q.init()
}

func (d *DriverQuark) init() error {
	query := url.Values{}
	query.Set("pr", d.pr)
	query.Set("fr", "pc")
	_, err := d.client.GET(d.api+"/config?"+query.Encode(), util.WithHeader(map[string]string{
		"Accept":  "application/json, text/plain, */*",
		"Referer": d.referer,
	}))

	return err
}

type File struct {
	Fid        string `json:"fid"`
	PdirFid    string `json:"pdir_fid"`
	FileName   string `json:"file_name"`
	Size       int64  `json:"size"`
	LUpdatedAt int64  `json:"l_updated_at"`
	File       bool   `json:"file"`
	UpdatedAt  int64  `json:"updated_at"`
}

type Download struct {
	FileName    string
	DownloadUrl string
	Size        int
	Header      http.Header
}

func (d *DriverQuark) List(pid string) ([]File, error) {
	files := make([]File, 0)
	page := 1
	size := 100

	query := url.Values{}
	query.Set("pr", d.pr)
	query.Set("fr", "pc")
	query.Set("pdir_fid", pid)
	query.Set("_size", strconv.Itoa(size))
	query.Set("_fetch_total", "1")
	for {
		query.Set("_page", strconv.Itoa(page))
		var result struct {
			Status  int    `json:"status"`
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				List []File `json:"list"`
			} `json:"data"`
			Metadata struct {
				Size  int    `json:"_size"`
				Page  int    `json:"_page"`
				Count int    `json:"_count"`
				Total int    `json:"_total"`
				Way   string `json:"way"`
			} `json:"metadata"`
		}

		raw, err := d.client.GET(d.api+"/file/sort?"+query.Encode(), util.WithHeader(map[string]string{
			"Accept":  "application/json, text/plain, */*",
			"Referer": d.referer,
		}))
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			return nil, err
		}

		files = append(files, result.Data.List...)
		if page*size >= result.Metadata.Total {
			break
		}
		page++
	}

	return files, nil
}

func (d *DriverQuark) Download(fid string) ([]Download, error) {
	var result struct {
		Status  int    `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Fid         string `json:"fid"`
			FileName    string `json:"file_name"`
			DownloadUrl string `json:"download_url"`
			Size        int    `json:"size"`
		} `json:"data"`
	}
	query := url.Values{}
	query.Set("pr", d.pr)
	query.Set("fr", "pc")
	raw, err := d.client.POST(d.api+"/file/download?"+query.Encode(), util.WithHeader(map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Referer":    d.referer,
		"User-Agent": d.ua,
	}), util.WithBody(map[string]interface{}{
		"fids": []string{fid},
	}))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &result)
	if err != nil {
		return nil, err
	}

	var cookies []string
	u, _ := url.Parse(d.api)
	for _, c := range d.client.GetCookies(u) {
		cookies = append(cookies, c.String())
	}

	var down []Download
	for _, v := range result.Data {
		down = append(down, Download{
			FileName:    v.FileName,
			DownloadUrl: v.DownloadUrl,
			Size:        v.Size,
			Header: map[string][]string{
				"Cookie":     []string{strings.Join(cookies, "; ")},
				"Referer":    []string{d.referer},
				"User-Agent": []string{d.ua},
			},
		})
	}

	return down, nil
}
