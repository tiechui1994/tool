package main

import (
	"os"
	"sync"
	"time"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/gitee"
	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

func main() {
	var (
		project cli.StringSlice
		sleep   int64
		cookie  string
	)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:     "project,p",
			Usage:    "repo name",
			Required: true,
			Value:    &project,
		},
		cli.Int64Flag{
			Name:        "sleep,s",
			Usage:       "interval sleep(s)",
			Value:       3,
			Destination: &sleep,
		},
		cli.StringFlag{
			Name:        "cookie,c",
			Usage:       "cookie info",
			Required:    true,
			Destination: &cookie,
		},
	}

	app.Action = func(c *cli.Context) error {
		if len(project) == 0 {
			log.Errorln("未设置的project")
			os.Exit(1)
		}
		if sleep > 10 {
			sleep = 10
		}

		gitee.InitParams(cookie, time.Duration(sleep)*time.Second)

		err := gitee.CsrfToken()
		if err != nil {
			log.Errorln("CsrfToken获取失败")
			return err
		}

		util.SyncCookieJar()

		err = gitee.Resources()
		if err != nil {
			log.Errorln("cookie内容不合法")
			return err
		}

		var wg sync.WaitGroup
		wg.Add(len(project))
		for _, p := range project {
			go func(project string) {
				defer wg.Done()
				err = gitee.ForceSync(project)
				if err != nil {
					log.Errorln("[%v] 同步失败", project)
					return
				}
			}(p)
		}
		wg.Wait()

		gitee.Mark()
		util.SyncCookieJar()
		return nil
	}

	app.Run(os.Args)
}
