package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/pborman/uuid"
	"github.com/tiechui1994/tool/log"
)

func Exec() {
	for {
		log.Log("%v %v", uuid.New(), time.Now().Unix())
		time.Sleep(2*time.Second)
	}
}

func main() {
	goDaemon := flag.Bool("d", false, "run app as a daemon with -d=true.")
	flag.Parse()

	if *goDaemon {
		cmd := exec.Command(os.Args[0], flag.Args()...)
		if err := cmd.Start(); err != nil {
			fmt.Printf("start %s failed, error: %v\n", os.Args[0], err)
			os.Exit(1)
		}
		fmt.Printf("%s [PID] %d running...\n", os.Args[0], cmd.Process.Pid)
		os.Exit(0)
	}

	Exec()
}
