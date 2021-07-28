package aliyun

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/urfave/cli"

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
		collections, err := Collections(rootid, project.ProjectId)
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
	Type     int         `json:"type"`
	Name     string      `json:"name"`
	NodeId   string      `json:"nodeid"`
	ParentId string      `json:"parentid"`
	Updated  string      `json:"updated"`
	Created  string      `json:"created"`
	Child    []*FileNode `json:"child,omitempty"`
	Url      string      `json:"url,omitempty"`
	Size     int         `json:"size,omitempty"`
	Private  interface{} `json:"private,omitempty"`
}

func search(arr []*FileNode, name string) (*FileNode, bool) {
	for i := range arr {
		if arr[i].Name == name {
			return arr[i], true
		}
	}
	return nil, false
}

func (node *FileNode) Search(name string) (*FileNode, bool) {
	return search(node.Child, name)
}

type ProjectFs struct {
	Name  string
	Orgid string

	mux        sync.Mutex
	projectid  string
	rootcollid string
	token      string
	root       *FileNode
	pwd        string
}

func NewProject(name, orgid string) (*ProjectFs, error) {
	p := ProjectFs{Name: name, Orgid: orgid, pwd: "/"}

	list, err := Projects(p.Orgid)
	if err != nil {
		fmt.Println(err, p.Orgid)
		return nil, err
	}

	for _, item := range list {
		if item.Name == p.Name {
			p.projectid = item.ProjectId
			p.rootcollid = item.RootCollectionId
		}
	}

	if p.rootcollid == "" && p.projectid == "" {
		return nil, errors.New("invalid name")
	}

	p.token, err = GetToken(p.projectid, p.rootcollid)
	if err != nil {
		return nil, err
	}

	p.root = &FileNode{
		Type:   Node_Dir,
		Name:   "/",
		NodeId: p.rootcollid,
		Child:  make([]*FileNode, 0, 10),
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

func (p *ProjectFs) fetchCollections(rootid string, paths []string, private ...interface{}) (list []*FileNode, err error) {
	list = make([]*FileNode, 0, 10)
	colls, err := Collections(rootid, p.projectid)
	if err == nil {
		for _, coll := range colls {
			node := &FileNode{
				Type:     Node_Dir,
				Name:     coll.Title,
				NodeId:   coll.Nodeid,
				ParentId: coll.ParentId,
				Updated:  coll.Updated,
				Created:  coll.Created,
				Child:    make([]*FileNode, 0, 10),
				Private:  private,
			}
			if len(paths) > 0 && node.Name == paths[0] {
				node.Child, err = p.fetchCollections(node.NodeId, paths[1:], private)
				if err != nil {
					return list, err
				}
			}
			list = append(list, node)
		}
	}

	return list, err
}

func (p *ProjectFs) fetchWorks(rootid string, private ...interface{}) (list []*FileNode, err error) {
	list = make([]*FileNode, 0, 10)
	works, err := Works(rootid, p.projectid)
	if err == nil {
		for _, work := range works {
			node := &FileNode{
				Type:     Node_File,
				Name:     work.FileName,
				NodeId:   work.Nodeid,
				ParentId: work.ParentId,
				Url:      work.DownloadUrl,
				Size:     work.FileSize,
				Updated:  work.Updated,
				Created:  work.Created,
				Private:  private,
			}
			list = append(list, node)
		}
	}

	return list, err
}

func (p *ProjectFs) find(path string) (node *FileNode, prefix string, exist bool, err error) {
	newpath, err := p.projectPath(path)
	if err != nil {
		return node, prefix, exist, err
	}

	subpaths := strings.Split(newpath[1:], "/")

	node = p.root
	child := p.root.Child

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
		accnode.Child, err = p.fetchWorks(accnode.NodeId)
		return accnode, err
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	// sync accnode
	subpath := strings.Split(newpath[len(prefix)+1:], "/")
	accnode.Child, err = p.fetchCollections(accnode.NodeId, subpath)
	if err != nil {
		return node, err
	}

	// query again
	accnode, prefix, exist, err = p.find(newpath)
	if err != nil {
		return node, err
	}
	if exist {
		accnode.Child, err = p.fetchWorks(accnode.NodeId)
		return accnode, err
	}

	// new path
	parent := accnode
	subpath = strings.Split(newpath[len(prefix)+1:], "/")
	for _, token := range subpath {
		cnode, err := CreateCollection(parent.NodeId, p.projectid, token)
		if err != nil {
			return node, err
		}

		curnode := &FileNode{
			Type:     Node_Dir,
			Name:     token,
			ParentId: cnode.ParentId,
			NodeId:   cnode.Nodeid,
			Updated:  cnode.Updated,
			Created:  cnode.Created,
		}
		parent = curnode
	}

	return parent, nil
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

	filenode, exist := targetNode.Search(info.Name())
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

	_, err = CreateWork(targetNode.NodeId, upload)
	return err
}

func (p *ProjectFs) Rename(before, after, dir string) error {
	node, _, exist, err := p.find(dir)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not exist")
	}

	works, err := p.fetchWorks(node.NodeId)
	if err != nil {
		return err
	}
	if val, exist := search(works, before); exist {
		return RenameWork(val.NodeId, after)
	}

	collections, err := p.fetchCollections(node.NodeId, nil)
	if err != nil {
		return err
	}
	if val, exist := search(collections, before); exist {
		return RenameCollection(val.NodeId, after)
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
		ObjectType: OBJECT_COLLECTION,
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
	accnode.Child, err = p.fetchCollections(accnode.NodeId, tokens)
	if err != nil {
		p.mux.Unlock()
		return err
	}

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
	return ArchiveProject(p.token, accnode.NodeId, p.projectid, accnode.Name, targetdir)
}

func (p *ProjectFs) List(dir string, onlyfile, onlydir bool) error {
	node, _, exist, err := p.find(dir)
	if err != nil {
		return err
	}
	if !exist {
		return errors.New("not exist dir")
	}

	works := make([]*FileNode, 0, 0)
	dirs := make([]*FileNode, 0, 0)
	if !onlyfile && !onlydir {
		onlyfile, onlydir = true, true
	}

	if onlyfile {
		works, err = p.fetchWorks(node.NodeId)
	}

	if onlydir {
		dirs, err = p.fetchCollections(node.NodeId, nil)
	}

	list := make([]*FileNode, 0, len(works)+len(dir))
	for _, val := range works {
		list = append(list, val)
	}
	for _, val := range dirs {
		list = append(list, val)
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Updated < list[j].Updated
	})

	tpl := `
total {{(len .)}}
{{- range . }}
{{ $x := "f" }}
{{- if (eq .Type 1) -}} 
	{{- $x = "d" -}} 
{{- end -}}
{{printf "%s  %24s  %s" $x .Updated .Name}}
{{- end }}
`

	tpl = strings.Trim(tpl, "\n")
	temp, err := template.New("").Parse(tpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = temp.Execute(&buf, list)
	if err != nil {
		return err
	}

	fmt.Println(buf.String())
	return nil
}

func (p *ProjectFs) Cwd(dir string) error {
	path, err := filepath.Rel(p.pwd, dir)
	if err != nil {
		return err
	}

	p.pwd = path
	return nil
}

func (p *ProjectFs) Pwd() {
	fmt.Println(p.pwd)
}

func Exec() cli.Command {
	Setup()

	cmd := cli.Command{
		Name:        "project",
		Description: "aliyun teambition management",
	}

	cmd.Subcommands = []cli.Command{
		{
			Name:  "pwd",
			Usage: "current dir",
			Action: func(c *cli.Context) error {
				fmt.Println("pwd")
				return nil
			},
		},
		{
			Name:      "cd",
			Usage:     "change to dir",
			ArgsUsage: "[DIR]",
			Action: func(c *cli.Context) error {
				fmt.Println("cd")
				return nil
			},
		},
		{
			Name:      "ls",
			Aliases:   []string{"list"},
			Usage:     "list files or dirs in the dir",
			ArgsUsage: "[DIR]",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "d",
					Usage: "only include dirs",
				},
				cli.BoolFlag{
					Name:  "f",
					Usage: "only include files",
				},
				cli.BoolFlag{
					Name:  "a",
					Usage: "include files and dirs",
				},
			},
			Action: func(c *cli.Context) error {
				fmt.Println("ls", c.Bool("a"))

				for i := 0; i < c.NArg(); i++ {
					fmt.Println(c.Args().Get(i))
				}

				return nil
			},
		},
		{
			Name:      "upload",
			Usage:     "upload file to project",
			ArgsUsage: "[LOCAL] [REMOTE]",
			Action: func(c *cli.Context) error {
				return nil
			},
		},
		{
			Name:      "mv",
			Aliases:   []string{"move"},
			Usage:     "move file or dir to dir",
			ArgsUsage: "[SRC] [DST]",
			Action: func(c *cli.Context) error {
				return nil
			},
		},
		{
			Name:      "cp",
			Aliases:   []string{"copy"},
			Usage:     "copy dir or file",
			ArgsUsage: "[SRC] [DST]",
			Action: func(c *cli.Context) error {
				return nil
			},
		},
		{
			Name:      "rm",
			Aliases:   []string{"remove"},
			Usage:     "remove dir or file",
			ArgsUsage: "[NAME]",
			Action: func(c *cli.Context) error {
				return nil
			},
		},
		{
			Name:      "rename",
			Usage:     "rename dir or file",
			ArgsUsage: "[SRCNAME] [DSTNAME]",
			Action: func(c *cli.Context) error {
				return nil
			},
		},
	}

	return cmd
}

func Setup() *ProjectFs {
	AutoLogin()

	_, orgs, err := GetCacheData()
	if err != nil {
		fmt.Println("catch data err", err)
		os.Exit(1)
	}

	tpl := `
{{ range $orgidx, $ele := . }}
{{ printf "%d. org: %s(%s)" $orgidx  .Name .OrganizationId -}}
{{ range $pidx, $val := .Projects }}
  {{ printf "%d.%d project: %s(%s)" $orgidx $pidx .Name .ProjectId -}}
{{ end }}
{{ end }}
`

	tpl = strings.Trim(tpl, "\n")
	temp, err := template.New("").Parse(tpl)
	if err != nil {
		os.Exit(1)
	}

	var buf bytes.Buffer
	err = temp.Execute(&buf, orgs)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(buf.String())

	reindex := regexp.MustCompile(`^([0-9])\.([0-9])$`)
retry:
	var idx string
	fmt.Printf("Select project index:")
	fmt.Scanf("%s", &idx)
	if !reindex.MatchString(idx) {
		fmt.Println("input fortmat error. eg: 1.1")
		goto retry
	}
	tokens := reindex.FindAllStringSubmatch(idx, -1)
	if len(tokens) == 0 || len(tokens[0]) != 3 {
		fmt.Println("input fortmat error. eg: 1.1")
		goto retry
	}

	id1, _ := strconv.Atoi(tokens[0][1])
	id2, _ := strconv.Atoi(tokens[0][2])
	if len(orgs) < id1 || len(orgs[id1].Projects) < id2 {
		fmt.Println("input fortmat error. eg: 1.1")
		goto retry
	}

	orgid := orgs[id1].OrganizationId
	name := orgs[id1].Projects[id2].Name
	p, err := NewProject(name, orgid)
	if err != nil {
		fmt.Println("new project err", err)
		os.Exit(1)
	}

	return p
}
