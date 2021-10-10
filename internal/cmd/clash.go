package main

import (
	"github.com/tiechui1994/tool/clash"
	"github.com/tiechui1994/tool/log"
	"sync"
)

func main() {
	providername := clash.DefaultProvider()

	var curproxy clash.Proxy
	proxyies, err := clash.Proxys()
	if err != nil {
		return
	}
	for _, v := range proxyies {
		if v.Name == providername {
			curproxy = v
			break
		}
	}

	// find and speedtest ok
	if curproxy.Name != "" {
		delay, err := clash.SpeedTest(curproxy.Now)
		if err == nil && delay < 500 {
			log.Infoln("provider: [%v] proxy: [%v] delay:%v", providername, curproxy.Now, delay)
			return
		}
	}

	// other
	provider, err := clash.Providers()
	if err != nil {
		return
	}
	minDelay := 50000
	proxyname := ""
	var lock sync.Mutex
	var wg sync.WaitGroup

	for _, v := range provider.Proxies {
		if v.Type == "Direct" {
			continue
		}

		wg.Add(1)
		go func(proxy string) {
			defer wg.Done()
			delay, err := clash.SpeedTest(proxy)
			if err == nil {
				lock.Lock()
				if minDelay > delay {
					minDelay = delay
					proxyname = proxy
				}
				lock.Unlock()
			}
		}(v.Name)
	}

	wg.Wait()
	clash.SetProxy(provider.Name, proxyname)
	log.Infoln("provider: [%v] proxy: [%v] delay:%v", provider.Name, proxyname, minDelay)
}
