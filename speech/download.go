package speech

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	"github.com/tiechui1994/tool/util"
)

const (
	fileSize = 9 * 1024 * 1024
)

type Context struct {
	Client map[string]interface{} `json:"client"`
}

type param struct {
	Context Context
	Header  map[string]string
	APIkey  string
}

var (
	params = map[string]param{
		"ANDROID_MUSIC": {
			Context: Context{
				Client: map[string]interface{}{
					"clientName":        "ANDROID_MUSIC",
					"clientVersion":     "5.16.51",
					"androidSdkVersion": 30,
				},
			},
			Header: map[string]string{
				"User-Agent": "com.google.android.apps.youtube.music/",
			},
			APIkey: "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		},
		"ANDROID_EMBED": {
			Context: Context{
				Client: map[string]interface{}{
					"clientName":        "ANDROID_EMBEDDED_PLAYER",
					"clientVersion":     "17.31.35",
					"clientScreen":      "EMBED",
					"androidSdkVersion": 30,
				},
			},
			Header: map[string]string{
				"User-Agent": "com.google.android.youtube/",
			},
			APIkey: "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		},
	}
)

type AudioFormat struct {
	ITag            int    `json:"itag"`
	Url             string `json:"url"`
	Fps             int    `json:"fps"`
	MimeType        string `json:"mimeType"`
	BitRate         int    `json:"bitrate"`
	AudioSampleRate string `json:"audioSampleRate"`
	ContentLength   string `json:"contentLength"`
}

type VideoFormat struct {
	ITag          int    `json:"itag"`
	Url           string `json:"url"`
	Fps           int    `json:"fps"`
	MimeType      string `json:"mimeType"`
	BitRate       int    `json:"bitrate"`
	Quality       string `json:"quality"`
	ContentLength string `json:"contentLength"`
}

func fetchVideoInfo(videoID string) (audios []AudioFormat, videos []VideoFormat, err error) {
	param := params["ANDROID_EMBED"]
	query := url.Values{}
	query.Set("key", param.APIkey)
	query.Set("contentCheckOk", "true")
	query.Set("racyCheckOk", "true")
	query.Set("videoId", videoID)

	u := "https://www.youtube.com/youtubei/v1/player?" + query.Encode()
	headers := map[string]string{
		"Content-Type":    "application/json",
		"User-Agent":      "com.google.android.apps.youtube.music/",
		"accept-language": "en-US,en",
	}
	body := map[string]interface{}{
		"context": param.Context,
	}

	raw, err := util.POST(u, util.WithHeader(headers), util.WithBody(body))
	if err != nil {
		fmt.Println(err)
		return nil, nil, fmt.Errorf("get player info error: %w", err)
	}

	var response struct {
		StreamingData struct {
			ExpiresInSeconds string        `json:"expiresInSeconds"`
			Formats          []AudioFormat `json:"formats"`
			AdaptiveFormats  []VideoFormat `json:"adaptiveFormats"`
		} `json:"streamingData"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, nil, fmt.Errorf("decode player failed: %w", err)
	}

	if response.StreamingData.ExpiresInSeconds == "" {
		return nil, nil, fmt.Errorf("palyer invalid")
	}
	return response.StreamingData.Formats, response.StreamingData.AdaptiveFormats, nil
}

func FetchYouTubeAudio(videoID, dst string) error {
	audios, _, err := fetchVideoInfo(videoID)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}

	audio := audios[0]
	size, _ := strconv.ParseInt(audio.ContentLength, 10, 64)
	download := int64(0)
	log.Printf("start download audio file: %v", audio.Url)
	for download < size {
		stopPos := download + fileSize
		if stopPos > size {
			stopPos = size
		}
		u := fmt.Sprintf("%v&range=%v-%v", audio.Url, download, stopPos)
		raw, err := util.GET(u, util.WithRetry(3))
		if err != nil {
			return fmt.Errorf("GET failed: %w", err)
		}
		n, err := fd.WriteAt(raw, download)
		if n != len(raw) || err != nil {
			return fmt.Errorf("WriteAt failed: %w", err)
		}

		download = stopPos
		log.Printf("current: %v, size: %v", download, size)
	}

	return nil
}

func FetchYouTubeVideo(videoID, dst string) error {
	_, videos, err := fetchVideoInfo(videoID)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}

	video := videos[0]
	size, _ := strconv.ParseInt(video.ContentLength, 10, 64)
	download := int64(0)
	log.Printf("start download video file: %v", video.Url)
	for download < size {
		stopPos := download + fileSize
		if stopPos > size {
			stopPos = size
		}
		u := fmt.Sprintf("%v&range=%v-%v", video.Url, download, stopPos)
		raw, err := util.GET(u, util.WithRetry(3))
		if err != nil {
			return fmt.Errorf("GET failed: %w", err)
		}
		n, err := fd.WriteAt(raw, download)
		if n != len(raw) || err != nil {
			return fmt.Errorf("WriteAt failed: %w", err)
		}

		download = stopPos
		log.Printf("current: %v, size: %v", download, size)
	}

	return nil
}
