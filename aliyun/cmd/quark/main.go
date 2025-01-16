package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/aliyun/quark"
	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

type Node struct {
	Name  string
	Files []quark.File
	Child map[string]*Node
}

func buildRoot(drive *quark.DriverQuark) (*Node, error) {
	fetch := func(name, pid string) (*Node, error) {
		files, err := drive.List(pid)
		if err != nil {
			return nil, err
		}

		return &Node{
			Name:  name,
			Files: files,
			Child: map[string]*Node{},
		}, nil
	}

	root, err := fetch("", "")
	if err != nil {
		return nil, err
	}

	nodes := []*Node{root}
	for len(nodes) > 0 {
		node := nodes[0]
		nodes = nodes[1:]

		for _, file := range node.Files {
			if file.File {
				continue
			}

			child, err := fetch(file.FileName, file.Fid)
			if err != nil {
				return nil, err
			}
			node.Child[file.FileName] = child
			nodes = append(nodes, child)
		}
	}

	return root, nil
}

func searchFile(root *Node, path string) ([]quark.File, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid path")
	}

	entries := strings.Split(path, "/")
	current := root
	for index, entry := range entries[1:] {
		if index == len(entries)-2 {
			if _, ok := current.Child[entry]; ok {
				return current.Child[entry].Files, nil
			}

			for _, file := range current.Files {
				if file.FileName == entry {
					return []quark.File{file}, nil
				}
			}

			return nil, fmt.Errorf("file [%v] not found", entry)
		}

		if _, ok := current.Child[entry]; ok {
			current = current.Child[entry]
			continue
		}

		return nil, fmt.Errorf("dir [%v] not found", entry)
	}

	return nil, fmt.Errorf("not found")
}

func Download(down quark.Download, parallel int, dir string) error {
	if parallel > 10 {
		parallel = 10
	}
	if parallel <= 3 {
		parallel = 3
	}

	m32 := uint(1024 * 1024 * 32)
	batch := uint(down.Size) / m32
	if uint(down.Size)%m32 != 0 {
		batch += 1
	}

	fd, err := os.Create(filepath.Join(dir, down.FileName))
	if err != nil {
		return err
	}

	log.Infoln("download file=%q size=%v ", down.FileName, down.Size)
	var wg sync.WaitGroup
	count := 0
	log.Infoln("download file=%q parallel=%v batch %v", down.FileName, parallel, batch)
	for i := uint(0); i < batch; i++ {
		wg.Add(1)
		count += 1
		go func(idx uint) {
			defer wg.Done()
			log.Infoln("download file=%q batch index %v", down.FileName, idx)
			from := idx * m32
			to := (idx+1)*m32 - 1
			if to >= uint(down.Size) {
				to = uint(down.Size) - 1
			}
			header := map[string]string{
				"range": fmt.Sprintf("bytes=%v-%v", from, to),
			}
			for k, v := range down.Header {
				header[k] = v[0]
			}
			raw, err := util.GET(down.DownloadUrl, util.WithHeader(header))
			if err != nil {
				return
			}
			log.Infoln("download file=%q batch index %v success", down.FileName, idx)
			_, _ = fd.WriteAt(raw, int64(from))
			_ = fd.Sync()
		}(i)
		if count == parallel {
			wg.Wait()
			count = 0
		}
	}

	if count > 0 {
		wg.Wait()
	}
	log.Infoln("download file=%q complete", down.FileName)

	return nil
}

func main() {
	cookie := flag.String("cookie", "", "quark cookie")
	path := flag.String("path", "", "quark download path")
	dir := flag.String("dir", ".", "download dir path")
	flag.Parse()

	cookieStr, err := ioutil.ReadFile(*cookie)
	if err != nil {
		fmt.Println("cookie read failed")
		os.Exit(1)
	}

	fs, err := quark.NewQuark(string(cookieStr))
	if err != nil {
		fmt.Println("init Quark failed", err)
		os.Exit(1)
	}

	root, err := buildRoot(fs)
	if err != nil {
		fmt.Println("init build root failed", err)
		os.Exit(1)
	}

	files, err := searchFile(root, *path)
	if err != nil {
		fmt.Println("search download path failed", err)
		os.Exit(1)
	}

	for _, v := range files {
		down, err := fs.Download(v.Fid)
		if err != nil {
			fmt.Println("download failed", v.FileName, err)
			os.Exit(1)
		}
		fmt.Println(down[0].DownloadUrl)
		_ = Download(down[0], 8, *dir)
	}
}
