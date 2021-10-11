package main

import (
	"container/heap"
	"strings"
	"sync"

	"github.com/tiechui1994/tool/clash"
	"github.com/tiechui1994/tool/log"
)

type proxydelay struct {
	delay int
	proxy string
}

type pheap []proxydelay

func (p pheap) Len() int { return len(p) }

func (p pheap) Less(i, j int) bool {
	return p[i].delay > p[j].delay
}

func (p pheap) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (p *pheap) Push(x interface{}) {
	*p = append(*p, x.(proxydelay))
	for p.Len() > 5 {
		p.Pop()
	}
}

func (p *pheap) Pop() interface{} {
	old := *p
	n := len(old)
	x := old[n-1]
	*p = old[0 : n-1]
	return x
}

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
		if err == nil && delay < 400 {
			log.Infoln("provider: [%v] proxy: [%v] delay:%v", providername, curproxy.Now, delay)
			return
		}
	}

	// other
	provider, err := clash.Providers()
	if err != nil {
		return
	}
	var lock sync.Mutex
	var wg sync.WaitGroup

	array := &pheap{}
	heap.Init(array)

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
				array.Push(proxydelay{
					delay: delay,
					proxy: proxy,
				})
				lock.Unlock()
			}
		}(v.Name)
	}

	wg.Wait()

	if array.Len() > 0 {
		first := array.Pop().(proxydelay)
		if strings.Contains(first.proxy, "香港") {
			goto set
		}
		for array.Len() > 0 {
			second := array.Pop().(proxydelay)
			if strings.Contains(first.proxy, "香港") {
				first = second
				goto set
			}
		}
	set:
		clash.SetProxy(provider.Name, first.proxy)
		log.Infoln("provider: [%v] proxy: [%v] delay:%v", provider.Name, first.proxy, first.delay)
		return
	}

	log.Errorln("no adapter proxy")
}
