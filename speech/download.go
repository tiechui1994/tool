package speech

import (
	"fmt"
	"github.com/tiechui1994/tool/util"
	"net/url"
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

func FetchInfo(videoID string) {
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

	fmt.Println(u, body)
	raw, err := util.POST(u, util.WithHeader(headers), util.WithBody(body))
	if err != nil {
		return
	}

	fmt.Println(string(raw))
}
