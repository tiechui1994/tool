package main

import (
	"github.com/urfave/cli"
	"os"

	"github.com/tiechui1994/tool/aliyun"
	"github.com/tiechui1994/tool/util"
)

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
			err := util.Deamon(args)
			if err != nil {
				return err
			}
		}

		return nil
	}

	app.Commands = append(app.Commands, aliyun.Exec())

	app.Run(os.Args)
}

func main() {
	App()
}
