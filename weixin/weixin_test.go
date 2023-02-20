package weixin

import (
	"io/ioutil"
	"testing"
)

var token = "65__urjOi5kgsVuLWxndRheHLtoWqT8UWo9dzBTxUFNMXpdX6L5mm-ok7MnyDQWulpl7m-eu4ACdRDOMooC5ZJw25Q58tzuvVCQayBh49rNdmC-AT_q9-o1X_I_tMJPZgACAYEO"

func TestUploadNewsImage(t *testing.T) {
	id, err := UploadNewsImage(token, "/home/quinn/Downloads/v.jpeg")
	t.Log("id", id)
	t.Log("err", err)
}

func TestAddPersitMaterial(t *testing.T) {
	id, err := AddPersitMaterial(token, MediaImage, "/home/quinn/Downloads/vietnamese-woman.jpg")
	t.Log("id", id)
	t.Log("err", err)
}

func TestMediaList(t *testing.T) {
	list, err := PersitMaterialList(token, MediaImage, 0, 1)
	t.Log("id", list)
	t.Log("err", err)
}

// crAgK-cmXMbGEq9kZymbzf_ZnZjo4XV7HHYef7yF_ZI
func TestAddDraft(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/quinn/Desktop/www.html")
	news := Article{
		Title:            "测试case",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzbQ2SZK08yL2dASCbsNrKRf3UH5s3yDc9ReYrv0IyNbd",
		ContentSourceUrl: "https://www.baidu.com",
		Content: string(data),
	}
	id, err := AddDraft(token, news)
	t.Log("id", id)
	t.Log("err", err)
}

func TestUpdateDraft(t *testing.T) {
	data, _ := ioutil.ReadFile("/home/quinn/Desktop/yyy.html")
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzWP0k6ziJ5W_OayqgGzT2Go",
		ContentSourceUrl: "https://www.baidu.com",
		Content: string(data),
	}
	err := UpdateDraft(token, "crAgK-cmXMbGEq9kZymbzf_ZnZjo4XV7HHYef7yF_ZI", 0, news)
	t.Log("err", err)
}

func TestGetDraft(t *testing.T) {
	news, err := GetDraft(token, "crAgK-cmXMbGEq9kZymbzfRNQraVlfgjkPuDLTrU2yY")
	t.Log("news", news)
	t.Log("err", err)
}
