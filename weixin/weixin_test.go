package weixin

import (
	"io/ioutil"
	"testing"
)

var token = "65_WN79odGQ3E-mjZfZaTKfyl"

func TestUploadNewsImage(t *testing.T) {
	url, err := UploadImage(token, "/home/user/Downloads/flower.jpg")
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	t.Log("url", url)
}

func TestAddPersistMaterial(t *testing.T) {
	url, id, err := AddPersistMaterial(token, MediaThumb, "/home/user/Downloads/superb.jpg")
	if err != nil {
		t.Fatalf("AddPersistMaterial: %v", err)
	}
	t.Log("url", url)
	t.Log("id", id)
}

func TestPersistMaterialList(t *testing.T) {
	list, err := PersistMaterialList(token, MediaImage, 0, 10)
	if err != nil {
		t.Fatalf("PersistMaterialList: %v", err)
	}

	for _, v := range list.([]Media) {
		t.Logf("name:%v, mediaid:%v, url:%v", v.Name, v.MediaID, v.Url)
	}
}

// crAgK-cmXMbGEq9kZymbzTLmVnS0qOcgZ0WY9r-EMYlUBa-0hUzLV8CHUT3Nknbr
func TestAddDraft(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/user/Desktop/xyz.html")
	news := Article{
		Title:            "测试case",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzTLmVnS0qOcgZ0WY9r-EMYlUBa-0hUzLV8CHUT3Nknbr",
		ContentSourceUrl: "https://www.baidu.com",
		Content:          string(data),
	}
	id, err := AddDraft(token, news)
	if err != nil {
		t.Fatalf("AddDraft: %v", err)
	}
	t.Log("id", id)
}

func TestUpdateDraft(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/quinn/Desktop/yyy.html")
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzWP0k6ziJ5W_OayqgGzT2Go",
		ContentSourceUrl: "https://www.baidu.com",
		Content:          string(data),
	}
	err := UpdateDraft(token, "crAgK-cmXMbGEq9kZymbzf_ZnZjo4XV7HHYef7yF_ZI", 0, news)
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
}

func TestGetDraft(t *testing.T) {
	news, err := GetDraft(token, "crAgK-cmXMbGEq9kZymbzX38HXl7Wr8_f1Bifz9Tc5pOT5c_i2b8L-UJschJbmL0")
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	t.Logf("url:%v", news[0].Url)
}
