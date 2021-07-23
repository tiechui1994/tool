package teambition

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/tiechui1994/tool/util"
)

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

	_, _, _, err := GetCacheData()
	if err != nil {
		log.Println(err)
		return
	}

	p, err := NewProject("data", "5f6707e0f0aab521364694ee")
	if err != nil {
		log.Println(err)
		return
	}

	_ = p

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
