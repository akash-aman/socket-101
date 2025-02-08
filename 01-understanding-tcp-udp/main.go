package main

import (
	"sync"
	"transport/tcp"
	//"transport/udp"
)

func main() {
	var sync sync.WaitGroup
	sync.Add(2)
	defer sync.Wait()
	go tcp.Server(&sync)
	go tcp.Client(&sync)
	// go udp.Server(&sync)
	// go udp.Client(&sync)

}
