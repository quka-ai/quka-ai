#!/bin/bash

# E2E 测试运行脚本

echo "启动 E2E 测试..."

# 启动代理服务
echo "启动代理服务..."
go run cmd/proxy/main.go -listen 127.0.0.1:3000 -register-port 3001 &
PROXY_PID=$!

# 等待代理启动
sleep 2

# 启动本地服务实例（仅通过代理提供服务）
echo "启动本地服务实例（内网建联模式）..."
go run cmd/localservice/main.go -name userservice -port 8080 -proxy localhost:3001 -proxy-only &
SERVICE1_PID=$!
go run cmd/localservice/main.go -name userservice -port 8081 -proxy localhost:3001 -proxy-only &
SERVICE2_PID=$!
go run cmd/localservice/main.go -name orderservice -port 8082 -proxy localhost:3001 -proxy-only &
SERVICE3_PID=$!

# 等待服务启动和注册
sleep 3

# 运行测试客户端
echo "运行测试客户端..."

echo "=== 测试 1: 请求根路径 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /

echo "=== 测试 2: 健康检查 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /health

echo "=== 测试 3: Hello 接口 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /api/hello

echo "=== 测试 4: Hello 接口带参数 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path "/api/hello?name=张三"

echo "=== 测试 5: 服务信息接口 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /api/info

echo "=== 测试 6: 数据接口 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /api/data

echo "=== 测试 7: 请求 orderservice ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target orderservice -path /api/hello

echo "=== 测试 8: 负载均衡测试 (userservice) ==="
for i in {1..3}; do
    echo "请求 $i:"
    go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target userservice -path /api/hello
done

# 测试 POST 请求 (需要使用 curl)
echo "=== 测试 9: POST 请求测试 ==="
curl -X POST \
  -H "quka-target: userservice" \
  -H "Content-Type: application/json" \
  -d '{"name":"新数据","value":999}' \
  http://127.0.0.1:3000/api/data

echo ""

# 测试 PUT 请求
echo "=== 测试 10: PUT 请求测试 ==="
curl -X PUT \
  -H "quka-target: userservice" \
  -H "Content-Type: application/json" \
  -d '{"name":"更新数据","value":888}' \
  http://127.0.0.1:3000/api/data/1

echo ""

# 测试 DELETE 请求
echo "=== 测试 11: DELETE 请求测试 ==="
curl -X DELETE \
  -H "quka-target: userservice" \
  http://127.0.0.1:3000/api/data/1

echo ""

# 测试服务不存在的情况
echo "=== 测试 12: 请求不存在的服务 ==="
go run e2e/testclient/main.go -proxy 127.0.0.1:3000 -target nonexistent -path /api/hello

echo ""

# 清理进程
echo "清理测试进程..."
kill $SERVICE1_PID $SERVICE2_PID $SERVICE3_PID $PROXY_PID 2>/dev/null

echo "E2E 测试完成!" 