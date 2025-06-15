package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	var (
		proxyAddr = flag.String("proxy", "127.0.0.1:3000", "proxy address")
		target    = flag.String("target", "service1", "quka-target header")
		path      = flag.String("path", "/hello", "request path")
	)
	flag.Parse()

	url := fmt.Sprintf("http://%s%s", *proxyAddr, *path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("quka-target", *target)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
}
