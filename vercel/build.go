package vercel

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

const (
	endpoint = "https://api.vercel.com"
)

var (
	rootdir, srcdir string
)

func init() {
	root, _ := filepath.Abs(".")
	rootdir = filepath.Join(root, "root")
	srcdir = filepath.Join(root, "root/src")

	os.RemoveAll(rootdir)
	os.MkdirAll(srcdir, 0766)
}

type File struct {
	File    string `json:"file"`
	Sha1    string `json:"sha"`
	Size    int64  `json:"size"`
	Url     string `json:"-"`
	AbsPath string `json:"-"`
	Name    string `json:"-"`
}

func Files(dir string) (files []File, err error) {
	dir, _ = filepath.Abs(dir)
	exec.Command("bash", "-c", fmt.Sprintf("cp -r %v/* %v", dir, srcdir)).
		Output()

	packagejson(rootdir)
	n := len(rootdir) + 1
	filepath.Walk(rootdir, func(path string, info os.FileInfo, err error) error {
		if info.Name() == "index.html" || info.Name() == "index.htm" {
			return err
		}

		if !info.IsDir() {
			files = append(files, File{
				Size:    info.Size(),
				Name:    info.Name(),
				AbsPath: path,
				File:    path[n:],
			})
		}
		return err
	})

	err = index("MySQL细节内容", "详细内容", filepath.Join(rootdir, "index.html"), files)
	if err != nil {
		return files, err
	}
	info, err := os.Stat(filepath.Join(rootdir, "index.html"))
	if err != nil {
		return files, err
	}
	files = append(files, File{
		Size:    info.Size(),
		Name:    info.Name(),
		AbsPath: filepath.Join(rootdir, "index.html"),
		File:    "index.html",
	})

	return files, nil
}

func Upload(file *File, token string) (err error) {
	data, err := ioutil.ReadFile(file.AbsPath)
	if err != nil {
		return err
	}

	hash := sha1.New()
	hash.Write(data)

	digest := hex.EncodeToString(hash.Sum(nil))
	header := map[string]string{
		"Authorization":  "Bearer " + token,
		"Content-Length": fmt.Sprintf("%v", file.Size),
		"x-now-digest":   digest,
	}
	u := endpoint + "/v2/now/files"

	raw, err := util.POST(u, util.WithBody(data),  util.WithHeader(header))
	if err != nil {
		return err
	}

	log.Infoln("%v", string(raw))

	var response struct {
		Urls []string
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return err
	}

	response.Urls = append(response.Urls, "https://dmmcy0pwk6bqi.cloudfront.net/"+digest)
	file.Sha1 = digest
	file.Url = response.Urls[0]
	return nil
}

func Deploy(files []File, token string) error {
	for _, i := range files {
		log.Infoln("file:%v, size:%v", i.File, i.Size)
	}

	// iad1 美国华盛顿特区
	// sfo1 美国旧金山
	regions := []string{"sfo1"}
	// framework: hugo, vue, angular, nextjs, hexo, jekyll, create-react-app, ionic-react
	body := map[string]interface{}{
		"name":    "mysql",
		"files":   files,
		"target":  "production",
		"regions": regions,
		"projectSettings": map[string]string{
			"outputDirectory": "public",
			"installCommand":  "npm install",
			"buildCommand":    "npm run build",
			"devCommand":      "npm run dev",
			"framework":       "vue",
		},
	}
	header := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	u := endpoint + "/v12/now/deployments"
	raw, err := util.POST(u,  util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return err
	}

	log.Infoln("%v", string(raw))
	return nil
}

func packagejson(root string) {
	// package.json
	packjson := `
{
  "name": "vercel",
  "version": "1.0.0",
  "description": "vercel deploy",
  "license": "GPL",
  "private": false,
  "scripts": {
    "dev": "echo 'dev'",
    "build": "rm -rf public && mkdir -p public/static && cp -r src/* public/static && cp index.html public"
  },
  "dependencies": {
  },
  "devDependencies": {
  },
  "engines": {
    "node": ">= 12.0.0",
    "npm": ">= 6.0.0"
  }
}`
	ioutil.WriteFile(filepath.Join(root, "package.json"), []byte(packjson), 0666)

	// css
	os.MkdirAll(filepath.Join(root, "src/css"), 0766)
	download := func(hash, name string, wg *sync.WaitGroup) {
		defer func() {
			if wg != nil {
				wg.Done()
			}
		}()

		raw, err := util.GET("https://dmmcy0pwk6bqi.cloudfront.net/"+hash, util.WithRetry(3))
		if err != nil {
			return
		}

		ioutil.WriteFile(filepath.Join(root, "src/css/"+name), raw, 0666)
	}

	var wg sync.WaitGroup
	wg.Add(4 + 2)
	for _, hash := range []string{
		"ccb8392c14f927325cc42daca346fe1eaafd450b",
		"3de8d89bfb4f3e3c5aac7b207a24d9709c13e786",
		"2ccd97a7d5e0d9f1a251a2139ed2bbc42a4521db",
		"40941051cec42bd216f5d6358223f0d8becd1446",
	} {
		go download(hash, hash+".css", &wg)
	}

	go download("d7b323bf221634391b939669296c9552b4870afd", "logo.svg", &wg)
	go download("2a54a7483278eb70bd8194ee3ff58a42aad5eae9", "app.18d1dc1fa31fd2c79c4a53d88bb831b0.css", &wg)

	wg.Wait()
}

func index(title, h1, path string, files []File) error {
	// index.html
	html := `
<html>
  <head>
    <meta charset="utf-8">
    <title>${title}</title>
    <link rel="icon shortcut" type="image/svg+xml" href="/static/css/logo.svg">
    <meta name="viewport" content="width=device-width,initial-scale=1,minimum-scale=1,maximum-scale=1,user-scalable=0,minimal-ui">
    <meta name="browsermode" content="application"> 
    <meta name="screen-orientation" content="portrait">
    <meta name="full-screen" content="yes">
    <meta name="wap-font-scale" content="no">
    <meta name="nightmode" content="disable">
    <meta name="imagemode" content="force">
    <meta name="x5-page-mode" content="app">
    <meta name="x5-orientation" content="portrait">
	<meta name="x5-fullscreen" content="true">
	<meta name="apple-mobile-web-app-capable" content="yes">
	<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
	<link rel="stylesheet" type="text/css" href="/static/css/app.18d1dc1fa31fd2c79c4a53d88bb831b0.css">
	<link rel="stylesheet" type="text/css" href="/static/css/ccb8392c14f927325cc42daca346fe1eaafd450b.css">
	<link rel="stylesheet" type="text/css" href="/static/css/3de8d89bfb4f3e3c5aac7b207a24d9709c13e786.css">
	<link rel="stylesheet" type="text/css" href="/static/css/2ccd97a7d5e0d9f1a251a2139ed2bbc42a4521db.css">
	<link rel="stylesheet" type="text/css" href="/static/css/40941051cec42bd216f5d6358223f0d8becd1446.css">
  </head>
  <body>
	<div id="app" class="fillcontain">
	  <div class="markdown-html">
	    <div class="body">
	      <div class="markdown-container">
            <h1>${h1}</h1>
	        <div><ul>${list}</ul></div>
	      </div>
        </div>
	  </div>
    </div>
  </body>
</html>
`
	var list string
	for _, file := range files {
		if strings.HasSuffix(file.Name, ".html") {
			list += fmt.Sprintf(`<li><a href="/static/%v" target="_parent" rel="noopener">%v</a></li>`,
				file.Name, strings.TrimSuffix(file.Name, ".html"))
		}
	}

	html = strings.ReplaceAll(html, "${title}", title)
	html = strings.ReplaceAll(html, "${h1}", h1)
	html = strings.ReplaceAll(html, "${list}", list)

	return ioutil.WriteFile(path, []byte(html), 0666)
}
