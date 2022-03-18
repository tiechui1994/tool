package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"

	"github.com/tiechui1994/tool/cloudflare"
	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
)

type dns struct {
	Domain string
	Ngrok  string
}

type config struct {
	Email  string
	Key    string
	Zoneid string
	Userid string
	DNS    []dns
}

func configUsage() {
	c := config{
		Email:  "cloudflare account email",
		Key:    "cloudflare api key",
		Zoneid: "cloudflare zoneid",
		Userid: "cloudflare userid",
		DNS: []dns{
			{
				Domain: "cloudflare dns domain",
				Ngrok:  "ngrox http tunnel tag name",
			},
		},
	}

	var buf bytes.Buffer
	encode := yaml.NewEncoder(&buf)
	encode.Encode(c)
	temp := `yaml config file:

%s`
	fmt.Printf(temp, buf.String())
}

func uploadNgrokLog(lg string) (links map[string]string) {
	links = make(map[string]string)
	defer func() {
		if len(links) > 0 {
			var buf bytes.Buffer
			en := json.NewEncoder(&buf)
			en.SetIndent("", " ")
			en.Encode(links)
			fmt.Println("data: ", buf.String())
			u := "https://jobs.tiechui1994.tk/api/mongo?key=ngrok"
			body := map[string]interface{}{"ttl": 28800, "value": links}
			util.POST(u, body, nil)
		}
	}()

	handle := func(raw string) {
		stu := make(map[string]interface{})
		json.Unmarshal([]byte(raw), &stu)
		if stu["name"] != nil && stu["obj"] == "tunnels" {
			if stu["name"] == "ssh" {
				re := regexp.MustCompile(`tcp://([^:]+?):([0-9]+)`)
				tokens := re.FindAllStringSubmatch(stu["url"].(string), 1)
				links["ssh"] = fmt.Sprintf("ssh root@%s -p %s", tokens[0][1], tokens[0][2])
			} else {
				links[stu["name"].(string)] = stu["url"].(string)
			}
		}
	}

	fifo, err := os.Open(lg)
	if err != nil {
		log.Errorln("invalid ngrok log file path")
		return links
	}

	buf := make([]byte, 8192)
	for {
		n, err := fifo.Read(buf)
		if err != nil {
			if err == io.EOF {
				return links
			}
			log.Errorln("ngrok read failed: %v", err)
			return links
		}

		var temp []byte
		begin, end := 0, n
		nums := bytes.Count(buf, []byte("}"))
		last := buf[n-1] == '}'
		for i := 0; i < nums; i++ {
			length := bytes.IndexByte(buf[begin:end], '}')
			if i == 0 && buf[0] != '{' {
				handle(string(append(temp, buf[begin:begin+length+1]...)))
				temp = nil
				begin += length + 1
				continue
			}

			handle(string(buf[begin : begin+length+1]))
			begin += length + 1
		}

		if !last {
			temp = buf[begin:end]
		}
	}
}

func uploadCpolarLog(lg string) (links map[string]string) {
	links = make(map[string]string)
	defer func() {
		if len(links) > 0 {
			var buf bytes.Buffer
			en := json.NewEncoder(&buf)
			en.SetIndent("", " ")
			en.Encode(links)
			fmt.Println("data: ", buf.String())
			u := "https://jobs.tiechui1994.tk/api/mongo?key=cpolar"
			body := map[string]interface{}{"ttl": 28800, "value": links}
			util.POST(u, body, nil)
		}
	}()

	re := regexp.MustCompile(`{("Type":"NewTunnel".*)}`)
	handle := func(raw string) {
		raw = strings.ReplaceAll(raw, "\\", "")
		tokens := re.FindAllString(raw, 1)

		if len(tokens) == 1 {
			var stu struct {
				Type    string
				Payload struct {
					TunnelName string
					Url        string
					LocalAddr  string
				}
			}
			json.Unmarshal([]byte(tokens[0]), &stu)
			if stu.Payload.TunnelName == "ssh" {
				re := regexp.MustCompile(`tcp://([^:]+?):([0-9]+)`)
				tokens := re.FindAllStringSubmatch(stu.Payload.Url, 1)
				links["ssh"] = fmt.Sprintf("ssh root@%s -p %s", tokens[0][1], tokens[0][2])
			} else {
				links[stu.Payload.TunnelName] = stu.Payload.Url
			}
		}
	}

	newfile := "/tmp/cpolar.log"
	cmd := fmt.Sprintf("grep -E 'NewTunnel' %v > %v", lg, newfile)
	err := exec.Command("bash", "-c", cmd).Run()
	if err != nil {
		log.Errorln("cmd(%v) failed: %v", cmd, err)
		return links
	}

	fifo, err := os.Open(newfile)
	if err != nil {
		log.Errorln("invalid cpolar log file path")
		return links
	}

	reader := bufio.NewReader(fifo)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return links
			}

			log.Errorln("cpolar ReadString: %v", err)
			return links
		}

		handle(str)
	}
}

func update(cl cloudflare.Cloudflare, links map[string]string, cfg config) error {
	rules, err := cl.PageRulesList()
	if err != nil {
		fmt.Println("Get PageRule List Failed:", err)
		return err
	}

	for _, dns := range cfg.DNS {
		exist := false
		for _, rule := range rules {
			if len(rule.Targets) == 0 || len(rule.Actions) == 0 {
				continue
			}

			if strings.HasPrefix(rule.Targets[0].Constraint.Value, dns.Domain) {
				exist = true
				if rule.Actions[0].ID == "forwarding_url" {
					val := rule.Actions[0].Value.(map[string]interface{})
					val["status_code"] = 301
					val["url"] = links[dns.Ngrok] + "/$1"
					rule.Actions[0].Value = val
				}

				err = cl.PageRulesUpdate(rule)
			}
		}

		if !exist {
			rule := cloudflare.PageRule{
				Targets: []cloudflare.Target{
					{
						Target: "url",
					},
				},
				Actions: []cloudflare.Action{
					{
						ID: "forwarding_url",
						Value: cloudflare.ActionRedirect{
							Url:        links[dns.Ngrok] + "/$1",
							StatusCode: 301,
						},
					},
				},
			}
			rule.Targets[0].Constraint.Operator = "matches"
			rule.Targets[0].Constraint.Value = dns.Ngrok + "/*"
			err = cl.PageRulesCreate(rule)
		}

		if err != nil {
			fmt.Println("Update PageRule Failed:", err)
			return err
		}
	}

	return nil
}

func main() {
	var (
		cfgfile, lg   string
		ngrok, cpolar string
		wait          int
	)
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "log,l",
			Usage:       "log file",
			Destination: &lg,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:  "start",
			Usage: "start ngrok worker",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "config",
					Usage:       "cloudflare yaml config file",
					Required:    true,
					Destination: &cfgfile,
				},
				cli.StringFlag{
					Name:        "ngrok,n",
					Usage:       "type ngrok log file",
					Destination: &ngrok,
				},
				cli.StringFlag{
					Name:        "cpolar,c",
					Usage:       "type cpolar log file",
					Destination: &cpolar,
				},
				cli.IntFlag{
					Name:        "wait,w",
					Usage:       "wait seconds before upload",
					Destination: &wait,
				},
			},
			Action: func(c *cli.Context) error {
				if len(lg) > 0 {
					os.Stdout, _ = os.Create(lg)
					log.SetOutput(os.Stdout)
				}

				var (
					cfg   config
					links map[string]string
				)
				data, err := ioutil.ReadFile(cfgfile)
				if err != nil {
					fmt.Println("param --config=xxx must be set")
					return err
				}

				err = yaml.Unmarshal(data, &cfg)
				if err != nil {
					configUsage()
					return err
				}

				if cfg.Userid == "" || cfg.Key == "" || cfg.Email == "" || cfg.Zoneid == "" {
					configUsage()
					return err
				}

				if cpolar == "" && ngrok == "" {
					fmt.Println("param --ngrok=FILE or --cpolar=FILE must be set")
					return fmt.Errorf("ngrok or cpolar")
				}

				cl := cloudflare.Cloudflare{
					AuthEmail: cfg.Email,
					AuthKey:   cfg.Key,
					ZoneID:    cfg.Zoneid,
					UserID:    cfg.Userid,
				}

				time.Sleep(time.Duration(wait) * time.Second)
			again:
				if len(cpolar) > 0 {
					links = uploadCpolarLog(cpolar)
				} else if len(ngrok) > 0 {
					links = uploadNgrokLog(ngrok)
				}
				if len(links) > 0 {
					update(cl, links, cfg)
				}

				time.Sleep(30 * time.Minute)
				goto again
			},
		},
		{
			Name:    "kill",
			Aliases: []string{"k"},
			Usage:   "kill ngrok worker",
			Action: func(c *cli.Context) error {
				if len(lg) > 0 {
					os.Stdout, _ = os.Create(lg)
				}
				return nil
			},
		},
	}

	app.Run(os.Args)
}
