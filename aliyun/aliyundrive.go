package aliyun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/aliyun/aliyundrive"
)

func FindDriveDrive(path string, accesstoken aliyundrive.Token) (file aliyundrive.File, err error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return file, errors.New("invalid path")
	}

	tokens := strings.Split(path[1:], "/")
	files, err := aliyundrive.Files("root", accesstoken)
	if err != nil {
		return file, err
	}

	exist := false
	for _, f := range files {
		if f.Name == tokens[0] {
			exist = true
			file = f
			break
		}
	}

	if !exist {
		return file, errors.New("no exist project")
	}

	tokens = tokens[1:]
	rootid := file.FileID
	for _, token := range tokens {
		files, err := aliyundrive.Files(rootid, accesstoken)
		if err != nil {
			return file, err
		}

		exist := false
		for _, f := range files {
			if f.Name == token {
				file = f
				rootid = f.FileID
				exist = true
				break
			}
		}

		if !exist {
			return file, errors.New("no exist path: " + token)
		}
	}

	return file, nil
}

type DriveFs struct {
	mux         sync.Mutex
	rootid      string
	accesstoken aliyundrive.Token
	root        *FileNode
	pwd         string
}

func NewDriveFs(accesstoken aliyundrive.Token) *DriveFs {
	p := DriveFs{rootid: "root", accesstoken: accesstoken, pwd: "/"}

	p.root = &FileNode{
		Type:   Node_Dir,
		Name:   "/",
		NodeId: p.rootid,
		Child:  make([]*FileNode, 0, 10),
	}
	return &p
}

func (d *DriveFs) handlePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path[0] != '/' {
		return "", errors.New("invalid path")
	}

	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	return path, nil
}

func (d *DriveFs) find(path string) (node *FileNode, prefix string, exist bool, err error) {
	path, err = d.handlePath(path)
	if err != nil {
		return node, prefix, exist, err
	}

	if path == "/" {
		return d.root, "/", true, nil
	}

	subpaths := strings.Split(path[1:], "/")

	node = d.root
	child := d.root.Child

	for idx, subpath := range subpaths {
		// subpath
		if val, ok := search(child, subpath); ok {
			node = val
			child = val.Child
			if idx == len(subpaths)-1 {
				exist = true
				return node, "/" + strings.Join(subpaths, "/"), exist, nil
			}
			continue
		}

		// subpath not exist
		exist = false
		if idx == 0 {
			return node, "", exist, nil
		}

		return node, "/" + strings.Join(subpaths[:idx], "/"), exist, nil
	}

	return node, "", false, errors.New("invalid path")
}

func (d *DriveFs) fetchDirs(rootid string, paths []string, private ...interface{}) (list []*FileNode, err error) {
	list = make([]*FileNode, 0, 10)
	files, err := aliyundrive.Files(rootid, d.accesstoken)
	if err == nil {
		for _, file := range files {
			var _type Type
			if file.Type == aliyundrive.TYPE_FILE {
				_type = Node_File
			} else {
				_type = Node_Dir
			}
			node := &FileNode{
				Type:     _type,
				Name:     file.Name,
				NodeId:   file.FileID,
				ParentId: file.ParentID,
				Url:      file.Url,
				Size:     file.Size,
				Child:    make([]*FileNode, 0, 10),
				Hash:     file.Hash,
				Private:  private,
			}
			if len(paths) > 0 && node.Name == paths[0] && node.Type == Node_Dir {
				node.Child, err = d.fetchDirs(node.NodeId, paths[1:], private)
				if err != nil {
					return list, err
				}
			}
			list = append(list, node)
		}
	}

	return list, err
}

func (d *DriveFs) fetchFiles(rootid string, private ...interface{}) (list []*FileNode, err error) {
	list = make([]*FileNode, 0, 10)
	files, err := aliyundrive.Files(rootid, d.accesstoken)
	if err == nil {
		for _, file := range files {
			var _type Type
			if file.Type == aliyundrive.TYPE_FILE {
				_type = Node_File
			} else {
				_type = Node_Dir
			}
			node := &FileNode{
				Type:     _type,
				Name:     file.Name,
				NodeId:   file.FileID,
				ParentId: file.ParentID,
				Url:      file.Url,
				Size:     file.Size,
				Hash:     file.Hash,
				Private:  private,
			}
			list = append(list, node)
		}
	}

	return list, err
}

func (d *DriveFs) mkdir(path string) (node *FileNode, err error) {
	path, err = d.handlePath(path)
	if err != nil {
		return node, err
	}

	// query path
	accnode, prefix, exist, err := d.find(path)
	if err != nil {
		return node, err
	}
	if exist {
		accnode.Child, err = d.fetchFiles(accnode.NodeId)
		return accnode, err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	// sync accnode
	subpath := strings.Split(path[len(prefix)+1:], "/")
	accnode.Child, err = d.fetchDirs(accnode.NodeId, subpath)
	if err != nil {
		return node, err
	}

	// query again
	accnode, prefix, exist, err = d.find(path)
	if err != nil {
		return node, err
	}
	if exist {
		accnode.Child, err = d.fetchFiles(accnode.NodeId)
		return accnode, err
	}

	// new path
	parent := accnode
	subpath = strings.Split(path[len(prefix)+1:], "/")
	for _, token := range subpath {
		upload, err := aliyundrive.CreateDirectory(token, parent.NodeId, d.accesstoken)
		if err != nil {
			return node, err
		}

		curnode := &FileNode{
			Type:     Node_Dir,
			Name:     token,
			ParentId: upload.ParentID,
			NodeId:   upload.FileID,
		}
		parent = curnode
	}

	return parent, nil
}

func (d *DriveFs) fileupload(path, target string) error {
	targetNode, err := d.mkdir(target)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	filenode, exist := targetNode.Search(info.Name())
	if exist && filenode.Size == int(info.Size()) {
		return nil
	}
	if exist {
		err = aliyundrive.Delete([]aliyundrive.File{{FileID: filenode.NodeId}}, d.accesstoken)
		if err != nil {
			return err
		}
	}

	_, err = aliyundrive.UploadFile(path, targetNode.NodeId, d.accesstoken)
	return err
}

func (d *DriveFs) Rename(before, after, dir string) error {
	node, _, exist, err := d.find(dir)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not exist")
	}

	files, err := d.fetchFiles(node.NodeId)
	if err != nil {
		return err
	}
	if val, exist := search(files, before); exist {
		return aliyundrive.Rename(after, val.NodeId, d.accesstoken)
	}

	dirs, err := d.fetchDirs(node.NodeId, nil)
	if err != nil {
		return err
	}
	if val, exist := search(dirs, before); exist {
		return aliyundrive.Rename(after, val.NodeId, d.accesstoken)
	}

	return nil
}

func (d *DriveFs) Move(src, dst string) error {
	srcnode, _, exist, err := d.find(src)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not found")
	}

	dstnode, err := d.mkdir(dst)
	if err != nil {
		return err
	}

	files := []aliyundrive.File{{
		FileID:  srcnode.NodeId,
		DriveID: d.accesstoken.DriveID,
	}}
	return aliyundrive.Move(dstnode.NodeId, files, d.accesstoken)
}

func (d *DriveFs) Copy(src, dst string) error {
	return errors.New("invalid node")
}

func (d *DriveFs) Delete(path string) error {
	path, err := d.handlePath(path)
	if err != nil {
		return err
	}

	var tokens []string
	node, prefix, exist, err := d.find(path)
	if err != nil {
		return err
	}
	if exist {
		goto remove
	}

	// sync dirs
	d.mux.Lock()
	tokens = strings.Split(path[len(prefix)+1:], "/")
	node.Child, err = d.fetchDirs(node.NodeId, tokens)
	if err != nil {
		d.mux.Unlock()
		return err
	}
	d.mux.Unlock()

	// query second
	node, prefix, exist, err = d.find(path)
	if err != nil {
		return err
	}
	if !exist {
		return nil
	}

remove:
	file := []aliyundrive.File{
		{
			FileID:  node.NodeId,
			DriveID: d.accesstoken.DriveID,
		},
	}
	return aliyundrive.Delete(file, d.accesstoken)
}

func (d *DriveFs) Download(srcpath, targetdir string) error {
	path, err := d.handlePath(srcpath)
	if err != nil {
		return err
	}

	var tokens []string

	// query first
	accnode, prefix, exist, err := d.find(path)
	if err != nil {
		return err
	}
	if exist {
		goto download
	}

	// sync dirs
	d.mux.Lock()
	tokens = strings.Split(path[len(prefix)+1:], "/")
	accnode.Child, err = d.fetchDirs(accnode.NodeId, tokens)
	if err != nil {
		d.mux.Unlock()
		return err
	}
	d.mux.Unlock()

	// query second
	accnode, prefix, exist, err = d.find(path)
	if err != nil {
		return err
	}
	if exist {
		goto download
	}

	// sync files
	d.mux.Lock()
	accnode.Child, err = d.fetchFiles(accnode.NodeId, accnode.Child)
	if err != nil {
		d.mux.Unlock()
		return err
	}
	d.mux.Unlock()

	// query again
	accnode, prefix, exist, err = d.find(path)
	if err != nil || !exist {
		if err == nil {
			err = errors.New("not exist path")
		}
		return err
	}

download:
	file := aliyundrive.File{
		Size:    accnode.Size,
		FileID:  accnode.NodeId,
		Name:    accnode.Name,
		Url:     accnode.Url,
		DriveID: d.accesstoken.DriveID,
		Hash:    accnode.Hash,
	}

	return aliyundrive.Download(file, 10, targetdir, d.accesstoken)
}

func (d *DriveFs) Upload(path, target string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// file
	if !info.IsDir() {
		return d.fileupload(path, target)
	}

	// dir
	absdir, _ := filepath.Abs(path)
	dirpaths := make(map[string][]string)
	curdir := ""
	filepath.Walk(absdir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			curdir = path
			return nil
		}

		dirpaths[curdir] = append(dirpaths[curdir], path)
		return nil
	})

	wg := sync.WaitGroup{}
	count := 0
	for dir, files := range dirpaths {
		target := filepath.Join(target, dir[len(absdir):])
		for _, file := range files {
			count++
			wg.Add(1)
			f := file
			go func() {
				defer wg.Done()
				d.fileupload(f, target)
			}()
			if count == 5 {
				wg.Wait()
				count = 0
			}
		}
	}

	if count > 0 {
		wg.Wait()
	}

	return nil
}

func (d *DriveFs) List(path string) ([]*FileNode, error) {
	path, err := d.handlePath(path)
	if err != nil {
		return nil, err
	}

	accnode, prefix, exist, err := d.find(path)
	if err != nil {
		return nil, err
	}

	// SYNC
	d.mux.Lock()
	accnode.Child, err = d.fetchFiles(accnode.NodeId, accnode.Child)
	if err != nil {
		d.mux.Unlock()
		return nil, err
	}
	d.mux.Unlock()
	if exist {
		return accnode.Child, nil
	}

	// SYNC
	d.mux.Lock()
	tokens := strings.Split(path[len(prefix)+1:], "/")
	accnode.Child, err = d.fetchDirs(accnode.NodeId, tokens)
	if err != nil {
		d.mux.Unlock()
		return nil, err
	}
	d.mux.Unlock()

	// FIND
	accnode, _, exist, err = d.find(path)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("no found")
	}

	if exist && accnode.Type == Node_File {
		return []*FileNode{accnode}, nil
	}

	d.mux.Lock()
	accnode.Child, err = d.fetchFiles(accnode.NodeId, accnode.Child)
	if err != nil {
		d.mux.Unlock()
		return nil, err
	}
	d.mux.Unlock()
	return accnode.Child, nil
}
