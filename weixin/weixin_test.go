package weixin

import "testing"

var token = "48_5RXgW2Tw8I9gUhFetD8oQUg7-1ZEpKrRaZE5BqD6c1lWHF7YfmqehczE98bSJhmIeAlJmdC53CjJg3zWw184I_PvJNLyjwCymntskHtgrVSs0ygVSkx9rewZ-59ex-fiW1CBurR9ziHmvFISJWNaACAVIZ"

func TestUploadNewsImage(t *testing.T) {
	id, err := UploadNewsImage(token, "/home/quinn/Pictures/develop_runtime_cfun7.jpeg")
	t.Log("id", id)
	t.Log("err", err)
}

func TestAddPersitMaterial(t *testing.T) {
	id, err := AddPersitMaterial(token, MediaImage, "/home/quinn/Pictures/develop_runtime_cfun7.jpeg")
	t.Log("id", id)
	t.Log("err", err)
}

func TestMediaList(t *testing.T) {
	id, err := MaterialList(token, "news")
	t.Log("id", id)
	t.Log("err", err)
}
