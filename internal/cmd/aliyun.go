package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/aliyun"
	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

func init() {
	util.SetLogPrefix()
	util.RegisterLocalJar()
}

func GetLocalToken() (token aliyun.Token, err error) {
	key := filepath.Join(util.ConfDir(), "drive.json")
	if util.ReadFile(key, &token) == nil {
		return token, nil
	}

	retry := 0
try:
	raw, err := util.GET("https://jobs.tiechui1994.tk/api/aliyun?response_type=refresh_token&key=yunpan", nil)
	if err != nil && retry < 4 {
		log.Errorln("err:%v, retry again", err)
		retry += 1
		goto try
	}

	if err != nil {
		log.Errorln("err:%v", err)
		return token, err
	}

	err = json.Unmarshal(raw, &token)
	if err != nil {
		log.Errorln("decode: %v", err)
		return token, err
	}
	util.WriteFile(key, token)
	return token, nil
}

func Startup() *aliyun.DriveFs {
	token, err := GetLocalToken()
	if err != nil {
		os.Exit(1)
	}
	return aliyun.NewDriveFs(token)
}

func Drive() []cli.Command {
	drive := Startup()

	return []cli.Command{
		{
			Name:      "upload",
			Aliases:   []string{"up"},
			Usage:     "upload file to project",
			ArgsUsage: "[LOCAL] [REMOTE]",
			Action: func(c *cli.Context) error {
				args := []string(c.Args())
				if len(args) != 2 {
					cli.ShowCommandHelp(c, "upload")
					os.Exit(1)
				}
				if _, err := os.Stat(args[0]); os.IsNotExist(err) {
					cli.ShowCommandHelp(c, "upload")
					return errors.New("invalid [LOCAL]")
				}

				if !strings.HasPrefix(args[1], "/") {
					cli.ShowCommandHelp(c, "upload")
					return errors.New("invalid [REMOTE]")
				}
				return drive.Upload(args[0], args[1])
			},
		},
		{
			Name:      "mv",
			Aliases:   []string{"move"},
			Usage:     "move file or dir to dir",
			ArgsUsage: "[SRC] [DST]",
			Action: func(c *cli.Context) error {
				args := []string(c.Args())
				if len(args) != 2 {
					cli.ShowCommandHelp(c, "mv")
					os.Exit(1)
				}
				if !strings.HasPrefix(args[0], "/") {
					cli.ShowCommandHelp(c, "mv")
					return errors.New("invalid [SRC]")
				}

				if !strings.HasPrefix(args[1], "/") {
					cli.ShowCommandHelp(c, "mv")
					return errors.New("invalid [DST]")
				}
				return drive.Move(args[0], args[1])
			},
		},
		{
			Name:      "rm",
			Aliases:   []string{"remove", "del", "delete"},
			Usage:     "remove dir or file",
			ArgsUsage: "[NAME]",
			Action: func(c *cli.Context) error {
				args := []string(c.Args())
				if len(args) != 1 {
					cli.ShowCommandHelp(c, "rm")
					os.Exit(1)
				}
				if !strings.HasPrefix(args[0], "/") {
					cli.ShowCommandHelp(c, "rm")
					return errors.New("invalid [SRC]")
				}
				return drive.Delete(args[0])
			},
		},
		{
			Name:      "rename",
			Usage:     "rename dir or file",
			ArgsUsage: "[SRCNAME] [DSTNAME] [BASE]",
			Action: func(c *cli.Context) error {
				args := []string(c.Args())
				if len(args) != 3 {
					cli.ShowCommandHelp(c, "rename")
					os.Exit(1)
				}
				if !strings.HasPrefix(args[2], "/") {
					cli.ShowCommandHelp(c, "rename")
					return errors.New("invalid [BASE]")
				}
				return drive.Rename(args[0], args[1], args[2])
			},
		},
		{
			Name:      "download",
			Aliases:   []string{"down"},
			Usage:     "download dir or file",
			ArgsUsage: "[SRCPATH] [TARGETDIR]",
			Action: func(c *cli.Context) error {
				args := []string(c.Args())
				if len(args) != 2 {
					cli.ShowCommandHelp(c, "download")
					os.Exit(1)
				}
				if !strings.HasPrefix(args[0], "/") {
					cli.ShowCommandHelp(c, "rename")
					return errors.New("invalid [SRCPATH]")
				}
				return drive.Download(args[0], args[1])
			},
		},
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "drive"
	app.Description = "aliyun drive management"
	app.ExitErrHandler = func(context *cli.Context, err error) {
		if err != nil {
			fmt.Println(err)
		}
	}
	app.Commands = Drive()
	app.Run(os.Args)
}
