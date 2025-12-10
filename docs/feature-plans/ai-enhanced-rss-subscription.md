# AI 增强的 RSS 订阅功能开发文档

## 文档信息

- **创建日期**: 2025-12-10
- **状态**: 规划中
- **负责人**: 待定
- **预计开始时间**: 待定

## 目录

- [1. 功能概述](#1-功能概述)
- [2. 产品定位](#2-产品定位)
- [3. 技术架构](#3-技术架构)
- [4. 分阶段实施计划](#4-分阶段实施计划)
  - [4.1 第一阶段：核心功能](#41-第一阶段核心功能)
  - [4.2 第二阶段：AI 增强](#42-第二阶段ai-增强)
  - [4.3 第三阶段：高级功能](#43-第三阶段高级功能)
- [5. 数据库设计](#5-数据库设计)
- [6. API 设计](#6-api-设计)
- [7. 关键技术点](#7-关键技术点)
- [8. 风险和挑战](#8-风险和挑战)
- [9. 测试计划](#9-测试计划)
- [10. 未来扩展](#10-未来扩展)

---

## 1. 功能概述

QukaAI 作为 AI 赋能的记忆管理应用，RSS 订阅功能不仅仅是传统的信息聚合工具，而是一个智能的信息摄入和处理系统，能够：

- 自动从 RSS 源获取内容并智能处理
- 将有价值的信息自动整合到用户的知识库
- 提供个性化的内容过滤和推荐
- 通过对话式交互让用户更高效地消费信息
- 将被动阅读转变为主动学习和记忆构建

### 核心价值主张

- **智能化**: AI 自动提取、分类、总结订阅内容
- **个性化**: 基于用户兴趣和知识库的智能推荐
- **知识化**: 无缝集成到用户的"第二大脑"
- **对话式**: 通过自然语言与订阅内容交互

---

## 2. 产品定位

### 与传统 RSS 阅读器的区别

| 特性     | 传统 RSS 阅读器 | QukaAI RSS 订阅       |
| -------- | --------------- | --------------------- |
| 内容展示 | 按时间倒序列表  | 智能排序 + 个性化推荐 |
| 内容处理 | 原样呈现        | AI 摘要 + 知识提取    |
| 信息管理 | 标记已读/未读   | 自动加入知识库 + 关联 |
| 交互方式 | 浏览 + 点击     | 对话式检索 + 主动提醒 |
| 学习支持 | 无              | 间隔复习 + 知识卡片   |

### 目标用户场景

1. **知识工作者**: 需要追踪行业动态，构建专业知识体系
2. **终身学习者**: 订阅多个学习资源，希望系统化吸收
3. **研究人员**: 追踪特定领域的研究进展
4. **内容创作者**: 收集灵感和素材

---

## 3. 技术架构

### 3.1 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         用户界面层                            │
│  (Web App / Desktop App / Mobile App)                       │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP/WebSocket
┌──────────────────▼──────────────────────────────────────────┐
│                      API Gateway (Gin)                       │
└──────────────────┬──────────────────────────────────────────┘
                   │
      ┌────────────┼────────────┐
      │            │            │
┌─────▼─────┐ ┌───▼────┐ ┌────▼─────┐
│ RSS Logic │ │ Chat   │ │Knowledge │
│  Module   │ │ Logic  │ │  Logic   │
└─────┬─────┘ └───┬────┘ └────┬─────┘
      │           │            │
      └───────────┼────────────┘
                  │
┌─────────────────▼──────────────────────────────────────────┐
│                      Core Services                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │RSS Fetch │  │AI Service│  │Vector DB │  │Schedule  │   │
│  │  Service │  │(摘要/提取)│  │ (pgvector)│  │  Service │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────┬──────────────────────────────────────────┘
                  │
      ┌───────────┼───────────┐
      │           │           │
┌─────▼─────┐ ┌──▼────┐ ┌───▼────┐
│PostgreSQL │ │ Redis │ │  S3    │
│(主数据库)  │ │(缓存) │ │(附件)  │
└───────────┘ └───────┘ └────────┘
```

### 3.2 核心模块

#### RSS 订阅管理模块

- **位置**: `app/logic/v1/rss_subscription.go`
- **职责**: RSS 源的 CRUD 操作、订阅管理

#### RSS 抓取服务

- **位置**: `pkg/rss/fetcher.go`
- **职责**: 定时抓取 RSS 源、解析 XML/JSON、去重

#### RSS 内容处理服务

- **位置**: `pkg/rss/processor.go`
- **职责**: AI 摘要生成、知识提取、标签分类

#### RSS 智能推荐服务

- **位置**: `pkg/rss/recommender.go`
- **职责**: 个性化排序、智能过滤、相关性评分

---

## 4. 分阶段实施计划

## 4.1 第一阶段：核心功能

### 目标

建立基础的 RSS 订阅系统，实现内容抓取、存储和基本的 AI 处理能力。

### 功能列表

#### 4.1.1 RSS 订阅管理

- [ ] 添加 RSS 订阅源（URL 验证）
- [ ] 查看订阅列表
- [ ] 编辑订阅源（名称、分组、更新频率）
- [ ] 删除订阅源
- [ ] 订阅源分组管理
- [ ] 订阅源启用/禁用

#### 4.1.2 RSS 内容抓取

- [ ] 定时抓取任务调度器
- [ ] RSS/Atom 格式解析
- [ ] 内容去重机制
- [ ] 增量更新支持
- [ ] 抓取失败重试机制
- [ ] 抓取日志记录

#### 4.1.3 内容存储

- [ ] RSS 文章数据模型设计
- [ ] 文章内容存储（标题、正文、作者、链接等）
- [ ] 附件/图片处理
- [ ] 文章状态管理（未读/已读/收藏）

#### 4.1.4 AI 智能摘要

- [ ] 集成现有 AI 服务（OpenAI/Qwen 等）
- [ ] 为每篇文章生成摘要
- [ ] 提取关键词
- [ ] 多语言支持（自动检测语言）
- [ ] 摘要缓存机制

#### 4.1.5 知识库自动集成

- [ ] 将 RSS 文章向量化（利用现有 pgvector）
- [ ] 自动加入用户知识库
- [ ] 与现有知识关联
- [ ] 支持在 RAG 检索中使用 RSS 内容

#### 4.1.6 基础 UI

- [ ] 订阅源管理界面
- [ ] 文章列表（时间线视图）
- [ ] 文章详情页
- [ ] 未读/已读标记
- [ ] 收藏功能

### 技术实现细节

#### 数据库表设计

```sql
-- RSS 订阅源表
CREATE TABLE rss_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    space_id VARCHAR(32) NOT NULL,      -- 订阅所属空间
    resource_id VARCHAR(32) NOT NULL,   -- 订阅内容存入的 resource（由用户指定）
    url VARCHAR(512) NOT NULL,
    title VARCHAR(255),
    description TEXT,
    category VARCHAR(100),
    update_frequency INT DEFAULT 3600, -- 秒
    last_fetched_at TIMESTAMP,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, url)
);

-- RSS 文章表（仅用于去重，不存储用户关系）
CREATE TABLE rss_articles (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    guid VARCHAR(512) NOT NULL, -- RSS item guid
    title VARCHAR(512),
    link VARCHAR(1024),
    description TEXT,
    content TEXT,
    author VARCHAR(255),
    published_at TIMESTAMP,
    fetched_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(subscription_id, guid)
);

-- 注意：用户的 RSS 文章内容存储在 quka_knowledge 表中
-- quka_knowledge 表字段映射：
-- - id: knowledge ID
-- - user_id: 用户ID
-- - space_id: 空间ID（从订阅配置继承）
-- - resource: resource ID（从订阅配置继承，控制过期策略）
-- - kind: 'rss' (标识为 RSS 类型)
-- - title: 文章标题
-- - content: 文章内容
-- - summary: AI 生成的摘要
-- - tags: 提取的关键词
-- - rel_doc_id: rss_articles.id (直接存储 article ID)
-- - expired_at: 根据 resource.cycle 自动计算
--
-- 设计说明：
-- 1. rel_doc_id 只存储 article_id，系统通过 kind='rss' 判断类型
-- 2. 不存储 is_read/is_starred，通过以下方式实现：
--    - "已读"：文章未过期即视为可阅读
--    - "收藏"：转存到永久 resource（cycle=0）
-- 3. 文章评分实时计算，基于 rss_user_interests 匹配度
-- 4. 原始文章信息（link, author）可通过 JOIN rss_articles 获取

-- 索引
CREATE INDEX idx_rss_subscriptions_user_space ON rss_subscriptions(user_id, space_id);
CREATE INDEX idx_rss_subscriptions_resource ON rss_subscriptions(resource_id);
CREATE INDEX idx_rss_subscriptions_enabled ON rss_subscriptions(enabled);
CREATE INDEX idx_rss_articles_subscription_id ON rss_articles(subscription_id);
CREATE INDEX idx_rss_articles_published_at ON rss_articles(published_at DESC);
```

#### API 端点设计

```
# RSS 订阅管理
POST   /api/v1/rss/subscriptions           # 添加订阅（需指定 space_id 和 resource_id）
GET    /api/v1/rss/subscriptions           # 获取订阅列表
GET    /api/v1/rss/subscriptions/:id       # 获取订阅详情
PUT    /api/v1/rss/subscriptions/:id       # 更新订阅（可修改 resource_id 调整过期策略）
DELETE /api/v1/rss/subscriptions/:id       # 删除订阅

POST   /api/v1/rss/subscriptions/:id/fetch # 手动触发单个订阅抓取

# RSS 文章查询（复用 knowledge API）
GET    /api/v1/knowledge?kind=rss&resource_id=xxx&include_expired=false
GET    /api/v1/knowledge/:id               # 获取文章详情

# RSS 文章操作（复用 knowledge API）
PUT    /api/v1/knowledge/:id               # 更新文章（可修改 resource 转存、更新 metadata）
DELETE /api/v1/knowledge/:id               # 删除文章

# 批量操作
POST   /api/v1/knowledge/batch/update      # 批量更新（如批量转存到其他 resource）
POST   /api/v1/knowledge/batch/delete      # 批量删除
```

**说明：**

- RSS 文章作为 knowledge 存储，复用现有的 knowledge API
- 转存文章：通过更新 knowledge.resource 字段实现

```

#### 核心代码结构

```

pkg/rss/
├── fetcher.go # RSS 抓取器
├── parser.go # RSS 解析器
├── processor.go # 内容处理器（AI 摘要、知识提取）
├── scheduler.go # 定时任务调度
└── types.go # 数据类型定义

app/logic/v1/
├── rss_subscription.go # 订阅管理逻辑
└── rss_article.go # 文章管理逻辑

app/store/sqlstore/
├── rss_subscription_store.go # 订阅数据访问层
└── rss_article_store.go # 文章数据访问层

````

### 验收标准

- [ ] 用户可以添加至少 10 个不同格式的 RSS 源
- [ ] 系统能够正确解析 RSS 2.0 和 Atom 1.0 格式
- [ ] 定时任务稳定运行，无内存泄漏
- [ ] AI 摘要准确率 > 85%（人工抽样评估）
- [ ] RSS 内容能在 RAG 检索中被正确召回
- [ ] 响应时间：列表加载 < 1s，详情加载 < 500ms

### 预估工作量

- **后端开发**: 10-12 工作日
- **前端开发**: 5-7 工作日
- **测试**: 3-4 工作日
- **总计**: 约 3-4 周

---

## 4.2 第二阶段：AI 增强

### 目标

利用 AI 能力提供个性化的内容过滤、智能推荐和主动提醒功能。

### 功能列表

#### 4.2.1 个性化过滤和排序

- [ ] 基于用户阅读历史的兴趣建模
- [ ] 内容相关性评分算法
- [ ] 智能排序（非简单时间排序）
- [ ] 自定义过滤规则
- [ ] 自动隐藏低价值内容
- [ ] 内容去重和聚合

#### 4.2.2 智能提醒

- [ ] 检测与用户知识库高度相关的新文章
- [ ] 检测用户正在关注的话题更新
- [ ] 检测与最近对话相关的内容
- [ ] 提醒策略配置（频率、方式）
- [ ] WebSocket 实时推送
- [ ] 邮件/桌面通知支持

#### 4.2.3 对话式检索

- [ ] 自然语言查询 RSS 内容
  - "最近有哪些关于 AI 的文章？"
  - "总结一下本周的技术新闻"
- [ ] 跨文章的综合回答
- [ ] 引用来源追溯
- [ ] 对话历史记忆

#### 4.2.4 内容深度分析

- [ ] 情感分析（正面/负面/中性）
- [ ] 主题分类
- [ ] 实体识别（人物、组织、地点等）
- [ ] 观点提取
- [ ] 事实核查（基础版）

#### 4.2.5 智能推荐

- [ ] 推荐相关文章（基于当前阅读）
- [ ] 推荐新的 RSS 源
- [ ] 发现内容缺口
- [ ] 每日/每周精选推送

### 技术实现细节

#### 用户兴趣模型

```sql
-- 用户兴趣模型表
CREATE TABLE rss_user_interests (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    weight FLOAT DEFAULT 1.0, -- 兴趣权重 (0.0 - 1.0)
    source VARCHAR(50), -- 'explicit'(用户明确表示) 或 'implicit'(行为推断)
    last_updated_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, topic)
);

CREATE INDEX idx_rss_user_interests_user_id ON rss_user_interests(user_id);
CREATE INDEX idx_rss_user_interests_weight ON rss_user_interests(user_id, weight DESC);
```

**用途说明：**
- **个性化推荐**：根据用户兴趣权重对 RSS 文章进行排序
- **智能提醒**：新文章匹配高权重兴趣时触发提醒
- **兴趣学习**：根据用户阅读行为自动更新权重
  - 用户阅读/收藏某主题文章 → 权重 +0.1
  - 用户跳过某主题文章 → 权重 -0.05
  - 定期衰减不活跃主题的权重

**示例数据：**
```
user_id='user123', topic='Rust编程', weight=0.95, source='implicit'
user_id='user123', topic='AI技术', weight=0.82, source='explicit'
user_id='user123', topic='前端开发', weight=0.45, source='implicit'
```

#### 智能提醒配置

```sql
-- 提醒规则表
CREATE TABLE rss_alert_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    rule_type VARCHAR(50) NOT NULL, -- 'keyword', 'topic', 'knowledge_related'
    rule_config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 提醒历史表
CREATE TABLE rss_alerts (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    article_id BIGINT NOT NULL,
    rule_id BIGINT,
    reason TEXT,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### API 端点扩展

```
GET    /api/v1/rss/feed/personalized       # 个性化信息流
GET    /api/v1/rss/alerts                  # 获取智能提醒
POST   /api/v1/rss/alerts/rules            # 创建提醒规则
GET    /api/v1/rss/recommendations         # 获取推荐内容
POST   /api/v1/rss/query                   # 对话式查询
POST   /api/v1/rss/articles/:id/feedback   # 用户反馈（喜欢/不喜欢）
```

#### 核心服务扩展

```
pkg/rss/
├── recommender.go      # 推荐引擎
├── scorer.go           # 内容评分器
├── alerter.go          # 智能提醒服务
├── analyzer.go         # 内容分析器（情感、主题等）
└── query_engine.go     # 对话式查询引擎
```

### 算法设计

#### 相关性评分算法

```go
// 伪代码
func CalculateRelevanceScore(article Article, user User) float64 {
    score := 0.0

    // 1. 与用户兴趣主题的匹配度 (40%)
    topicScore := CalculateTopicMatch(article.Keywords, user.Interests)
    score += topicScore * 0.4

    // 2. 与用户知识库的相关性 (30%)
    knowledgeScore := VectorSimilarity(article.Embedding, user.KnowledgeBase)
    score += knowledgeScore * 0.3

    // 3. 与最近聊天的相关性 (20%)
    chatScore := CalculateChatRelevance(article, user.RecentChats)
    score += chatScore * 0.2

    // 4. 历史阅读行为 (10%)
    behaviorScore := CalculateBehaviorScore(article, user.ReadingHistory)
    score += behaviorScore * 0.1

    return score
}
```

#### 智能提醒触发条件

```go
// 触发提醒的条件
type AlertCondition struct {
    // 相关性阈值（0-1）
    RelevanceThreshold float64

    // 时间窗口（避免频繁打扰）
    MinIntervalHours int

    // 用户活跃时段
    ActiveHours []int
}

func ShouldAlert(article Article, user User, condition AlertCondition) bool {
    // 1. 相关性必须超过阈值
    if article.RelevanceScore < condition.RelevanceThreshold {
        return false
    }

    // 2. 距离上次提醒需要超过最小间隔
    if time.Since(user.LastAlertTime) < time.Hour * time.Duration(condition.MinIntervalHours) {
        return false
    }

    // 3. 当前在用户活跃时段
    if !IsActiveHour(time.Now().Hour(), condition.ActiveHours) {
        return false
    }

    return true
}
```

### 验收标准

- [ ] 个性化排序相比时间排序，用户点击率提升 > 30%
- [ ] 智能提醒的准确率 > 70%（用户反馈）
- [ ] 对话式查询响应时间 < 3s
- [ ] 推荐文章的点击率 > 15%
- [ ] 系统能够正确识别用户的前 10 个兴趣主题

### 预估工作量

- **后端开发**: 12-15 工作日
- **AI 算法调优**: 5-7 工作日
- **前端开发**: 6-8 工作日
- **测试和优化**: 4-5 工作日
- **总计**: 约 4-5 周

---

## 4.3 第三阶段：高级功能

### 目标

打造完整的知识管理生态，将 RSS 订阅深度融入用户的"第二大脑"。

### 功能列表

#### 4.3.1 知识图谱集成

- [ ] 从 RSS 文章中提取实体和关系
- [ ] 构建个人知识图谱
- [ ] 可视化知识网络
- [ ] 基于知识图谱的探索式阅读
- [ ] 自动发现概念之间的联系

#### 4.3.2 跨源分析

- [ ] 多源信息聚合
- [ ] 观点对比分析
- [ ] 趋势追踪和预测
- [ ] 争议话题识别
- [ ] 信息可信度评估

#### 4.3.3 定期综述生成

- [ ] 每日摘要（Daily Digest）
- [ ] 每周综述（Weekly Roundup）
- [ ] 主题深度报告
- [ ] 自定义报告模板
- [ ] 报告导出（PDF/Markdown）

#### 4.3.4 学习增强功能

- [ ] 间隔复习系统集成
  - 自动生成复习卡片
  - 智能调度复习时间
- [ ] 知识卡片生成
  - 从文章自动生成 Q&A 卡片
  - 概念解释卡片
- [ ] 个性化重写
  - 根据用户知识水平调整内容
  - 专业术语解释

#### 4.3.5 协作和分享

- [ ] 分享订阅集合（OPML 导出/导入）
- [ ] 分享单篇文章（带笔记）
- [ ] 团队协作（共享订阅源）
- [ ] 文章评论和讨论
- [ ] 公开知识库（可选）

#### 4.3.6 高级内容处理

- [ ] 全文提取（处理摘要型 RSS）
- [ ] 视频/音频内容转文本
- [ ] 多媒体内容理解
- [ ] 代码片段提取和高亮
- [ ] 数据表格结构化

#### 4.3.7 智能订阅源管理

- [ ] RSS 源质量评估
- [ ] 自动推荐新源
- [ ] 死链检测和清理
- [ ] 源更新频率自适应
- [ ] 订阅源健康度仪表板

### 技术实现细节

#### 知识图谱扩展

```sql
-- 知识实体表
CREATE TABLE knowledge_entities (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50), -- 'person', 'organization', 'concept', 'event', etc.
    description TEXT,
    source_type VARCHAR(50), -- 'rss', 'manual', 'chat', etc.
    source_id BIGINT,
    properties JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 实体关系表
CREATE TABLE knowledge_relations (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    from_entity_id BIGINT NOT NULL,
    to_entity_id BIGINT NOT NULL,
    relation_type VARCHAR(100), -- 'related_to', 'part_of', 'caused_by', etc.
    strength FLOAT DEFAULT 1.0,
    evidence TEXT[], -- 支持该关系的证据来源
    created_at TIMESTAMP DEFAULT NOW()
);

-- 文章与实体关联表
CREATE TABLE rss_article_entities (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    entity_id BIGINT NOT NULL,
    relevance FLOAT DEFAULT 1.0,
    context TEXT, -- 实体在文章中的上下文
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 学习卡片系统

```sql
-- 学习卡片表
CREATE TABLE learning_cards (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    card_type VARCHAR(50), -- 'qa', 'concept', 'summary'
    source_type VARCHAR(50), -- 'rss', 'knowledge', 'manual'
    source_id BIGINT,
    front TEXT NOT NULL, -- 问题或提示
    back TEXT NOT NULL,  -- 答案或内容
    tags TEXT[],
    difficulty INT DEFAULT 0, -- 0=未评级, 1-5=难度等级
    created_at TIMESTAMP DEFAULT NOW()
);

-- 间隔复习记录表
CREATE TABLE spaced_repetition_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    card_id BIGINT NOT NULL,
    reviewed_at TIMESTAMP NOT NULL,
    quality INT NOT NULL, -- 0-5, 用户回忆质量评分
    easiness_factor FLOAT,
    interval INT, -- 下次复习间隔（天）
    next_review_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 综述生成服务

```
pkg/rss/
├── digest_generator.go  # 摘要生成器
├── trend_analyzer.go    # 趋势分析器
└── report_builder.go    # 报告构建器
```

#### API 端点扩展

```
GET    /api/v1/rss/digest/daily            # 每日摘要
GET    /api/v1/rss/digest/weekly           # 每周综述
POST   /api/v1/rss/digest/custom           # 自定义综述
GET    /api/v1/rss/knowledge-graph         # 知识图谱数据
GET    /api/v1/rss/trends                  # 趋势分析
POST   /api/v1/rss/learning-cards          # 生成学习卡片
GET    /api/v1/rss/subscriptions/health    # 订阅源健康度
POST   /api/v1/rss/share                   # 分享订阅或文章
```

### 核心算法

#### 知识图谱构建

```go
// 从文章构建知识图谱
func BuildKnowledgeGraph(article Article) ([]Entity, []Relation) {
    // 1. 实体识别（使用 NER）
    entities := ExtractEntities(article.Content)

    // 2. 关系抽取
    relations := ExtractRelations(article.Content, entities)

    // 3. 实体链接（链接到已有实体）
    linkedEntities := LinkEntities(entities, existingKnowledgeBase)

    // 4. 关系强度计算
    for relation := range relations {
        relation.Strength = CalculateRelationStrength(relation, article)
    }

    return linkedEntities, relations
}
```

#### 趋势分析

```go
// 分析某个主题的趋势
func AnalyzeTrend(topic string, timeRange TimeRange) TrendReport {
    // 1. 获取相关文章
    articles := FetchArticlesByTopic(topic, timeRange)

    // 2. 时间序列分析
    timeSeries := BuildTimeSeries(articles)

    // 3. 情感变化
    sentimentTrend := AnalyzeSentimentOverTime(articles)

    // 4. 关键事件识别
    keyEvents := IdentifyKeyEvents(articles)

    // 5. 预测未来趋势
    prediction := PredictFutureTrend(timeSeries)

    return TrendReport{
        Topic:          topic,
        TimeSeries:     timeSeries,
        SentimentTrend: sentimentTrend,
        KeyEvents:      keyEvents,
        Prediction:     prediction,
    }
}
```

#### 间隔复习算法（SM-2 算法）

```go
// SuperMemo 2 算法实现
func CalculateNextReview(card LearningCard, quality int) SpacedRepetitionLog {
    log := SpacedRepetitionLog{
        CardID:     card.ID,
        Quality:    quality,
        ReviewedAt: time.Now(),
    }

    // 获取上次复习记录
    lastLog := GetLastReviewLog(card.ID)

    if quality < 3 {
        // 回忆质量差，重新开始
        log.Interval = 1
        log.EasinessFactor = max(1.3, lastLog.EasinessFactor - 0.2)
    } else {
        // 计算新的间隔
        if lastLog.Interval == 0 {
            log.Interval = 1
        } else if lastLog.Interval == 1 {
            log.Interval = 6
        } else {
            log.Interval = int(float64(lastLog.Interval) * lastLog.EasinessFactor)
        }

        // 更新容易度因子
        log.EasinessFactor = lastLog.EasinessFactor +
            (0.1 - float64(5-quality) * (0.08 + float64(5-quality) * 0.02))
    }

    log.NextReviewAt = time.Now().AddDate(0, 0, log.Interval)
    return log
}
```

### 验收标准

- [ ] 知识图谱能够正确识别 > 80% 的主要实体
- [ ] 趋势预测准确率 > 60%（一周内的预测）
- [ ] 学习卡片自动生成质量 > 75%（人工评估）
- [ ] 每日摘要生成时间 < 30s
- [ ] 知识图谱可视化支持 > 1000 个节点
- [ ] 订阅源健康度评估准确率 > 85%

### 预估工作量

- **后端开发**: 15-20 工作日
- **知识图谱引擎**: 8-10 工作日
- **AI 模型训练和调优**: 7-10 工作日
- **前端开发**: 10-12 工作日
- **测试和优化**: 5-7 工作日
- **总计**: 约 7-9 周

---

## 5. 数据库设计

### 5.1 完整 ERD

```
users (现有表)
  ↓
rss_subscriptions ──────┐
  ↓                      │
rss_articles             │ (仅用于去重)
  (订阅源的文章)          │
                         │
quka_space ←─────────────┤
  ↓                      │
quka_resource ←──────────┤ (控制过期策略, cycle 字段)
  ↓                      │
quka_knowledge ←─────────┘ (存储用户的 RSS 文章内容)
  (kind='rss')
  ↓
rss_user_interests (用户兴趣模型, 用于推荐)
  ↓
rss_alert_rules (提醒规则)
  ↓
rss_alerts (提醒历史)
  ↓
knowledge_entities (第三阶段: 知识图谱)
  ↓
knowledge_relations (第三阶段: 实体关系)
  ↓
learning_cards (第三阶段: 学习卡片)
  ↓
spaced_repetition_logs (第三阶段: 间隔复习)
```

**核心设计说明：**

1. **RSS 订阅流程**
   ```
   用户创建订阅 → 指定 space_id + resource_id
                  ↓
   系统抓取文章 → 保存到 rss_articles (去重)
                  ↓
   为用户创建 → quka_knowledge 记录
                  - resource: 用户指定的 resource
                  - expired_at: 根据 resource.cycle 计算
   ```

2. **核心表职责**
   - `rss_subscriptions`: 订阅配置（URL, space, resource, 更新频率）
   - `rss_articles`: 文章去重（防止重复抓取）
   - `quka_knowledge`: 用户的 RSS 内容（kind='rss'）
   - `quka_resource`: 过期策略配置（cycle 天数）
   - `rss_user_interests`: 用户兴趣权重（用于推荐排序）

3. **数据关系**
   - 一个订阅 → 多篇文章 (rss_subscriptions → rss_articles)
   - 一篇文章 → 多个用户的 knowledge (rss_articles → quka_knowledge)
   - 一个 resource → 多篇 knowledge (quka_resource → quka_knowledge)
   - 一个用户 → 多个兴趣 (users → rss_user_interests)

### 5.2 数据迁移脚本

所有数据库迁移脚本应放在 `db/migrations/` 目录下，命名格式：

```
YYYYMMDDHHMMSS_description.up.sql
YYYYMMDDHHMMSS_description.down.sql
```

例如：

```
20251210000001_create_rss_tables.up.sql
20251210000001_create_rss_tables.down.sql
```

### 5.3 性能优化建议

1. **分区表**: 对于 `rss_articles` 表，按时间范围分区

   ```sql
   CREATE TABLE rss_articles_2024 PARTITION OF rss_articles
   FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
   ```

2. **索引优化**:

   - 为常用查询字段添加复合索引
   - 使用 GIN 索引处理数组字段（如 keywords）
   - 使用 pgvector 索引处理向量检索

3. **数据归档**:
   - 6 个月以上的文章归档到历史表
   - 保留热数据在主表，提升查询性能

---

## 6. API 设计

### 6.1 API 版本控制

所有 RSS 相关 API 使用 `/api/v1/rss` 前缀。

### 6.2 认证和鉴权

- 所有 API 需要 JWT 认证
- 用户只能访问自己的订阅和文章数据

### 6.3 分页规范

```json
{
  "page": 1,
  "page_size": 20,
  "total": 150,
  "data": [...]
}
```

### 6.4 错误码规范

```
400001 - RSS URL 格式错误
400002 - RSS 源无法访问
400003 - RSS 格式解析失败
404001 - 订阅源不存在
404002 - 文章不存在
429001 - 请求过于频繁
```

### 6.5 完整 API 列表

参见各阶段的 API 端点设计部分。

### 6.6 WebSocket 协议

用于实时推送新文章和智能提醒：

```javascript
// 连接
ws://api.qukaai.com/ws/rss?token=<jwt_token>

// 接收消息格式
{
  "type": "new_article",
  "data": {
    "article_id": 12345,
    "title": "...",
    "summary": "...",
    "relevance_score": 0.85
  }
}

{
  "type": "alert",
  "data": {
    "alert_id": 67890,
    "reason": "与你最近的聊天高度相关",
    "article": {...}
  }
}
```

---

## 7. 关键技术点

### 7.1 RSS 解析

#### 支持的格式

- RSS 2.0
- RSS 1.0
- Atom 1.0
- JSON Feed

#### 推荐库

```go
import (
    "github.com/mmcdole/gofeed"
)

func ParseRSS(url string) (*gofeed.Feed, error) {
    fp := gofeed.NewParser()
    feed, err := fp.ParseURL(url)
    return feed, err
}
```

### 7.2 定时任务调度

使用 `robfig/cron` 进行任务调度：

```go
import (
    "github.com/robfig/cron/v3"
)

func StartScheduler() {
    c := cron.New()

    // 每小时抓取一次高频订阅
    c.AddFunc("0 * * * *", func() {
        FetchHighFrequencyFeeds()
    })

    // 每天凌晨 2 点抓取低频订阅
    c.AddFunc("0 2 * * *", func() {
        FetchLowFrequencyFeeds()
    })

    // 每天早上 8 点生成每日摘要
    c.AddFunc("0 8 * * *", func() {
        GenerateDailyDigest()
    })

    c.Start()
}
```

### 7.3 内容去重

使用 SimHash 算法进行文本去重：

```go
func IsDuplicate(newArticle Article, existingArticles []Article) bool {
    newHash := SimHash(newArticle.Content)

    for _, existing := range existingArticles {
        existingHash := SimHash(existing.Content)
        distance := HammingDistance(newHash, existingHash)

        // 汉明距离小于阈值认为是重复
        if distance < 3 {
            return true
        }
    }

    return false
}
```

### 7.4 AI 服务集成

复用现有的 `pkg/ai` 模块：

```go
import (
    "qukaai/pkg/ai"
)

func GenerateSummary(article Article) (string, error) {
    prompt := fmt.Sprintf(
        "请为以下文章生成一段 100 字左右的摘要：\n\n标题：%s\n\n内容：%s",
        article.Title,
        article.Content,
    )

    resp, err := ai.Complete(prompt, ai.CompletionOptions{
        Model:       "gpt-4",
        MaxTokens:   200,
        Temperature: 0.3,
    })

    return resp.Content, err
}
```

### 7.5 向量化和检索

使用现有的 pgvector 能力：

```go
func VectorizeArticle(article Article) error {
    // 1. 生成文章的向量表示
    embedding, err := ai.GenerateEmbedding(article.Content)
    if err != nil {
        return err
    }

    // 2. 存储到向量数据库
    query := `
        INSERT INTO knowledge_embeddings (
            user_id, source_type, source_id, embedding
        ) VALUES ($1, 'rss', $2, $3)
    `

    _, err = db.Exec(query, article.UserID, article.ID, embedding)
    return err
}

func SearchSimilarArticles(queryText string, limit int) ([]Article, error) {
    // 1. 查询文本向量化
    queryEmbedding, err := ai.GenerateEmbedding(queryText)
    if err != nil {
        return nil, err
    }

    // 2. 向量检索
    query := `
        SELECT a.*
        FROM rss_articles a
        JOIN knowledge_embeddings e ON e.source_id = a.id AND e.source_type = 'rss'
        ORDER BY e.embedding <-> $1
        LIMIT $2
    `

    var articles []Article
    err = db.Select(&articles, query, queryEmbedding, limit)
    return articles, err
}
```

### 7.6 缓存策略

使用 Redis 进行多级缓存：

```go
// 缓存层次
// L1: 文章列表（5 分钟）
// L2: 文章详情（30 分钟）
// L3: 摘要和关键词（永久，直到文章更新）

func GetArticleList(userID int64, page int) ([]Article, error) {
    cacheKey := fmt.Sprintf("rss:articles:%d:page:%d", userID, page)

    // 尝试从缓存获取
    var articles []Article
    err := redis.Get(cacheKey, &articles)
    if err == nil {
        return articles, nil
    }

    // 缓存未命中，从数据库获取
    articles, err = db.GetArticleList(userID, page)
    if err != nil {
        return nil, err
    }

    // 写入缓存
    redis.Set(cacheKey, articles, 5*time.Minute)

    return articles, nil
}
```

---

## 8. 风险和挑战

### 8.1 技术风险

| 风险               | 影响 | 概率 | 缓解措施                          |
| ------------------ | ---- | ---- | --------------------------------- |
| RSS 源格式多样性   | 高   | 高   | 使用成熟的解析库，建立格式测试集  |
| AI 摘要质量不稳定  | 中   | 中   | 建立评估体系，持续优化 prompt     |
| 向量检索性能瓶颈   | 高   | 中   | 使用 HNSW 索引，分片存储          |
| 定时任务大规模调度 | 中   | 低   | 使用分布式任务队列（如 RabbitMQ） |
| 存储成本增长快     | 中   | 高   | 实施数据归档策略，压缩历史数据    |

### 8.2 产品风险

| 风险                 | 影响 | 概率 | 缓解措施                       |
| -------------------- | ---- | ---- | ------------------------------ |
| 用户不认可 AI 推荐   | 高   | 中   | A/B 测试，提供手动控制选项     |
| 提醒过于频繁打扰用户 | 中   | 高   | 智能频率控制，用户可自定义     |
| 隐私担忧             | 高   | 低   | 明确数据使用政策，本地优先处理 |
| 与现有功能冲突       | 中   | 中   | 与产品团队充分沟通，设计一致   |

### 8.3 运营风险

| 风险                 | 影响 | 概率 | 缓解措施                   |
| -------------------- | ---- | ---- | -------------------------- |
| AI API 成本过高      | 高   | 中   | 实施成本控制，使用本地模型 |
| 第三方 RSS 源不稳定  | 中   | 高   | 重试机制，健康度监控       |
| 法律合规问题（版权） | 高   | 低   | 仅存储摘要，链接到原文     |

---

## 9. 测试计划

### 9.1 单元测试

覆盖核心功能模块：

- RSS 解析器（各种格式）
- 内容去重算法
- 评分算法
- 定时任务调度

**目标覆盖率**: > 80%

### 9.2 集成测试

测试各模块协作：

- RSS 抓取 → 存储 → 向量化 → 检索 全流程
- 用户订阅 → 文章推送 → 阅读 → 反馈 循环
- 智能提醒触发和推送

### 9.3 性能测试

| 测试场景               | 目标指标 |
| ---------------------- | -------- |
| 单个 RSS 源抓取        | < 5s     |
| 100 个 RSS 源并发抓取  | < 60s    |
| 文章列表加载（100 条） | < 1s     |
| 向量检索（1000 万条）  | < 500ms  |
| 对话式查询             | < 3s     |
| WebSocket 消息延迟     | < 100ms  |

### 9.4 压力测试

- 模拟 10000 并发用户
- 每用户 50 个订阅源
- 每日新增文章 100 万条

### 9.5 用户验收测试（UAT）

邀请 50-100 名内测用户，收集反馈：

- AI 推荐准确性
- 智能提醒有用性
- UI/UX 体验
- 性能和稳定性

---

## 10. 未来扩展

### 10.1 多模态内容支持

- 视频 RSS（YouTube、Bilibili）
- 播客 RSS（自动转录）
- 图片流（Instagram、Pinterest）

### 10.2 社交功能

- 关注其他用户的公开订阅
- 共享阅读列表
- 文章讨论区

### 10.3 移动端优化

- 离线阅读
- 推送通知
- 语音播报

### 10.4 企业版功能

- 团队协作
- 竞品监控
- 舆情分析
- 合规管理

### 10.5 第三方集成

- 与 Notion、Obsidian 等笔记工具集成
- 导出到 Pocket、Instapaper
- 与日历应用集成（基于文章提醒）

---

## 附录

### A. 参考文档

- [RSS 2.0 规范](https://www.rssboard.org/rss-specification)
- [Atom 1.0 规范](https://datatracker.ietf.org/doc/html/rfc4287)
- [pgvector 文档](https://github.com/pgvector/pgvector)
- [OpenAI Embeddings API](https://platform.openai.com/docs/guides/embeddings)

### B. 竞品分析

- **Feedly**: 成熟的 RSS 阅读器，有 AI 功能但不深入
- **Inoreader**: 强大的过滤和自动化规则
- **Readwise Reader**: 与学习系统集成较好
- **QukaAI 的差异化**: 更深度的 AI 集成 + 知识库融合

### C. 术语表

- **RSS**: Really Simple Syndication，简易信息聚合
- **RAG**: Retrieval-Augmented Generation，检索增强生成
- **SimHash**: 一种局部敏感哈希算法，用于文本去重
- **HNSW**: Hierarchical Navigable Small World，向量检索算法
- **SM-2**: SuperMemo 2 算法，间隔重复学习算法

---

## 变更日志

| 日期       | 版本 | 变更内容                 | 作者   |
| ---------- | ---- | ------------------------ | ------ |
| 2025-12-10 | 1.0  | 初始版本，完成三阶段规划 | Claude |

---

## 待确认问题

1. 是否需要支持 OPML 格式的订阅源导入/导出？
2. AI 摘要是否需要支持多语言输出（例如中文文章生成英文摘要）？
3. 用户数据保留策略：文章保留多久？是否需要手动归档？
4. 是否需要内置一些推荐的 RSS 源？
5. 对于付费墙内容（如 Medium 会员文章），如何处理？
6. 是否需要支持 Newsletter 订阅（邮件转 RSS）？
7. 移动端是使用 React Native 还是原生开发？
8. 第一阶段完成后是否先上线 Beta 版本收集反馈？

---

**文档结束**
