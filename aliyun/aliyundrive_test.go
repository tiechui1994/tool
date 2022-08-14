package aliyun

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/tiechui1994/tool/aliyun/aliyundrive"
	"github.com/tiechui1994/tool/util"
)

var token aliyundrive.Token

func init() {
	util.RegisterDNS([]string{
		"114.114.114.114:53",           // 天翼
		"223.5.5.5:53", "223.6.6.6:53", // 阿里云
		"218.30.19.40:53", "218.30.19.50:53", // 西安
	})

	util.RegisterFileJar()
	raw, _ := ioutil.ReadFile(filepath.Join(util.JarDir(), "drive.json"))
	json.Unmarshal(raw, &token)
}

func TestDriveFsList(t *testing.T) {
	drive := NewDriveFs(token)
	list, err := drive.List("/book")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, node := range list {
		t.Logf("name:%v, filetype: %v", node.Name, node.Type)
	}
}
