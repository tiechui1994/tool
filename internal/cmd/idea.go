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
	)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "path,p",
			Usage:       "local jetbrains install dir",
			Value:       "/root",
			Destination: &path,
		},
	}

	app.Action = func(c *cli.Context) error {
		code := idea.GetCode2()
		if !idea.ValidCode(code) {
			code = idea.GetCode1()
		}
		if !idea.ValidCode(code) {
			log.Errorln("no code")
			os.Exit(1)
		}

		paths := idea.SearchFile(path)
		for _, path := range paths {
			idea.WriteCode(code, path)
		}

		return nil
	}

	app.Run(os.Args)
}
