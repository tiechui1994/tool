package weixin

import "testing"

var token = ""

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
	list, err := PersitMaterialList(token, MediaImage)
	t.Log("id", list)
	t.Log("err", err)
}

// crAgK-cmXMbGEq9kZymbzc9iYfy8UIf4ng6epM602TM
func TestAddPersitNews(t *testing.T) {
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzaHLCE_7aLLFwnWA99U598Y",
		ContentSourceUrl: "https://www.baidu.com",
		Content: `
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
    p {
		font-size: 30px;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
		`,
	}
	id, err := AddPersitNews(token, news)
	t.Log("id", id)
	t.Log("err", err)
}

func TestUpdatePersitNews(t *testing.T) {
	news := Article{
		Title:            "测试当中",
		Author:           "tiechui1994",
		ThumbMediaID:     "crAgK-cmXMbGEq9kZymbzaHLCE_7aLLFwnWA99U598Y",
		ContentSourceUrl: "https://www.baidu.com",
		Content: `
<div>
<h1>Welcome to nginx!</h1>
<p style="font-size: 30px;">If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</div>
		`,
	}
	err := UpdatePersitNews(token, "crAgK-cmXMbGEq9kZymbzc9iYfy8UIf4ng6epM602TM", 0, news)
	t.Log("err", err)
}

func TestGetPersitNews(t *testing.T) {
	news, err := GetPersitNews(token, "crAgK-cmXMbGEq9kZymbzc9iYfy8UIf4ng6epM602TM")
	t.Log("news", news)
	t.Log("err", err)
}
