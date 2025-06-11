package proxy_pool

import (
	"bufio"
	"os"
	"sync/atomic"
)

var index = atomic.Int32{}
var availableProxies []string

func readProxies(f *os.File) []string {
	scanner := bufio.NewScanner(f)
	var proxies []string
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			panic(err)
		}
		proxy := scanner.Text()
		if len(proxy) < 5 {
			continue
		}
		proxies = append(proxies, proxy)
	}
	return proxies
}

func GetProxy() string {
	if len(availableProxies) == 0 {
		return ""
	}
	i := int(index.Add(1) % int32(len(availableProxies)))
	return availableProxies[i]
}

func GetAllProxies() []string {
	return availableProxies
}

func init() {
	f, err := os.Open("proxies.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	} else {
		defer f.Close()
		availableProxies = readProxies(f)
	}
}
