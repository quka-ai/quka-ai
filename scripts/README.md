# 构建脚本使用说明

## 脚本概览

本目录包含了支持多架构构建的脚本：

- `build.sh` - Go 二进制文件多架构构建
- `build-image.sh` - Docker 多架构镜像构建  

## 多架构构建支持

所有构建脚本都支持多架构构建，默认支持 `linux/amd64` 和 `linux/arm64`。

### Go 二进制构建 (`build.sh`)

构建 Go 二进制文件，支持交叉编译到不同平台架构。

#### 基本用法
```bash
./scripts/build.sh
```

#### 自定义架构
通过 `ARCHITECTURES` 环境变量指定目标架构（格式：`GOOS/GOARCH`）：

```bash
# 只构建 AMD64
ARCHITECTURES="linux/amd64" ./scripts/build.sh

# 只构建 ARM64
ARCHITECTURES="linux/arm64" ./scripts/build.sh

# 构建多个架构
ARCHITECTURES="linux/amd64,linux/arm64" ./scripts/build.sh

# 构建 Windows 和 macOS
ARCHITECTURES="windows/amd64,darwin/arm64" ./scripts/build.sh
```

#### 构建标签控制
通过 `BUILD_TAGS` 环境变量控制构建特性（默认构建开源版本）：

```bash
# 默认构建
./scripts/build.sh

#### 输出文件
- **单架构构建**: 生成 `_build/quka`
- **多架构构建**: 生成 `_build/quka-{GOOS}-{GOARCH}` 格式的文件

### Docker 镜像构建 (`build-image.sh`)

使用 Docker Buildx 构建多架构容器镜像。

#### 基本用法
```bash
./scripts/build-image.sh <IMAGE_PROJECT> <VERSION> [push]
```

#### 参数说明
- `IMAGE_PROJECT`: 镜像仓库前缀（如 `registry.example.com/myproject`）
- `VERSION`: 镜像版本标签
- `push`: 可选参数，指定 "push" 时会推送镜像到仓库

#### 使用示例
```bash
# 本地构建多架构镜像
./scripts/build-image.sh myregistry/quka v1.0.0

# 构建并推送多架构镜像
./scripts/build-image.sh myregistry/quka v1.0.0 push

# 自定义平台
PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7" ./scripts/build-image.sh myregistry/quka v1.0.0
```

#### 环境变量控制
- `PLATFORMS`: 指定构建平台，默认 `linux/amd64,linux/arm64`
- `HTTP_PROXY`: HTTP 代理设置
- `HTTPS_PROXY`: HTTPS 代理设置

### 本地 Docker 构建 (`build-local.sh`)

使用本地 Dockerfile 构建多架构镜像。

#### 基本用法
```bash
./scripts/build-local.sh <IMAGE_PROJECT> <VERSION>
```

#### 使用示例
```bash
# 构建并推送本地多架构镜像
./scripts/build-local.sh myregistry/quka-service v1.0.0

# 自定义平台
PLATFORMS="linux/amd64" ./scripts/build-local.sh myregistry/quka-service v1.0.0
```

## 构建环境要求

### Go 构建要求
- Go 1.25+ (推荐)
- 支持目标平台的工具链

### Docker 构建要求
- Docker 20.10+
- Docker Buildx 插件
- 多架构支持（自动创建 buildx builder）

## 高级配置

### 支持的架构组合
常用的架构组合：
```bash
# 服务器架构
ARCHITECTURES="linux/amd64,linux/arm64"

# 桌面应用
ARCHITECTURES="windows/amd64,darwin/arm64,darwin/amd64,linux/amd64"

# 嵌入式设备
ARCHITECTURES="linux/arm/v6,linux/arm/v7,linux/arm64"

# 容器平台
PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7"
```

### 构建优化

构建脚本包含以下优化：
- 静态链接以减少依赖
- CGO 禁用以提高兼容性
- 自动创建 Docker Buildx builder
- 构建失败时的错误处理

### 故障排除

#### Go 构建问题
1. **依赖问题**: 运行 `go mod tidy` 更新依赖
2. **CGO 错误**: 确保设置了 `CGO_ENABLED=0`
3. **架构不支持**: 检查 Go 是否支持目标架构

#### Docker 构建问题
1. **Buildx 未安装**: 安装 Docker Buildx 插件
2. **平台不支持**: 检查 Docker 多架构支持
3. **网络问题**: 设置代理环境变量

## 示例工作流

### 开发环境构建
```bash
# 快速本地构建（当前架构）
ARCHITECTURES="$(go env GOOS)/$(go env GOARCH)" ./scripts/build.sh
```

### 生产环境发布
```bash
# 构建多架构二进制
./scripts/build.sh

# 构建并推送多架构 Docker 镜像
./scripts/build-image.sh myregistry/quka v1.0.0 push
```

### CI/CD 集成
```bash
# 在 CI 环境中的典型使用
export ARCHITECTURES="linux/amd64,linux/arm64"
export PLATFORMS="linux/amd64,linux/arm64"

./scripts/build.sh
./scripts/build-image.sh $CI_REGISTRY_IMAGE $CI_COMMIT_TAG push
```