package teambition

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/util"
)

type FileSystem interface {
	Init() error
	DownloadUrl(srcpath, targetdir string) error
	UploadFile(filepath, targetdir string) error
	UploadDir(filepath, targetdir string) error
}

const (
	Node_Dir  = 1
	Node_File = 2
)

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

func (p *ProjectFs) Init() (err error) {
	if p.Name == "" || p.Orgid == "" {
		return
	}

	list, err := Projects(p.Orgid)
	if err != nil {
		fmt.Println(err, p.Orgid)
		return err
	}

	for _, item := range list {
		if item.Name == p.Name {
			p.projectid = item.ID
			p.rootcollid = item.RootCollectionId
		}
	}

	if p.rootcollid == "" && p.projectid == "" {
		return errors.New("invalid name")
	}

	p.token, err = GetProjectToken(p.projectid, p.rootcollid)
	if err != nil {
		return
	}

	p.root = &FileNode{
		Type:   Node_Dir,
		Name:   "/",
		NodeId: p.rootcollid,
		Child:  make(map[string]*FileNode),
	}
	p.collections(p.root.NodeId, p.root.Child, nil)
	return nil
}

func (p *ProjectFs) fixpath(path string) string {
	path = strings.TrimSpace(path)
	if path[0] != '/' {
		return ""
	}

	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	if strings.HasPrefix(path, "/"+p.Name) {
		path = path[len(p.Name)+1:]
	}

	return path
}

func (p *ProjectFs) collections(rootid string, root map[string]*FileNode, tokens []string, private ...interface{}) {
	colls, err := Collections(rootid, p.projectid)
	if err == nil {
		for _, coll := range colls {
			node := &FileNode{
				Type:     Node_Dir,
				Name:     coll.Title,
				NodeId:   coll.ID,
				ParentId: coll.ParentId,
				Updated:  coll.Updated,
				Child:    make(map[string]*FileNode),
				Private:  private,
			}
			if len(tokens) > 0 && node.Name == tokens[0] {
				p.collections(node.NodeId, node.Child, tokens[1:], private)
			}
			root[coll.Title] = node
		}
	}
}

func (p *ProjectFs) works(rootid string, root map[string]*FileNode, private ...interface{}) {
	works, err := Works(rootid, p.projectid)
	if err == nil {
		for _, work := range works {
			root[work.FileName] = &FileNode{
				Type:     Node_File,
				Name:     work.FileName,
				NodeId:   work.ID,
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
	newpath := p.fixpath(path)
	if newpath == "" {
		return node, prefix, exist, errors.New("invalid path")
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
	newpath := p.fixpath(path)
	if newpath == "" {
		return node, errors.New("invalid path")
	}

	// query path
	accnode, prefix, exist, err := p.find(newpath)
	if err != nil {
		return node, err
	}
	if exist {
		p.works(accnode.NodeId, accnode.Child)
		return accnode, nil
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	// sync accnode
	tokens := strings.Split(newpath[len(prefix)+1:], "/")
	p.collections(accnode.NodeId, accnode.Child, tokens)

	// query again
	accnode, prefix, exist, err = p.find(newpath)
	if err != nil {
		return node, err
	}
	if exist {
		p.works(accnode.NodeId, accnode.Child)
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
					NodeId:   coll.ID,
					Updated:  coll.Updated,
				}
				root = root.Child[coll.Title]
				break
			}
		}
	}

	return root, nil
}

func (p *ProjectFs) UploadFile(filepath, targetdir string) error {
	if targetdir[0] != '/' {
		return errors.New("invalid dst")
	}

	info, err := os.Stat(filepath)
	if err != nil {
		return err
	}

	node, err := p.mkdir(targetdir)
	if err != nil {
		return err
	}

	fmt.Println("node", node)

	filenode, exist := node.Child[info.Name()]
	if exist && filenode.Size == int(info.Size()) {
		return nil
	}
	if exist {
		err = Archive(filenode.NodeId)
		if err != nil {
			return err
		}
	}

	upload, err := UploadProjectFile(p.token, filepath)
	if err != nil {
		return err
	}

	return CreateWork(node.NodeId, upload)
}
func (p *ProjectFs) UploadDir(srcdir, targetdir string) error {
	_, err := os.Stat(srcdir)
	if err != nil {
		return err
	}

	srcdir, _ = filepath.Abs(srcdir)
	dirpaths := make(map[string][]string)
	curdir := ""
	filepath.Walk(srcdir, func(path string, info os.FileInfo, err error) error {
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
		target := filepath.Join(targetdir, dir[len(srcdir):])
		for _, file := range files {
			count++
			wg.Add(1)
			f := file
			go func() {
				defer wg.Done()
				p.UploadFile(f, target)
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
func (p *ProjectFs) DownloadUrl(srcpath, targetdir string) (error) {
	newpath := p.fixpath(srcpath)
	if newpath == "" {
		return errors.New("invalid path")
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
	p.collections(accnode.NodeId, accnode.Child, tokens)
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
	p.works(accnode.NodeId, accnode.Child)
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

func FindPanDir(dir string) (node Node, err error) {
	dir = strings.TrimSpace(dir)
	if !strings.HasPrefix(dir, "/") {
		return node, errors.New("invalid path")
	}

	_, org, spaces, err := GetCacheData()
	if err != nil {
		return node, err
	}

	tokens := strings.Split(dir[1:], "/")
	parentid := spaces[0].RootId
	exist := false
	for i, p := range tokens {
		nodes, err := Nodes(org.OrganizationId, org.DriveId, parentid)
		if err != nil {
			return node, err
		}

		for _, n := range nodes {
			if n.Name == p {
				parentid = n.NodeId
				node = n
				exist = i == len(tokens)-1
			}
		}
	}

	if !exist {
		return node, errors.New("no exist path")
	}

	return node, nil
}

func PanMkdirP(dir string) (nodeid string, err error) {
	dir = strings.TrimSpace(dir)
	if !strings.HasPrefix(dir, "/") {
		return nodeid, errors.New("invalid path")
	}

	_, org, spaces, err := GetCacheData()
	if err != nil {
		return nodeid, err
	}

	isSearch := true
	tokens := strings.Split(dir[1:], "/")
	parentid := spaces[0].RootId
	for _, p := range tokens {
		if isSearch {
			nodes, err := Nodes(org.OrganizationId, org.DriveId, parentid)
			if err != nil {
				return nodeid, err
			}

			exist := false
			for _, n := range nodes {
				if n.Name == p {
					parentid = n.NodeId
					exist = true
					break
				}
			}

			if !exist {
				isSearch = false
				parentid, err = CreateFolder(p, org.OrganizationId, parentid, spaces[0].SpaceId, org.DriveId)
				if err != nil {
					return nodeid, err
				}
			}

			continue
		}

		parentid, err = CreateFolder(p, org.OrganizationId, parentid, spaces[0].SpaceId, org.DriveId)
		if err != nil {
			return nodeid, err
		}
	}

	return parentid, nil
}

func AutoLogin() {
	u, _ := url.Parse(www + "/")
	session := util.GetCookie(u, "TEAMBITION_SESSIONID")
	if session != nil {
		return
	}

	clientid, token, pubkey, err := LoginParams()
	if err != nil {
		return
	}

	remail := regexp.MustCompile(`^[A-Za-z0-9]+([_\\.][A-Za-z0-9]+)*@([A-Za-z0-9\-]+\.)+[A-Za-z]{2,6}$`)
	rphone := regexp.MustCompile(`^1[3-9]\\d{9}$`)
retry:
	var username string
	var password string
	fmt.Printf("Input Email/Phone:")
	fmt.Scanf("%s", &username)
	fmt.Printf("Input Password:")
	fmt.Scanf("%s", &password)

	if username == "" || password == "" {
		goto retry
	}

	if remail.MatchString(username) {
		_, err = Login(clientid, pubkey, token, username, "", password)
	} else if rphone.MatchString(username) {
		_, err = Login(clientid, pubkey, token, "", username, password)
	} else {
		goto retry
	}

	if err != nil {
		fmt.Println(err.Error())
		goto retry
	}

	fmt.Println("登录成功!!!")
}

func main() {
	AutoLogin()
	UserAgent = agents[0]

	_, _, _, err := GetCacheData()
	if err != nil {
		log.Println(err)
		return
	}

	p := ProjectFs{Name: "data", Orgid: "5f6707e0f0aab521364694ee"}
	fmt.Println(p.Init())
	fmt.Println(p.UploadDir("/home/user/go/src/golearn/http", "/wx"))

	//dir := "/packages"
	//log.Println("Making Dir", dir)
	//nodeid, err := PanMkdirP(dir)
	//if err != nil {
	//	log.Println("PanMkdirP", err)
	//	return
	//}
	//log.Println("Success")
	//
	//var files = []string{"/home/user/Downloads/gz/PgyVPN_Ubuntu_2.2.1_X86_64.deb"}
	//for _, file := range files {
	//	log.Println("Starting CreateFile:", file)
	//	files, err := CreatePanFile(org.OrganizationId, nodeid, spaces[0].SpaceId, org.DriveId, file)
	//	if err == nil {
	//		log.Println("Starting UploadUrl ...")
	//		upload, err := CreatePanUpload(org.OrganizationId, files[0])
	//		if err == nil {
	//			log.Println("Starting UploadPanFile ...")
	//			UploadPanFile(org.OrganizationId, upload, file)
	//			log.Println("Success")
	//		}
	//	}
	//
	//	time.Sleep(500 * time.Millisecond)
	//}
	//
	//time.Sleep(5 * time.Second)
}
