package speech

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/tiechui1994/tool/util"
)

type Format string

const (
	MP3Format  Format = "mp3"
	M4AFormat  Format = "m4a"
	WEBMFormat Format = "webm"
	AACFormat  Format = "aac"
	OGGFormat  Format = "ogg"

	MP4480PFormat  Format = "480"
	MP4720PFormat  Format = "720"
	MP41080PFormat Format = "1080"
	MP41440PFormat Format = "1440"
	WEBM4KFormat   Format = "4k"
	WEBM8KFormat   Format = "8k"
)

func YoutubeDownload(youtube, path string, format Format) error {
	rURL := regexp.MustCompile(`https://www.youtube.com/watch\?v=.*`)
	if !rURL.Match([]byte(youtube)) {
		return fmt.Errorf("invalid url %q", youtube)
	}

	val := url.Values{}
	val.Set("format", string(format))
	val.Set("url", youtube)

	u := "https://loader.to/ajax/download.php?" + val.Encode()
	header := map[string]string{
		"origin":  "https://y2down.cc",
		"referer": "https://y2down.cc/",
	}

	u = "https://cloud.unicast.workers.dev/" + u
	raw, err := util.GET(u, util.WithHeader(header), util.WithRetry(1))
	if err != nil {
		return fmt.Errorf("download %w", err)
	}

	var downloadInfo struct {
		Success bool   `json:"success"`
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	err = json.Unmarshal(raw, &downloadInfo)
	if err != nil {
		return err
	}

	fmt.Printf("download id: %v \n", downloadInfo.ID)

	if !downloadInfo.Success {
		return fmt.Errorf("download failed")
	}

	u, err = progress(downloadInfo.ID)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	reader, err := util.File(u, "GET", util.WithHeader(header), util.WithRetry(3))
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(fd, reader, make([]byte, 8192))
	return err
}

func progress(id string) (string, error) {
	u := "https://loader.to/ajax/progress.php?id=" + id
	header := map[string]string{
		"referer": "https://y2down.cc/",
	}
	u = "https://cloud.unicast.workers.dev/" + u
	raw, err := util.GET(u, util.WithHeader(header), util.WithRetry(1))
	if err != nil {
		return "", fmt.Errorf("download %w", err)
	}

	var progressInfo struct {
		Progress int    `json:"progress"`
		URL      string `json:"download_url"`
		Text     string `json:"text"`
	}
	err = json.Unmarshal(raw, &progressInfo)
	if err != nil {
		return "", err
	}

	fmt.Printf("current process %d\n", progressInfo.Progress)

	if progressInfo.Progress == 1000 {
		return progressInfo.URL, nil
	}

	time.Sleep(time.Second)

	return progress(id)
}
