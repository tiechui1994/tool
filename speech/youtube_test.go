package speech

import (
	"testing"
)

func TestYoutubeDownload(t *testing.T) {
	err := YoutubeDownload("https://www.youtube.com/watch?v=dvKMbiolsw4",
		"./xyz.mp4", MP4480PFormat)
	t.Logf("%v", err)
}

func TestSpeechToText(t *testing.T) {
	raw, err := SpeechToText("/tmp/music1242919276/youtube.mp3")
	t.Logf("%v, %v", raw, err)
}

func TestFetchLanZou1(t *testing.T) {
	files, err := FetchLanZouInfo("https://wwjp.lanzoul.com/b03ka1s9c", "520")
	if err == nil {
		for _, f := range files {
			t.Logf("name: %v, url: %v, download: %v", f.Name, f.Share, f.Download)
			u, err := LanZouRealURL(f.Download)
			t.Logf("download: %v, %v", err, u)
			return
		}
	}

	t.Logf("%v", err)
}

func TestFetchLanZou2(t *testing.T) {
	files, err := FetchLanZouInfo("https://wwmq.lanzouy.com/iwYWX0wrtyeh", "1122")
	if err == nil {
		for _, f := range files {
			t.Logf("name: %v, url: %v, download: %v", f.Name, f.Share, f.Download)
			u, err := LanZouRealURL(f.Download)
			t.Logf("download: %v, %v", err, u)
			return
		}
	}

	t.Logf("%v", err)
}

func TestFetchInfo(t *testing.T) {
	err := FetchYouTubeAudio("IjR0BS-MIBs", "/tmp/youtube.mp3")
	t.Logf("%v", err)
}