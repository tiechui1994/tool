package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/tiechui1994/tool/aliyun"
	"os"
	"os/exec"
	"time"

	"github.com/pborman/uuid"
	"github.com/tiechui1994/tool/log"
)

func Exec() {
	for {
		fmt.Println(log.Log(log.INFO, "%v %v", uuid.New(), time.Now().Unix()))
	}
}

func Main() {
	d := flag.Bool("d", false, "run app as a daemon with -d=true.")
	flag.Parse()

	if *d {
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

func main() {
	md5.New()

	aliyun.CalProof("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiJhOGZiMjM5Mjc4ZDg0NmZlOWExYmIzMGI2N2JiYjVmZCIsImN1c3RvbUpzb24iOiJ7XCJjbGllbnRJZFwiOlwiMjVkelgzdmJZcWt0Vnh5WFwiLFwiZG9tYWluSWRcIjpcImJqMjlcIixcInNjb3BlXCI6W1wiRFJJVkUuQUxMXCIsXCJTSEFSRS5BTExcIixcIkZJTEUuQUxMXCIsXCJVU0VSLkFMTFwiLFwiU1RPUkFHRS5BTExcIixcIlNUT1JBR0VGSUxFLkxJU1RcIixcIkJBVENIXCIsXCJPQVVUSC5BTExcIixcIklNQUdFLkFMTFwiLFwiSU5WSVRFLkFMTFwiLFwiQUNDT1VOVC5BTExcIl0sXCJyb2xlXCI6XCJ1c2VyXCIsXCJyZWZcIjpcImh0dHBzOi8vd3d3LmFsaXl1bmRyaXZlLmNvbS9cIn0iLCJleHAiOjE2MjcxMzk1NzEsImlhdCI6MTYyNzEzMjMxMX0.uPF3LPTUYBsI5BoaOaW1USfpM-ndnFspL-5l-gFVmhdSC6tPC8Jj1NDF1AKj39Q-NbxdOBD2_gG6NYIQQVIEz7k7_vMS15BJsv6txZXxWe_1WUldPh6lYNSkTJvJEdprq61A93ig-sNhVC81yD2Di6azyx3MWQ_pXEufDfI7nB4", "/home/quinn/Desktop/charles.tar.gz")

	data := make([]byte, 8)
	fd, _ := os.Open("/home/quinn/Desktop/charles.tar.gz")

	fd.ReadAt(data, 29928710)

	fmt.Println(hex.EncodeToString(data))
	fmt.Println(base64.StdEncoding.EncodeToString(data))
}
