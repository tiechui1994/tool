package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/aliyun"
	"github.com/tiechui1994/tool/util"
)

func init() {
	util.RegisterLocalJar()
}

func GetCacheData() (roles []aliyun.Role, orgs []aliyun.Org, err error) {
	var result struct {
		Roles []aliyun.Role
		Org   []aliyun.Org
	}

	key := filepath.Join(util.ConfDir(), "teambition.json")
	if util.ReadFile(key, &result) == nil {
		return result.Roles, result.Org, nil
	}

	roles, err = aliyun.Roles()
	if err != nil {
		log.Println(err)
		return
	}

	if len(roles) == 0 {
		err = errors.New("no roles")
		return
	}

	var (
		lock sync.Mutex
		wg   sync.WaitGroup
	)
	for _, role := range roles {
		if role.Level == 0 {
			continue
		}
		wg.Add(1)
		orgid := role.OrganizationId
		go func(orgid string) {
			defer wg.Done()
			org, err := aliyun.Orgs(orgid)
			if err != nil {
				log.Println("fetch org err:", orgid, err)
				return
			}
			org.Projects, err = aliyun.Projects(orgid)
			if err != nil {
				log.Println("fetch project err:", orgid, err)
				return
			}

			lock.Lock()
			orgs = append(orgs, org)
			lock.Unlock()
		}(orgid)
	}

	wg.Wait()

	result.Roles = roles
	result.Org = orgs
	util.WriteFile(key, result)
	return
}

func AutoLogin() {
	u, _ := url.Parse("https://www.teambition.com/")
	cookie := util.GetCookie(u, "TEAMBITION_SESSIONID")
	if cookie != nil {
		return
	}

	clientid, token, pubkey, err := aliyun.LoginParams()
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
		_, err = aliyun.Login(clientid, pubkey, token, username, "", password)
	} else if rphone.MatchString(username) {
		_, err = aliyun.Login(clientid, pubkey, token, "", username, password)
	} else {
		goto retry
	}

	if err != nil {
		fmt.Println(err.Error())
		goto retry
	}

	fmt.Println("登录成功!!!")
	util.SyncCookieJar()
}

func Setup() *aliyun.ProjectFs {
	AutoLogin()

	_, orgs, err := GetCacheData()
	if err != nil {
		fmt.Println("catch data err", err)
		os.Exit(1)
	}

	tpl := `
{{ range $orgidx, $ele := . }}
{{ printf "%d org: %s(%s)" $orgidx  .Name .OrganizationId -}}
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
		fmt.Println("input fortmat error. eg: 0.1")
		goto retry
	}
	tokens := reindex.FindAllStringSubmatch(idx, -1)
	if len(tokens) == 0 || len(tokens[0]) != 3 {
		fmt.Println("input fortmat error. eg: 0.1")
		goto retry
	}

	id1, _ := strconv.Atoi(tokens[0][1])
	id2, _ := strconv.Atoi(tokens[0][2])
	if !(len(orgs) > id1 && len(orgs[id1].Projects) > id2) {
		fmt.Println("input fortmat error. eg: 0.1")
		goto retry
	}

	orgid := orgs[id1].OrganizationId
	name := orgs[id1].Projects[id2].Name
	p, err := aliyun.NewProject(name, orgid)
	if err != nil {
		fmt.Println("new project err:", err)
		os.Exit(1)
	}

	return p
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

func App() {
	var daemon bool
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "daemon,d",
			Usage:       "run as daemon",
			Destination: &daemon,
		},
	}

	app.Action = func(c *cli.Context) error {
		if daemon {
			var args []string
			for _, arg := range os.Args {
				if arg != "-d" && arg != "--daemon" {
					args = append(args, arg)
				}
			}
			err := util.Deamon1(args)
			if err != nil {
				return err
			}
		}

		return nil
	}

	app.Commands = append(app.Commands, Exec())

	app.Run(os.Args)
}

func main() {
	App()
}
