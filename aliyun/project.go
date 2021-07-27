package aliyun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/util"
)

type FileSystem interface {
	Rename(src, dst, dir string) error
	Move(src, dst string) error
	Copy(src, dst string) error
	Delete(path string) error
	Download(path, dstdir string) error
	Upload(path string, target string) error
}

const (
	Node_Dir  = 1
	Node_File = 2
)

func FindProjectDir(path, orgid string) (c Collection, err error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return c, errors.New("invalid path")
	}

	tokens := strings.Split(path[1:], "/")
	projects, err := Projects(orgid)
	if err != nil {
		return c, err
	}

	var project Project
	exist := false
	for _, p := range projects {
		if p.Name == tokens[0] {
			exist = true
			project = p
			break
		}
	}

	if !exist {
		return c, errors.New("no exist project")
	}

	tokens = tokens[1:]
	rootid := project.RootCollectionId
	for _, token := range tokens {
		collections, err := Collections(rootid, project.ID)
		if err != nil {
			return c, err
		}

		exist := false
		for _, coll := range collections {
			if coll.Title == token {
				c = coll
				rootid = coll.Nodeid
				exist = true
				break
			}
		}

		if !exist {
			return c, errors.New("no exist path: " + token)
		}
	}

	return c, nil
}

func FindProjectFile(path, orgid string) (w Work, err error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return w, errors.New("invalid path")
	}

	dir, name := filepath.Split(path)
	dir = dir[:len(dir)-1]
	c, err := FindProjectDir(dir, orgid)
	if err != nil {
		return w, err
	}

	works, err := Works(c.Nodeid, c.ProjectId)
	if err != nil {
		return w, err
	}

	for _, work := range works {
		if work.FileName == name {
			return work, nil
		}
	}

	return w, errors.New("not exist file:" + name)
}

type FileNode struct {
	Type     int                  `json:"type"`
	Name     string               `json:"name"`
	NodeId   string               `json:"nodeid"`
	ParentId string               `json:"parentid"`
	Updated  string               `json:"updated"`
	Child    map[string]*FileNode `json:"child,omitempty"`
	Url      string               `json:"url,omitempty"`
	Size     int                  `json:"size,omitempty"`
	Private  interface{}          `json:"private,omitempty"`
}

type ProjectFs struct {
	Name  string
	Orgid string

	mux        sync.Mutex
	projectid  string
	rootcollid string
	token      string
	root       *FileNode
}

func NewProject(name, orgid string) (*ProjectFs, error) {
	p := ProjectFs{Name: name, Orgid: orgid}

	list, err := Projects(p.Orgid)
	if err != nil {
		fmt.Println(err, p.Orgid)
		return nil, err
	}

	for _, item := range list {
		if item.Name == p.Name {
			p.projectid = item.ID
			p.rootcollid = item.RootCollectionId
		}
	}

	if p.rootcollid == "" && p.projectid == "" {
		return nil, errors.New("invalid name")
	}

	p.token, err = GetProjectToken(p.projectid, p.rootcollid)
	if err != nil {
		return nil, err
	}

	p.root = &FileNode{
		Type:   Node_Dir,
		Name:   "/",
		NodeId: p.rootcollid,
		Child:  make(map[string]*FileNode),
	}
	return &p, nil
}

func (p *ProjectFs) projectPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path[0] != '/' {
		return "", errors.New("invalid path")
	}

	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	if strings.HasPrefix(path, "/"+p.Name) {
		path = path[len(p.Name)+1:]
	}

	return path, nil
}

func (p *ProjectFs) fetchCollections(rootid string, root map[string]*FileNode, paths []string, private ...interface{}) {
	colls, err := Collections(rootid, p.projectid)
	if err == nil {
		for _, coll := range colls {
			node := &FileNode{
				Type:     Node_Dir,
				Name:     coll.Title,
				NodeId:   coll.Nodeid,
				ParentId: coll.ParentId,
				Updated:  coll.Updated,
				Child:    make(map[string]*FileNode),
				Private:  private,
			}
			if len(paths) > 0 && node.Name == paths[0] {
				p.fetchCollections(node.NodeId, node.Child, paths[1:], private)
			}
			root[coll.Title] = node
		}
	}
}

func (p *ProjectFs) fetchWorks(rootid string, root map[string]*FileNode, private ...interface{}) {
	works, err := Works(rootid, p.projectid)
	if err == nil {
		for _, work := range works {
			root[work.FileName] = &FileNode{
				Type:     Node_File,
				Name:     work.FileName,
				NodeId:   work.Nodeid,
				ParentId: work.ParentId,
				Url:      work.DownloadUrl,
				Size:     work.FileSize,
				Updated:  work.Updated,
				Private:  private,
			}
		}
	}
}

func (p *ProjectFs) find(path string) (node *FileNode, prefix string, exist bool, err error) {
	newpath, err := p.projectPath(path)
	if err != nil {
		return node, prefix, exist, err
	}

	defer func() {
		if err == nil && node != nil && node.Child == nil {
			node.Child = make(map[string]*FileNode)
		}
	}()

	tokens := strings.Split(newpath[1:], "/")
	node = p.root
	root := p.root.Child

	for idx, token := range tokens {
		if val, ok := root[token]; ok {
			node = val
			root = val.Child
			if idx == len(tokens)-1 {
				exist = true
				return node, "/" + strings.Join(tokens, "/"), exist, nil
			}
			continue
		}

		exist = false
		if idx == 0 {
			return node, "", exist, nil
		}

		return node, "/" + strings.Join(tokens[:idx], "/"), exist, nil
	}

	return
}

func (p *ProjectFs) mkdir(path string) (node *FileNode, err error) {
	newpath, err := p.projectPath(path)
	if err != nil {
		return node, err
	}

	// query path
	accnode, prefix, exist, err := p.find(newpath)
	if err != nil {
		return node, err
	}
	if exist {
		p.fetchWorks(accnode.NodeId, accnode.Child)
		return accnode, nil
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	// sync accnode
	tokens := strings.Split(newpath[len(prefix)+1:], "/")
	p.fetchCollections(accnode.NodeId, accnode.Child, tokens)

	// query again
	accnode, prefix, exist, err = p.find(newpath)
	if err != nil {
		return node, err
	}
	if exist {
		p.fetchWorks(accnode.NodeId, accnode.Child)
		return accnode, nil
	}

	// new path
	root := accnode
	tokens = strings.Split(newpath[len(prefix)+1:], "/")
	for _, token := range tokens {
		err = CreateCollection(root.NodeId, p.projectid, token)
		if err != nil {
			return node, err
		}
		list, err := Collections(root.NodeId, p.projectid)
		if err != nil {
			return node, err
		}

		if root.Child == nil {
			root.Child = make(map[string]*FileNode)
		}
		for _, coll := range list {
			if coll.Title == token {
				root.Child[coll.Title] = &FileNode{
					Type:     Node_Dir,
					Name:     token,
					ParentId: coll.ParentId,
					NodeId:   coll.Nodeid,
					Updated:  coll.Updated,
				}
				root = root.Child[coll.Title]
				break
			}
		}
	}

	return root, nil
}

func (p *ProjectFs) fileupload(path, target string) error {
	targetNode, err := p.mkdir(target)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	filenode, exist := targetNode.Child[info.Name()]
	if exist && filenode.Size == int(info.Size()) {
		return nil
	}
	if exist {
		err = Archive(filenode.NodeId)
		if err != nil {
			return err
		}
	}

	upload, err := UploadProjectFile(p.token, path)
	if err != nil {
		return err
	}

	return CreateWork(targetNode.NodeId, upload)
}

func (p *ProjectFs) Rename(before, after, dir string) error {
	node, _, exist, err := p.find(dir)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not exist")
	}

	works := make(map[string]*FileNode)
	p.fetchWorks(node.NodeId, works)
	if works[before] != nil {
		node = works[before]
		return RenameWork(node.NodeId, after)
	}

	collections := make(map[string]*FileNode)
	p.fetchCollections(node.NodeId, collections, nil)
	if collections[before] != nil {
		node = collections[before]
		return RenameCollection(node.NodeId, after)
	}

	return nil
}

func (p *ProjectFs) Move(src, dst string) error {
	srcnode, _, exist, err := p.find(src)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not found")
	}

	dstnode, err := p.mkdir(dst)
	if err != nil {
		return err
	}

	if srcnode.Type == Node_File {
		return MoveWork(srcnode.NodeId, dstnode.NodeId)
	}

	return MoveCollection(srcnode.NodeId, dstnode.NodeId)
}

func (p *ProjectFs) Copy(src, dst string) error {
	srcnode, _, exist, err := p.find(src)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not found")
	}

	dstnode, err := p.mkdir(dst)
	if err != nil {
		return err
	}

	c := Collection{
		Nodeid:     dstnode.NodeId,
		ParentId:   dstnode.ParentId,
		ProjectId:  p.projectid,
		ObjectType: Object_Collection,
	}

	if srcnode.Type == Node_File {
		return CopyWork(srcnode.NodeId, c)
	}

	return CopyCollection(srcnode.NodeId, c)
}

func (p *ProjectFs) Delete(path string) error {
	node, _, exist, err := p.find(path)
	if err != nil {
		return err
	}
	if !exist {
		return nil
	}

	if node.Type == Node_File {
		return DeleteWork(node.NodeId)
	}

	return DeleteCollection(node.NodeId)
}

func (p *ProjectFs) Upload(path string, target string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// file
	if !info.IsDir() {
		return p.fileupload(path, target)
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
				p.fileupload(f, target)
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

func (p *ProjectFs) Download(srcpath, targetdir string) error {
	newpath, err := p.projectPath(srcpath)
	if err != nil {
		return err
	}

	var tokens []string

	// query first
	accnode, prefix, exist, err := p.find(newpath)
	if err != nil {
		return err
	}
	if exist {
		goto download
	}

	// sync dirs
	p.mux.Lock()
	tokens = strings.Split(newpath[len(prefix)+1:], "/")
	p.fetchCollections(accnode.NodeId, accnode.Child, tokens)
	p.mux.Unlock()

	// query second
	accnode, prefix, exist, err = p.find(newpath)
	if err != nil {
		return err
	}
	if exist {
		goto download
	}

	// sync files
	p.mux.Lock()
	p.fetchWorks(accnode.NodeId, accnode.Child)
	p.mux.Unlock()

	// query again
	accnode, prefix, exist, err = p.find(newpath)
	if err != nil || !exist {
		if err == nil {
			err = errors.New("not exist path")
		}
		return err
	}
	goto download

download:
	if accnode.Type == Node_File {
		return util.File(accnode.Url, "GET", nil, nil, filepath.Join(targetdir, accnode.Name))
	}
	return ArchiveProjectDir(p.token, accnode.NodeId, p.projectid, accnode.Name, targetdir)
}
