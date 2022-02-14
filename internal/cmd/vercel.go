package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/urfave/cli"

	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/vercel"
)

func main() {
	var path, token string

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "path,p",
			Usage:       "source html path dir",
			Destination: &path,
		},
		cli.StringFlag{
			Name:        "token,t",
			Usage:       "vercel token value",
			Destination: &token,
		},
	}

	app.Action = func(c *cli.Context) error {
		if path == "" || token == "" {
			log.Errorln("path and token must be set")
			return fmt.Errorf("invalid params")
		}

		files, err := vercel.Files(path)
		if err != nil {
			log.Errorln("Files: %v", err)
			return err
		}

		var wg sync.WaitGroup
		batch := 10
		for i := 0; i < len(files); i += batch {
			count := batch
			if i+batch >= len(files) {
				count = len(files) - i
			}

			wg.Add(count)
			for k := i; k < count+i; k++ {
				go func(idx int) {
					defer wg.Done()
					vercel.Upload(&files[idx], token)
				}(k)
			}
			wg.Wait()
		}

		return vercel.Deploy(files, token)
	}

	app.Run(os.Args)
}
