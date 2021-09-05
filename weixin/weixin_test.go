package weixin

import (
	"io/ioutil"
	"testing"
)

var token = "48_sU3K7Ezw5aL3Kyd4ChGcG_f-Mkm_bTij5jTv9u224hXkbMAXX7QBbvx4rLLX-lqd7JkbhqwlIQ5pbgKIlsledrmJdCmyFlXMuHkvAkWAYJcTmOb1L0OzHCSHsoz7izmjogWWoNTFkaDw43o3NSKdAEAGLD"

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
	list, err := PersitMaterialList(token, MediaImage, 0, 1)
	t.Log("id", list)
	t.Log("err", err)
}

// crAgK-cmXMbGEq9kZymbzf_ZnZjo4XV7HHYef7yF_ZI
func TestAddPersitNews(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/quinn/Desktop/www.html")
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzWP0k6ziJ5W_OayqgGzT2Go",
		ContentSourceUrl: "https://www.baidu.com",
		Content: string(data),
	}
	id, err := AddPersitNews(token, news)
	t.Log("id", id)
	t.Log("err", err)
}

func TestUpdatePersitNews(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/quinn/Desktop/yyy.html")
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzWP0k6ziJ5W_OayqgGzT2Go",
		ContentSourceUrl: "https://www.baidu.com",
		Content: string(data),
	}
	err := UpdatePersitNews(token, "crAgK-cmXMbGEq9kZymbzf_ZnZjo4XV7HHYef7yF_ZI", 0, news)
	t.Log("err", err)
}

func TestGetPersitNews(t *testing.T) {
	news, err := GetPersitNews(token, "crAgK-cmXMbGEq9kZymbzfRNQraVlfgjkPuDLTrU2yY")
	t.Log("news", news)
	t.Log("err", err)
}
