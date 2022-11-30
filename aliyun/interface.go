package aliyun

type FileSystem interface {
	Rename(src, dst, dir string) error
	Move(src, dst string) error
	Copy(src, dst string) error
	Delete(path string) error
	Download(path, dstdir string) error
	Upload(path string, target string) error
	List(path string) ([]*FileNode, error)
}

type Type int32

const (
	Node_Dir  Type = 1
	Node_File Type = 2
)

var (
	types = map[Type]string{
		Node_Dir:  "Dir",
		Node_File: "File",
	}
)

func (t Type) String() string {
	return types[t]
}

type FileNode struct {
	Type     Type        `json:"type"`
	Name     string      `json:"name"`
	NodeId   string      `json:"nodeid"`
	ParentId string      `json:"parentid"`
	Updated  string      `json:"updated"`
	Created  string      `json:"created"`
	Child    []*FileNode `json:"child,omitempty"`
	Url      string      `json:"url,omitempty"`
	Size     int         `json:"size,omitempty"`
	Hash     string      `json:"hash,omitempty"`
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
