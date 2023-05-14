package speech

import (
	"testing"
)

func TestYoutubeDownload(t *testing.T) {
	err := YoutubeDownload("https://www.youtube.com/watch?v=dvKMbiolsw4",
		"./xyz.mp4", MP4480PFormat)
	t.Logf("%v", err)
}

func TestFetch(t *testing.T) {
	files, err := FetchLanZouInfo("https://wwfr.lanzoul.com/b03k94ueb", "123")
	if err == nil {
		for _, f := range files {
			t.Logf("name: %v, url: %v, download: %v", f.Name, f.ShareURL, f.Download)
		}
	}
}
