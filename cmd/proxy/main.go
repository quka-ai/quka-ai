package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceInstance struct {
	Name string
	Addr string
}

type EtcdRegistry struct {
	cli *clientv3.Client
}

func NewEtcdRegistry(endpoints []string) (*EtcdRegistry, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdRegistry{cli: cli}, nil
}

func (r *EtcdRegistry) ListServiceInstances(service string) ([]ServiceInstance, error) {
	resp, err := r.cli.Get(context.Background(), "/services/"+service+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var res []ServiceInstance
	for _, kv := range resp.Kvs {
		var inst ServiceInstance
		if err := json.Unmarshal(kv.Value, &inst); err == nil {
			res = append(res, inst)
		}
	}
	return res, nil
}

func (r *EtcdRegistry) RegisterServiceInstance(service, addr string, ttl int64) error {
	key := fmt.Sprintf("/services/%s/%s", service, addr)
	val, _ := json.Marshal(ServiceInstance{Name: service, Addr: addr})
	lease, err := r.cli.Grant(context.Background(), ttl)
	if err != nil {
		return err
	}
	_, err = r.cli.Put(context.Background(), key, string(val), clientv3.WithLease(lease.ID))
	if err != nil {
		return err
	}
	// 保持租约
	ch, err := r.cli.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		return err
	}
	go func() {
		for range ch {
		}
	}()
	return nil
}

type ProxyServer struct {
	registry    *EtcdRegistry
	mu          sync.Mutex
	rr          map[string]int        // round robin index
	connections map[string][]net.Conn // 内网服务连接
	connMutex   sync.RWMutex
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Weight   int    `json:"weight"`
}

func NewProxyServer(reg *EtcdRegistry) *ProxyServer {
	return &ProxyServer{
		registry:    reg,
		rr:          make(map[string]int),
		connections: make(map[string][]net.Conn),
	}
}

func (p *ProxyServer) StartRegisterServer(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("启动注册服务器失败: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("接受连接失败: %v", err)
				continue
			}
			go p.handleRegistration(conn)
		}
	}()

	return nil
}

func (p *ProxyServer) handleRegistration(conn net.Conn) {
	// 读取注册请求
	var req RegisterRequest
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("解析注册请求失败: %v", err)
		conn.Close()
		return
	}

	// 验证请求
	if req.Name == "" || req.Protocol == "" || req.Port <= 0 {
		log.Printf("无效的注册请求: %+v", req)
		conn.Close()
		return
	}

	log.Printf("服务 %s 注册成功", req.Name)

	// 存储连接
	p.connMutex.Lock()
	if p.connections[req.Name] == nil {
		p.connections[req.Name] = make([]net.Conn, 0)
	}
	p.connections[req.Name] = append(p.connections[req.Name], conn)
	p.connMutex.Unlock()

	// 监听连接断开
	go func() {
		// 等待连接断开
		buf := make([]byte, 1)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				log.Printf("服务 %s 连接断开: %v", req.Name, err)
				p.removeConnection(req.Name, conn)
				break
			}
		}
	}()
}

func (p *ProxyServer) removeConnection(serviceName string, conn net.Conn) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	connections := p.connections[serviceName]
	for i, c := range connections {
		if c == conn {
			// 移除连接
			p.connections[serviceName] = append(connections[:i], connections[i+1:]...)
			conn.Close()
			break
		}
	}

	// 如果没有连接了，删除服务
	if len(p.connections[serviceName]) == 0 {
		delete(p.connections, serviceName)
		log.Printf("服务 %s 所有连接已断开", serviceName)
	}
}

func (p *ProxyServer) getServiceConnection(serviceName string) (net.Conn, error) {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	connections := p.connections[serviceName]
	if len(connections) == 0 {
		return nil, fmt.Errorf("服务 %s 没有可用连接", serviceName)
	}

	// 简单轮询负载均衡
	p.mu.Lock()
	idx := p.rr[serviceName]
	conn := connections[idx%len(connections)]
	p.rr[serviceName] = (idx + 1) % len(connections)
	p.mu.Unlock()

	return conn, nil
}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("quka-target")
	if target == "" {
		http.Error(w, "missing quka-target", 400)
		return
	}

	// 获取服务连接
	conn, err := p.getServiceConnection(target)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}

	// 转发请求到内网服务
	if err := r.Write(conn); err != nil {
		http.Error(w, "转发请求失败", 500)
		return
	}

	// 读取响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), r)
	if err != nil {
		http.Error(w, "读取响应失败", 500)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("复制响应体失败: %v", err)
	}
}

func main() {
	etcdEndpoints := flag.String("etcd", "localhost:2379", "etcd endpoints")
	listen := flag.String("listen", "0.0.0.0:3000", "listen addr")
	registerPort := flag.Int("register-port", 3001, "register server port")
	flag.Parse()

	reg, err := NewEtcdRegistry(strings.Split(*etcdEndpoints, ","))
	if err != nil {
		log.Fatal(err)
	}

	proxy := NewProxyServer(reg)

	// 启动注册服务器
	if err := proxy.StartRegisterServer(*registerPort); err != nil {
		log.Fatalf("启动注册服务器失败: %v", err)
	}
	log.Printf("注册服务器运行在端口 %d", *registerPort)

	log.Printf("代理服务器运行在 %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, proxy))
}
