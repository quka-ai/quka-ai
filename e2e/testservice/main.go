package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceInstance struct {
	Name string
	Addr string
}

func main() {
	var (
		etcdAddr = flag.String("etcd", "localhost:2379", "etcd address")
		name     = flag.String("name", "service1", "service name")
		port     = flag.Int("port", 8080, "service port")
	)
	flag.Parse()

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{*etcdAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 注册到 etcd
	key := fmt.Sprintf("/services/%s/%s", *name, addr)
	val, _ := json.Marshal(ServiceInstance{Name: *name, Addr: addr})
	lease, err := cli.Grant(context.Background(), 10)
	if err != nil {
		log.Fatal(err)
	}
	_, err = cli.Put(context.Background(), key, string(val), clientv3.WithLease(lease.ID))
	if err != nil {
		log.Fatal(err)
	}
	ch, err := cli.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for range ch {
		}
	}()

	// 启动 HTTP 服务
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from %s on %s\n", *name, addr)
	})
	log.Printf("service %s listen on %s", *name, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
