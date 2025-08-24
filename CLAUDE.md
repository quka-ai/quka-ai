

# QukaAI 项目说明

## 项目简介
QukaAI (Quokka) 是一个轻量级、用户友好的检索增强生成 (RAG) 系统，旨在帮助用户构建自己的第二大脑。

## 技术栈
- **后端**: Go 1.23.1
- **数据库**: PostgreSQL + pgvector 扩展
- **Web框架**: Gin
- **数据库操作**: sqlx + squirrel
- **缓存**: Redis
- **消息**: WebSocket (gorilla/websocket)
- **AI集成**: 支持 OpenAI、Azure OpenAI、Qwen、Gemini、Ollama 等多种AI服务
- **对象存储**: S3 兼容存储 (AWS S3/MinIO)
- **认证**: JWT
- **监控**: Prometheus
- **国际化**: i18n支持

## 项目结构
```
app/
├── core/           # 核心模块（配置、插件、服务）
├── logic/v1/       # 业务逻辑层
├── response/       # 响应处理
└── store/sqlstore/ # 数据库存储层

cmd/
├── main.go         # 主程序入口
└── service/        # 服务配置和路由

pkg/
├── ai/            # AI服务集成
├── types/         # 数据类型定义
├── utils/         # 工具函数
└── ...

scripts/
├── build.sh       # 构建脚本
└── build-image.sh # 镜像构建脚本
```

## 构建和运行
```bash
# 构建服务
go build -o quka ./cmd/

# 运行服务
./quka service -c config.toml

# 使用构建脚本
./scripts/build.sh
```

## 开发规范
- 使用Go标准代码风格
- 数据库操作统一使用sqlx
- API遵循RESTful设计
- 错误处理使用统一的错误类型
- **错误信息国际化**: 项目中所有错误处理都必须遵循当前项目的i18n方式，使用 `pkg/i18n` 包中定义的错误码
- 日志使用结构化日志记录
- 测试覆盖率要求：核心业务逻辑 > 80%

## 主要功能模块
1. **用户管理**: 用户注册、登录、权限管理
2. **聊天系统**: 实时聊天、消息历史、会话管理
3. **知识库**: 文档上传、向量化、检索
4. **RAG系统**: 检索增强生成、多AI模型支持
5. **日志系统**: 结构化日志、用户行为记录
6. **文件管理**: 文件上传、存储、预览
7. **分享功能**: 内容分享、权限控制

## 测试
```bash
# 运行单元测试
go test ./...

# 运行特定包的测试
go test ./app/logic/v1/...
```

## 代码提交规范
- feat: 新功能
- fix: 修复问题
- docs: 文档更新
- style: 代码格式化
- refactor: 重构代码
- test: 添加测试
- chore: 构建过程或辅助工具的变动

## 部署
项目支持Docker部署，配置文件位于 `cmd/service/etc/service-default.toml`

## 开发环境设置
1. 确保Go 1.23.1+已安装
2. 安装PostgreSQL并启用pgvector扩展
3. 配置Redis服务
4. 复制并修改配置文件
5. 运行数据库迁移脚本

## Claude Code 开发配置

### Go 开发环境
- Go版本: 1.23.1+
- 依赖管理: Go Modules
- 代码格式化: gofmt
- 静态分析: go vet, golint

### 常用命令
```bash
# 格式化代码
go fmt ./...

# 静态检查
go vet ./...

# 综合代码检查 (推荐)
golangci-lint run

# 运行测试
go test ./...

# 运行测试并生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 构建
go build -o quka ./cmd/

# 使用构建脚本
./scripts/build.sh

# 清理模块缓存
go clean -modcache

# 更新依赖
go mod tidy
```

### 推荐开发工具
- IDE: VS Code + Go扩展
- 调试: delve
- 性能分析: go tool pprof
- 依赖分析: go mod graph

### 代码质量检查
- 使用 `gofmt` 进行代码格式化
- 使用 `go vet` 进行静态分析
- 使用 `go test` 运行单元测试
- 推荐使用 `golangci-lint` 进行综合代码检查

## 项目文档管理规范

### 重要提醒：每次制定修改计划都需要落入文档
- **文档组织方式**: 按功能模块创建专门的文件夹
- **命名规范**: 文件名使用英文，便于管理和国际化协作
- **内容语言**: 文档内容使用中文，便于中国开发团队理解
- **文档结构**: 每个重构计划都有专门的 markdown 文件

### 文档目录结构
```
docs/
├── refactoring-plans/          # 重构计划文档
│   ├── knowledge-list-optimization.md
│   ├── user-auth-enhancement.md
│   └── ...
├── api-design/                 # API 设计文档
├── architecture/               # 架构设计文档
└── deployment/                 # 部署相关文档
```

### Claude Code 工作流程
1. 分析项目需求和数据结构
2. 制定详细的改造计划
3. **为每个计划创建专门的文档** (例如: `/docs/refactoring-plans/feature-name.md`)
4. 等待用户 review 和确认
5. 开始实施代码修改

### 文档内容规范
每个改造计划文档应包含:
- 问题描述和背景
- 改造目标和步骤
- 详细的实施方案
- 关键考虑点
- 时间线和状态追踪
- 需要确认的问题
- 相关文件列表

## 相关链接
- [主仓库](https://github.com/quka-ai/quka-ai)
- [前端项目](https://github.com/quka-ai/webapp)
- [Discord社区](https://discord.gg/YGrbmbCVRF)