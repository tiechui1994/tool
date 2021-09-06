package main

import (
	"os"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/idea"
	"github.com/tiechui1994/tool/log"
)

func main() {
	var (
		path string
		code1, code2 bool
	)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "path,p",
			Usage:       "local jetbrains install dir",
			Value:       "/root",
			Destination: &path,
		},
		cli.BoolFlag{
			Name:        "1",
			Usage:       "code 1 website(idea.94goo.com)",
			Destination: &code1,
		},
		cli.BoolFlag{
			Name:        "2",
			Usage:       "code 2 website(vrg123.com)",
			Destination: &code2,
		},
	}

	app.Action = func(c *cli.Context) error {
		if !code1 && !code2 {
			code1  = true
		}

		var code string
		if code1 {
			code = idea.GetCode1()
			if !idea.ValidCode(code) {
				var username, password string
				code, username, password = idea.GetCode2()
				log.Infoln("username:%v, password:%v, you can try", username, password)
			}
			if !idea.ValidCode(code) {
				log.Errorln("no code")
				os.Exit(1)
			}
		} else {
			code, username, password := idea.GetCode2()
			if !idea.ValidCode(code) {
				log.Infoln("username:%v, password:%v, you can try", username, password)
				code = idea.GetCode1()
			}
			if !idea.ValidCode(code) {
				log.Errorln("no code")
				os.Exit(1)
			}
		}

		paths := idea.SearchFile(path)
		for _, path := range paths {
			idea.WriteCode(code, path)
		}

		return nil
	}

	app.Run(os.Args)
}
