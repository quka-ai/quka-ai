# RSS 订阅功能实现状态

## 已完成

### 1. 数据库表定义 ✅
- [x] `app/store/sqlstore/rss_subscription.sql`
- [x] `app/store/sqlstore/rss_article.sql`
- [x] `app/store/sqlstore/rss_user_interest.sql`

### 2. 数据类型定义 ✅
- [x] `pkg/types/rss.go`
  - RSSSubscription
  - RSSArticle
  - RSSUserInterest
  - RSSFeedItem
  - RSSFeed

### 3. 数据访问层 ✅
- [x] `app/store/sqlstore/rss_subscription_store.go`
- [x] `app/store/sqlstore/rss_article_store.go`
- [x] `app/store/sqlstore/rss_user_interest_store.go`
- [x] 在 `app/store/store.go` 中注册新的 Store 接口
- [x] 在 `app/store/sqlstore/provider.go` 中注册新的 Store 实现

### 4. RSS 解析和抓取 ✅
- [x] `pkg/rss/parser.go` - RSS/Atom 格式解析（使用 gofeed 库）
- [x] `pkg/rss/fetcher.go` - HTTP 抓取器（带重试机制）

### 5. 内容处理 ✅
- [x] `pkg/rss/processor.go`
  - 文章去重
  - 创建 Knowledge 记录
  - 自动过期管理
  - 用户兴趣模型更新

### 6. 业务逻辑层 ✅
- [x] `app/logic/v1/rss_subscription.go`
  - 订阅 CRUD（创建、查询、更新、删除）
  - 手动触发抓取
  - 权限检查
- [x] `app/logic/v1/rss_fetcher.go`
  - 单个订阅抓取
  - 批量抓取所有启用的订阅
  - 旧文章清理

## 待实现

### 7. API 层 ✅
- [x] `cmd/service/handler/rss.go` - HTTP 处理器
  - CreateRSSSubscription - 创建RSS订阅
  - GetRSSSubscription - 获取订阅详情
  - ListRSSSubscriptions - 获取订阅列表
  - UpdateRSSSubscription - 更新订阅配置
  - DeleteRSSSubscription - 删除订阅
  - TriggerRSSFetch - 手动触发抓取
- [x] 在路由中注册 RSS API

### 8. 定时任务
- [ ] `pkg/rss/scheduler.go`
  - Cron 任务调度
  - 定时抓取

### 9. 测试
- [ ] 单元测试
- [ ] 集成测试

## 依赖库

需要添加到 go.mod:
```
github.com/mmcdole/gofeed v1.2.1  // RSS 解析
github.com/robfig/cron/v3 v3.0.1  // 定时任务
```

## 下一步

建议按以下顺序继续实现：
1. 完成剩余的 Store (rss_article_store, rss_user_interest_store)
2. 实现 RSS 解析器和抓取器
3. 实现内容处理器（AI 摘要）
4. 实现业务逻辑层
5. 实现 API 层
6. 实现定时任务
7. 编写测试
