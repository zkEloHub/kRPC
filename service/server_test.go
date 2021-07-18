package service

import (
	"context"
	client2 "krpc/client"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

//type Foo int
//type Args struct {Num1, Num2 int}

//func (f Foo) Sum(args Args, reply *int) error {
//	*reply = args.Num2 + args.Num1
//	return nil
//}

func startServer(addr chan string) {
	var foo Foo
	if err := DefaultServer.Register(&foo); err != nil {
		log.Fatal("register error: ", err)
	}
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("init listen error:", err)
		return
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	Accept(l)
}

func TestServeDay3(t *testing.T) {
	log.SetFlags(0)
	addr := make(chan string)

	// init a server first.
	go startServer(addr)

	// kRPC client.
	client, _ := client2.Dial("tcp", <- addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)

	// send request to server & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num2: i, Num1: i * i}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error: ", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}


func TestServeDay4(t *testing.T) {
	log.SetFlags(0)
	addr := make(chan string)

	// init a server first.
	go startServer(addr)

	// kRPC client.
	client, _ := client2.Dial("tcp", <- addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)

	// send request to server & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			// Call timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second * 5)
			defer wg.Done()
			args := &Args{Num2: i, Num1: i * i}
			var reply int
			if err := client.Call(ctx, "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error: ", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}