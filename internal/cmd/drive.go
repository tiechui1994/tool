package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/drive"
	"github.com/tiechui1994/tool/log"
)

func main() {
	app := cli.NewApp()

	app.Action = func(c *cli.Context) error {
		uri, err := drive.BuildAuthorizeUri()
		if err != nil {
			return err
		}

	retry:
		fmt.Println("open browther with auth url and get code:", uri)
		var code string
		fmt.Printf("input code:")
		fmt.Scanf("%s", &code)
		if code == "" {
			fmt.Println("invalid code, retry again")
			goto retry
		}
		err = drive.Token(code)
		if err != nil {
			log.Errorln("Token: %v", err)
			goto retry
		}

		drive.Exec()

		return nil
	}

	app.Run(os.Args)
}
