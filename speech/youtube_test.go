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

func TestFetch(t *testing.T) {
	files, err := FetchLanZouInfo("https://wwfr.lanzoul.com/b03k94ueb", "123")
	if err == nil {
		for _, f := range files {
			t.Logf("name: %v, url: %v, download: %v", f.Name, f.Share, f.Download)
			err = LanZouRealURL(&f)
			t.Logf("download: %v, %v", err, f.URL)
			return
		}
	}

	t.Logf("%v", err)
}
