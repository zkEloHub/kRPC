package client

import (
	"context"
	"fmt"
	"krpc/service"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("init listen error:", err)
		return
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	service.Accept(l)
}

func TestServerDay2(t *testing.T) {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)

	// default Opts
	// ... go client.receive() ...
	client, _ := Dial("tcp", <- addr)
	defer func() {_ = client.Close()}()

	time.Sleep(time.Second)

	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("kRPC req %d", i)
			var reply string
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error: ", err)
				return
			}
			log.Println("reply: ", reply)
		}(i)
	}
	wg.Wait()
}