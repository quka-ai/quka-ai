package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type RegisterRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Weight   int    `json:"weight"`
}

type LocalService struct {
	name         string
	port         int
	proxyAddr    string
	registerConn net.Conn
	upgrader     websocket.Upgrader
	router       *gin.Engine
}

func NewLocalService(name string, port int, proxyAddr string) *LocalService {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	service := &LocalService{
		name:      name,
		port:      port,
		proxyAddr: proxyAddr,
		router:    gin.New(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	// 设置中间件
	service.router.Use(gin.Logger())
	service.router.Use(gin.Recovery())
	service.router.Use(service.corsMiddleware())

	// 设置路由
	service.setupRoutes()

	return service
}

func (s *LocalService) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, quka-target")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *LocalService) setupRoutes() {
	// API 路由组
	api := s.router.Group("/api")
	{
		api.GET("/hello", s.handleHello)
		api.GET("/info", s.handleInfo)
		api.GET("/data", s.handleData)
		api.POST("/data", s.handleCreateData)
		api.PUT("/data/:id", s.handleUpdateData)
		api.DELETE("/data/:id", s.handleDeleteData)
	}

	// WebSocket 路由
	s.router.GET("/ws", s.handleWebSocket)

	// 健康检查
	s.router.GET("/health", s.handleHealth)

	// 根路径
	s.router.GET("/", s.handleRoot)
}

func (s *LocalService) RegisterToProxy() error {
	// 连接到代理服务器的注册端口
	conn, err := net.Dial("tcp", s.proxyAddr)
	if err != nil {
		return fmt.Errorf("连接代理服务器失败: %v", err)
	}
	s.registerConn = conn

	// 发送注册请求
	registerReq := RegisterRequest{
		Name:     s.name,
		Protocol: "http",
		Port:     s.port,
		Weight:   1,
	}

	if err := json.NewEncoder(conn).Encode(registerReq); err != nil {
		return fmt.Errorf("发送注册请求失败: %v", err)
	}

	log.Printf("服务 %s 已注册到代理服务器", s.name)

	// 启动请求处理协程
	go s.handleProxyRequests()

	return nil
}

func (s *LocalService) handleProxyRequests() {
	reader := bufio.NewReader(s.registerConn)

	for {
		if s.registerConn == nil {
			break
		}

		// 读取来自代理的 HTTP 请求
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("读取代理请求失败: %v", err)
			}
			s.registerConn = nil
			break
		}

		log.Printf("收到代理请求: %s %s", req.Method, req.URL.Path)

		// 处理请求并发送响应
		go s.processProxyRequest(req)
	}
}

func (s *LocalService) processProxyRequest(req *http.Request) {
	// 创建一个管道来捕获响应
	pr, pw := io.Pipe()

	// 创建响应写入器
	respWriter := &proxyResponseWriter{
		pipeWriter: pw,
		header:     make(http.Header),
		statusCode: 200,
	}

	// 在新的协程中处理请求
	go func() {
		defer pw.Close()

		// 使用 Gin 路由处理请求
		s.router.ServeHTTP(respWriter, req)
	}()

	// 构造 HTTP 响应并发送回代理
	response := &http.Response{
		Status:        fmt.Sprintf("%d %s", respWriter.statusCode, http.StatusText(respWriter.statusCode)),
		StatusCode:    respWriter.statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        respWriter.header,
		Body:          pr,
		ContentLength: -1,
	}

	if err := response.Write(s.registerConn); err != nil {
		log.Printf("发送响应到代理失败: %v", err)
	}
}

// 自定义响应写入器，用于捕获 Gin 的响应
type proxyResponseWriter struct {
	pipeWriter *io.PipeWriter
	header     http.Header
	statusCode int
}

func (w *proxyResponseWriter) Header() http.Header {
	return w.header
}

func (w *proxyResponseWriter) Write(data []byte) (int, error) {
	return w.pipeWriter.Write(data)
}

func (w *proxyResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (s *LocalService) StartHTTPServer() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("本地服务 %s 启动在端口 %d", s.name, s.port)
	return s.router.Run(addr)
}

func (s *LocalService) handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": s.name,
		"message": "服务运行正常",
		"port":    s.port,
		"time":    time.Now().Format(time.RFC3339),
		"endpoints": []string{
			"/api/hello",
			"/api/info",
			"/api/data",
			"/ws",
			"/health",
		},
	})
}

func (s *LocalService) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": s.name,
		"port":    s.port,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleHello(c *gin.Context) {
	name := c.DefaultQuery("name", "World")

	c.JSON(http.StatusOK, gin.H{
		"service": s.name,
		"message": fmt.Sprintf("Hello, %s! from %s", name, s.name),
		"port":    s.port,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":     s.name,
		"port":        s.port,
		"status":      "running",
		"uptime":      time.Now().Format(time.RFC3339),
		"method":      c.Request.Method,
		"path":        c.Request.URL.Path,
		"remote_addr": c.ClientIP(),
		"user_agent":  c.GetHeader("User-Agent"),
		"headers":     c.Request.Header,
		"query":       c.Request.URL.Query(),
	})
}

func (s *LocalService) handleData(c *gin.Context) {
	// 模拟一些业务数据
	data := []gin.H{
		{"id": 1, "name": "数据1", "value": 100, "created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
		{"id": 2, "name": "数据2", "value": 200, "created_at": time.Now().Add(-12 * time.Hour).Format(time.RFC3339)},
		{"id": 3, "name": "数据3", "value": 300, "created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
	}

	c.JSON(http.StatusOK, gin.H{
		"service": s.name,
		"data":    data,
		"count":   len(data),
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleCreateData(c *gin.Context) {
	var newData struct {
		Name  string `json:"name" binding:"required"`
		Value int    `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&newData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数错误",
			"details": err.Error(),
		})
		return
	}

	// 模拟创建数据
	createdData := gin.H{
		"id":         4,
		"name":       newData.Name,
		"value":      newData.Value,
		"created_at": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, gin.H{
		"service": s.name,
		"message": "数据创建成功",
		"data":    createdData,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleUpdateData(c *gin.Context) {
	id := c.Param("id")

	var updateData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数错误",
			"details": err.Error(),
		})
		return
	}

	// 模拟更新数据
	updatedData := gin.H{
		"id":         id,
		"name":       updateData.Name,
		"value":      updateData.Value,
		"updated_at": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, gin.H{
		"service": s.name,
		"message": "数据更新成功",
		"data":    updatedData,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleDeleteData(c *gin.Context) {
	id := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"service": s.name,
		"message": fmt.Sprintf("数据 %s 删除成功", id),
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *LocalService) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket 连接建立: %s", c.ClientIP())

	// 发送欢迎消息
	welcomeMsg := gin.H{
		"type":    "welcome",
		"service": s.name,
		"message": "WebSocket 连接已建立",
		"time":    time.Now().Format(time.RFC3339),
	}
	conn.WriteJSON(welcomeMsg)

	// 处理消息
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("读取 WebSocket 消息失败: %v", err)
			break
		}

		// 回显消息
		response := gin.H{
			"type":     "echo",
			"service":  s.name,
			"received": msg,
			"time":     time.Now().Format(time.RFC3339),
		}

		if err := conn.WriteJSON(response); err != nil {
			log.Printf("发送 WebSocket 消息失败: %v", err)
			break
		}
	}
}

func (s *LocalService) Close() {
	if s.registerConn != nil {
		s.registerConn.Close()
	}
}

func main() {
	var (
		name      = flag.String("name", "localservice", "服务名称")
		port      = flag.Int("port", 8080, "服务端口")
		proxyAddr = flag.String("proxy", "localhost:3001", "代理服务器注册地址")
		debug     = flag.Bool("debug", false, "启用调试模式")
		proxyOnly = flag.Bool("proxy-only", false, "仅通过代理提供服务，不启动本地HTTP服务器")
	)
	flag.Parse()

	// 设置 Gin 模式
	if *debug {
		gin.SetMode(gin.DebugMode)
	}

	service := NewLocalService(*name, *port, *proxyAddr)

	// 注册到代理服务器
	if err := service.RegisterToProxy(); err != nil {
		log.Fatalf("注册到代理服务器失败: %v", err)
	}

	// 优雅关闭
	defer service.Close()

	if *proxyOnly {
		// 仅通过代理提供服务，保持程序运行
		log.Printf("服务 %s 仅通过代理提供服务", *name)
		select {} // 阻塞主协程
	} else {
		// 启动本地 HTTP 服务器（用于直接访问和调试）
		if err := service.StartHTTPServer(); err != nil {
			log.Fatalf("启动 HTTP 服务器失败: %v", err)
		}
	}
}
