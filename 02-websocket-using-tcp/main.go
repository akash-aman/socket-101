package main

import (
	"sync"
	tcp "websocket/tcp"
)

func main() {
	var sync sync.WaitGroup
	sync.Add(1)
	defer sync.Wait()
	go tcp.NewServer(&sync)
	//go tcp.NewClient(&sync)

}
