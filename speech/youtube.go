package speech

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

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

	itags = map[int][2]string{
		// PROGRESSIVE VIDEO
		5:   {"240p", "64kbps"},
		6:   {"270p", "64kbps"},
		13:  {"144p", ""},
		17:  {"144p", "24kbps"},
		18:  {"360p", "96kbps"},
		22:  {"720p", "192kbps"},
		34:  {"360p", "128kbps"},
		35:  {"480p", "128kbps"},
		36:  {"240p", ""},
		37:  {"1080p", "192kbps"},
		38:  {"3072p", "192kbps"},
		43:  {"360p", "128kbps"},
		44:  {"480p", "128kbps"},
		45:  {"720p", "192kbps"},
		46:  {"1080p", "192kbps"},
		59:  {"480p", "128kbps"},
		78:  {"480p", "128kbps"},
		82:  {"360p", "128kbps"},
		83:  {"480p", "128kbps"},
		84:  {"720p", "192kbps"},
		85:  {"1080p", "192kbps"},
		91:  {"144p", "48kbps"},
		92:  {"240p", "48kbps"},
		93:  {"360p", "128kbps"},
		94:  {"480p", "128kbps"},
		95:  {"720p", "256kbps"},
		96:  {"1080p", "256kbps"},
		100: {"360p", "128kbps"},
		101: {"480p", "192kbps"},
		102: {"720p", "192kbps"},
		132: {"240p", "48kbps"},
		151: {"720p", "24kbps"},
		300: {"720p", "128kbps"},
		301: {"1080p", "128kbps"},

		// DASH VIDEO
		133: {"240p", ""},  // MP4
		134: {"360p", ""},  // MP4
		135: {"480p", ""},  // MP4
		136: {"720p", ""},  // MP4
		137: {"1080p", ""}, // MP4
		138: {"2160p", ""}, // MP4
		160: {"144p", ""},  // MP4
		167: {"360p", ""},  // WEBM
		168: {"480p", ""},  // WEBM
		169: {"720p", ""},  // WEBM
		170: {"1080p", ""}, // WEBM
		212: {"480p", ""},  // MP4
		218: {"480p", ""},  // WEBM
		219: {"480p", ""},  // WEBM
		242: {"240p", ""},  // WEBM
		243: {"360p", ""},  // WEBM
		244: {"480p", ""},  // WEBM
		245: {"480p", ""},  // WEBM
		246: {"480p", ""},  // WEBM
		247: {"720p", ""},  // WEBM
		248: {"1080p", ""}, // WEBM
		264: {"1440p", ""}, // MP4
		266: {"2160p", ""}, // MP4
		271: {"1440p", ""}, // WEBM
		272: {"4320p", ""}, // WEBM
		278: {"144p", ""},  // WEBM
		298: {"720p", ""},  // MP4
		299: {"1080p", ""}, // MP4
		302: {"720p", ""},  // WEBM
		303: {"1080p", ""}, // WEBM
		308: {"1440p", ""}, // WEBM
		313: {"2160p", ""}, // WEBM
		315: {"2160p", ""}, // WEBM
		330: {"144p", ""},  // WEBM
		331: {"240p", ""},  // WEBM
		332: {"360p", ""},  // WEBM
		333: {"480p", ""},  // WEBM
		334: {"720p", ""},  // WEBM
		335: {"1080p", ""}, // WEBM
		336: {"1440p", ""}, // WEBM
		337: {"2160p", ""}, // WEBM
		394: {"144p", ""},  // MP4
		395: {"240p", ""},  // MP4
		396: {"360p", ""},  // MP4
		397: {"480p", ""},  // MP4
		398: {"720p", ""},  // MP4
		399: {"1080p", ""}, // MP4
		400: {"1440p", ""}, // MP4
		401: {"2160p", ""}, // MP4
		402: {"4320p", ""}, // MP4
		571: {"4320p", ""}, // MP4
		694: {"144p", ""},  // MP4
		695: {"240p", ""},  // MP4
		696: {"360p", ""},  // MP4
		697: {"480p", ""},  // MP4
		698: {"720p", ""},  // MP4
		699: {"1080p", ""}, // MP4
		700: {"1440p", ""}, // MP4
		701: {"2160p", ""}, // MP4
		702: {"4320p", ""}, // MP4

		// DASH AUDIO
		139: {"", "48kbps"},  // MP4
		140: {"", "128kbps"}, // MP4
		141: {"", "256kbps"}, // MP4
		171: {"", "128kbps"}, // WEBM
		172: {"", "256kbps"}, // WEBM
		249: {"", "50kbps"},  // WEBM
		250: {"", "70kbps"},  // WEBM
		251: {"", "160kbps"}, // WEBM
		256: {"", "192kbps"}, // MP4
		258: {"", "384kbps"}, // MP4
		325: {"", ""},        // MP4
		328: {"", ""},        // MP4
	}
)

type Format struct {
	ITag     int
	Url      string
	Fps      int
	MineType string   // video/webm
	Codecs   []string // ['vp8', 'vorbis']
	Type     string   // video
	SubType  string   // mp4

	VideoCodecs string
	AudioCodecs string

	Res      string
	BitRate  int
	FileSize int64

	DurationMs int64
}

func (f *Format) isAdaptive() bool {
	return len(f.Codecs)%2 == 1
}

func (f *Format) includesAudioTrack() bool {
	isProgressive := !f.isAdaptive()
	return isProgressive || f.Type == "audio"
}

func (f *Format) includesVideoTrack() bool {
	isProgressive := !f.isAdaptive()
	return isProgressive || f.Type == "video"
}

func (f *Format) parseCodecs() (video, audio string) {
	if !f.isAdaptive() {
		video, audio = f.Codecs[0], f.Codecs[1]
	} else if f.includesVideoTrack() {
		video = f.Codecs[0]
	} else if f.includesAudioTrack() {
		audio = f.Codecs[0]
	}

	return video, audio
}

type audioFormat struct {
	ITag             int    `json:"itag"`
	Url              string `json:"url"`
	Fps              int    `json:"fps"`
	MimeType         string `json:"mimeType"`
	BitRate          int    `json:"bitrate"`
	AudioSampleRate  string `json:"audioSampleRate"`
	ContentLength    string `json:"contentLength"`
	ApproxDurationMs string `json:"approxDurationMs"`
}

type videoFormat struct {
	ITag             int    `json:"itag"`
	Url              string `json:"url"`
	Fps              int    `json:"fps"`
	MimeType         string `json:"mimeType"`
	BitRate          int    `json:"bitrate"`
	Quality          string `json:"quality"`
	ContentLength    string `json:"contentLength"`
	ApproxDurationMs string `json:"approxDurationMs"`
}

func fetchInfoFromAPI(videoID string) (audios []audioFormat, videos []videoFormat, err error) {
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

	raw, err := util.POST(u, util.WithHeader(headers), util.WithBody(body), util.WithRetry(2))
	if err != nil {
		fmt.Println(err)
		return nil, nil, err
	}

	var response struct {
		StreamingData struct {
			ExpiresInSeconds string        `json:"expiresInSeconds"`
			Formats          []audioFormat `json:"formats"`
			AdaptiveFormats  []videoFormat `json:"adaptiveFormats"`
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

func fetchInfoFromWeb(videoID string) (audios []audioFormat, videos []videoFormat, err error) {
	u := "https://www.youtube.com/watch?v=" + videoID
	raw, err := util.GET(u, util.WithRetry(3), util.WithHeader(map[string]string{
		"user-agent": "Linux",
	}))
	if err != nil {
		return audios, videos, err
	}

	r := regexp.MustCompile(`(?s:ytInitialPlayerResponse\s*=\s*(.*?);\s*</script>)`)
	values := r.FindAllStringSubmatch(string(raw), 1)
	if len(values) == 0 || len(values[0]) < 2 {
		return audios, videos, fmt.Errorf("get youtube web no response: %v", u)
	}

	var response struct {
		StreamingData struct {
			ExpiresInSeconds string        `json:"expiresInSeconds"`
			Formats          []audioFormat `json:"formats"`
			AdaptiveFormats  []videoFormat `json:"adaptiveFormats"`
		} `json:"streamingData"`
	}
	err = json.Unmarshal([]byte(values[0][1]), &response)
	if err != nil {
		return nil, nil, fmt.Errorf("decode player failed: %w", err)
	}

	if response.StreamingData.ExpiresInSeconds == "" {
		return nil, nil, fmt.Errorf("palyer invalid")
	}
	return response.StreamingData.Formats, response.StreamingData.AdaptiveFormats, nil
}

func getProfile(iTag int) (res, bitrate string) {
	if val, ok := itags[iTag]; ok {
		res, bitrate = val[0], val[1]
	}
	return res, bitrate
}

func format(data interface{}) (val Format) {
	pattern := regexp.MustCompile(`(\w+\/\w+)\;\scodecs=\"([a-zA-Z-0-9.,\s]*)\"`)
	codes := func(mimeTypeCodecs string) (mimeType string, codecs []string) {
		tokens := pattern.FindAllStringSubmatch(mimeTypeCodecs, -1)
		if len(tokens) == 0 || len(tokens[0]) < 2 {
			return "", nil
		}
		mimeType = tokens[0][1]
		split := strings.Split(tokens[0][1], ",")
		for _, v := range split {
			codecs = append(codecs, strings.TrimSpace(v))
		}
		return mimeType, codecs
	}
	subtype := func(mimeType string) (_type, subType string) {
		kv := strings.Split(mimeType, "/")
		if len(kv) < 2 {
			return "", ""
		}

		return kv[0], kv[1]
	}

	switch v := data.(type) {
	case audioFormat:
		val.Url = v.Url
		val.Fps = v.Fps
		val.ITag = v.ITag
		val.MineType, val.Codecs = codes(v.MimeType)
		val.Type, val.SubType = subtype(val.MineType)
		val.BitRate = v.BitRate
		val.FileSize, _ = strconv.ParseInt(v.ContentLength, 10, 64)
		val.Res, _ = getProfile(val.ITag)
		val.VideoCodecs, val.AudioCodecs = val.parseCodecs()
		val.DurationMs, _ = strconv.ParseInt(v.ApproxDurationMs, 10, 64)

	case videoFormat:
		val.Url = v.Url
		val.Fps = v.Fps
		val.ITag = v.ITag
		val.MineType, val.Codecs = codes(v.MimeType)
		val.Type, val.SubType = subtype(val.MineType)
		val.BitRate = v.BitRate
		val.FileSize, _ = strconv.ParseInt(v.ContentLength, 10, 64)
		val.Res, _ = getProfile(val.ITag)
		val.VideoCodecs, val.AudioCodecs = val.parseCodecs()
		val.DurationMs, _ = strconv.ParseInt(v.ApproxDurationMs, 10, 64)
	}

	return val
}

type Filter func(Format) bool

func WithVideoOnly(format Format) bool {
	return format.includesVideoTrack() && !format.includesAudioTrack()
}

func WithAudioOnly(format Format) bool {
	return format.includesAudioTrack() && !format.includesVideoTrack()
}

func QualityOrder(i, j Format) bool {
	if i.Res == j.Res {
		return i.Fps > j.Fps
	}

	get := func(resource string) int {
		if strings.HasSuffix(resource, "p") {
			v, _ := strconv.ParseInt(resource[:len(resource)-1], 10, 64)
			return int(v)
		}
		return 0
	}

	return get(i.Res) > get(j.Res)
}

type YouTube struct {
	VideoID string
	formats []Format

	exec     []func()
	initOnce sync.Once
	err      error
}

func (tube *YouTube) init() {
	if tube.VideoID == "" {
		tube.err = fmt.Errorf("VideoID not exist")
		return
	}

	tube.initOnce.Do(func() {
		audios, videos, err := fetchInfoFromAPI(tube.VideoID)
		if err != nil {
			fmt.Println("22222222222")
			audios, videos, err = fetchInfoFromWeb(tube.VideoID)
		}
		if err != nil {
			tube.err = err
			return
		}

		for _, audio := range audios {
			tube.formats = append(tube.formats, format(audio))
		}

		for _, video := range videos {
			tube.formats = append(tube.formats, format(video))
		}
	})

	return
}

func (tube *YouTube) Filter(options ...Filter) *YouTube {
	tube.exec = append(tube.exec, func() {
		var formats []Format
		for i := range tube.formats {
			ok := true
			for _, option := range options {
				if !option(tube.formats[i]) {
					ok = false
					break
				}
			}
			if ok {
				formats = append(formats, tube.formats[i])
			}
		}
		tube.formats = formats
	})

	return tube
}

func (tube *YouTube) OrderBy(order func(i, j Format) bool) *YouTube {
	tube.exec = append(tube.exec, func() {
		sort.Slice(tube.formats, func(i, j int) bool {
			return order(tube.formats[i], tube.formats[j])
		})
	})

	return tube
}

func (tube *YouTube) execute() error {
	tube.init()
	if tube.err != nil {
		return tube.err
	}

	for _, exec := range tube.exec {
		exec()
		if tube.err != nil {
			return tube.err
		}
	}
	return tube.err
}

func (tube *YouTube) IndexOf(i int) (format Format, err error) {
	err = tube.execute()
	if err != nil {
		return format, err
	}

	if i >= 0 && len(tube.formats) > i {
		return tube.formats[i], nil
	}

	return format, fmt.Errorf("no result")
}

func (tube *YouTube) All() (format []Format, err error) {
	err = tube.execute()
	if err != nil {
		return format, err
	}

	return tube.formats, nil
}

func (tube *YouTube) First() (format Format, err error) {
	return tube.IndexOf(0)
}

func (tube *YouTube) Last() (format Format, err error) {
	err = tube.execute()
	if err != nil {
		return format, err
	}

	if len(tube.formats) > 0 {
		return tube.formats[len(tube.formats)-1], nil
	}

	return format, fmt.Errorf("no result")
}

func (f *Format) Download(dst string) error {
	fd, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}

	size := f.FileSize
	download := int64(0)
	log.Printf("start download audio file: %v", f.Url)
	for size == 0 || download < size {
		stopPos := download + fileSize

		// size != 0
		if size > 0 {
			if stopPos > size {
				stopPos = size
			}
		}

		u := fmt.Sprintf("%v&range=%v-%v", f.Url, download, stopPos)
		raw, err := util.GET(u, util.WithRetry(3))
		if err != nil {
			return fmt.Errorf("GET failed: %w", err)
		}

		n, err := fd.WriteAt(raw, download)
		if n != len(raw) || err != nil {
			return fmt.Errorf("WriteAt failed: %w", err)
		}

		// size == 0
		if size == 0 && len(raw) < fileSize {
			break
		}
		// size != 0
		if size != 0 && stopPos == size {
			break
		}

		download = stopPos
		log.Printf("current: %v, size: %v", download, size)
	}

	return nil
}

func FetchYouTubeAudio(videoID, dst string) error {
	tube := YouTube{VideoID: videoID}
	format, err := tube.Filter(WithAudioOnly).First()
	if err != nil {
		return err
	}

	return format.Download(dst)
}

func FetchYouTubeVideo(videoID, dst string) error {
	tube := YouTube{VideoID: videoID}
	format, err := tube.Filter(WithVideoOnly).First()
	if err != nil {
		return err
	}

	return format.Download(dst)
}
