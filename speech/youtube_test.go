package speech

import (
	"testing"
)

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

func TestYoutube(t *testing.T) {
	yt := YouTube{VideoID: "IjR0BS-MIBs"}
	formats, err := yt.Filter(WithVideoOnly).OrderBy(QualityOrder).All()
	t.Logf("%v", err)
	for _, format := range formats {
		t.Logf("%v, %v, %v, %v", format.Res, format.Fps, format.FileSize, format.SubType)
	}

	t.Logf("===========")

	formats, err = yt.OrderBy(QualityOrder).All()
	t.Logf("%v", err)
	for _, format := range formats {
		t.Logf("%v, %v, %v, %v", format.Res, format.Fps, format.FileSize, format.SubType)
	}
}
