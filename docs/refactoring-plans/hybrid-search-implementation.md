# 倒排索引 + 混合检索实现方案

> **创建时间**: 2026-02-24
> **更新时间**: 2026-02-24
> **状态**: 设计中
> **优先级**: 高
> **默认模式**: 是
> **存量数据**: 功能上线后仅新数据支持

---

## 需求确认

- ✅ **多语言支持**: 支持中文分词（使用 zhparser 扩展）
- ✅ **BM25 索引范围**: 仅 `chunk` 字段，不含 `title`
- ✅ **上线模式**: 作为默认模式
- ✅ **存量数据**: 功能上线后仅新数据支持，存量数据可选迁移
- ✅ **BM25 实现**: 使用 PostgreSQL 内置 `ts_rank()`，无需额外插件（除中文分词扩展）

---

## 一、背景与目标

### 1.1 当前系统现状

当前 QukaAI 的 RAG 召回体系完全依赖 **pgvector 向量检索**，存在以下局限性：

| 问题 | 描述 | 影响 |
|------|------|------|
| **语义泛化** | 向量检索擅长语义匹配，但对精确关键词匹配能力弱 | 搜索 "PostgreSQL" 可能召回 "MySQL" 相关内容 |
| **专业术语** | 技术术语、人名、代码片段等在向量空间中分布不均匀 | 专业词汇召回效果差 |
| **长尾内容** | 低频但重要的内容向量质量可能不佳 | 关键信息可能漏召回 |
| **黑盒问题** | 无法解释为什么某些内容被召回 | 用户体验和调试困难 |

### 1.2 混合检索优势

混合检索（Hybrid Search）结合 **Dense Retrieval（向量）** 和 **Sparse Retrieval（BM25）**：

```
┌─────────────────────────────────────────────────────────────┐
│                    混合检索架构                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   用户查询                                                   │
│      │                                                      │
│      ├─────────────────────────────────────────────────┐    │
│      ▼                                                 ▼    │
│  ┌─────────────┐                                  ┌──────────┐│
│  │ 向量检索     │                                  │ BM25检索  ││
│  │ (Dense)     │                                  │ (Sparse) ││
│  │             │                                  │          ││
│  │ • 语义匹配   │                                  │ • 关键词  ││
│  │ • 余弦相似度 │                                  │ • 精确匹配 ││
│  │ • Top-K     │                                  │ • 词频权重 ││
│  └─────────────┘                                  └──────────┘│
│      │                                                 │     │
│      └─────────────────────────────────────────────────┘     │
│                       ▼                                       │
│              ┌──────────────┐                                │
│              │  RRF 融合     │                                │
│              │  (可选加权)   │                                │
│              └──────────────┘                                │
│                       │                                       │
│                       ▼                                       │
│              ┌──────────────┐                                │
│              │  Rerank      │                                │
│              │  (AI重排序)   │                                │
│              └──────────────┘                                │
└─────────────────────────────────────────────────────────────┘
```

**核心收益**：
- ✅ **互补优势**: 向量捕获语义，BM25 保证精确匹配
- ✅ **召回提升**: RRF 融合后召回率通常提升 15-30%
- ✅ **可解释性**: BM25 分数可解释性更强
- ✅ **低耦合**: PostgreSQL 原生支持，无需额外组件

---

## 二、技术方案设计

### 2.1 数据库层改造

#### 2.1.1 中文分词扩展安装

**重要**: PostgreSQL 默认不支持中文分词，需要安装 `zhparser` 扩展。

##### 安装步骤

**Ubuntu/Debian**:
```bash
# 安装 zhparser 扩展
sudo apt-get install postgresql-14-zhparser

# 或从源码编译
git clone https://github.com/amutu/zhparser.git
cd zhparser
make && sudo make install
```

**CentOS/RHEL**:
```bash
# 从源码编译
git clone https://github.com/amutu/zhparser.git
cd zhparser
make && sudo make install
```

**Docker 环境**:
```dockerfile
FROM postgres:14
RUN apt-get update && apt-get install -y postgresql-14-zhparser
```

##### 配置中文分词

```sql
-- 1. 创建扩展
CREATE EXTENSION IF NOT EXISTS zhparser;

-- 2. 创建中文文本搜索配置
CREATE TEXT SEARCH CONFIGURATION chinese_zh (PARSER = zhparser);

-- 3. 添加 token 类型映射
ALTER TEXT SEARCH CONFIGURATION chinese_zh
  ADD MAPPING FOR a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z
  WITH simple;

-- 4. 验证分词效果
SELECT to_tsquery('chinese_zh', '北京欢迎使用');
-- 预期输出: '北京' & '欢迎' & '使用'
```

#### 2.1.2 添加全文检索列

**表**: `quka_knowledge_chunk`

```sql
-- 添加全文检索列
ALTER TABLE quka_knowledge_chunk ADD COLUMN IF NOT EXISTS content_tsv tsvector;

-- 添加注释
COMMENT ON COLUMN quka_knowledge_chunk.content_tsv IS '全文检索向量，用于 BM25 检索（支持中文分词）';

-- 创建 GIN 索引（高性能全文检索）
CREATE INDEX IF NOT EXISTS idx_knowledge_chunk_content_tsv
ON quka_knowledge_chunk USING GIN(content_tsv);

-- 创建自动更新触发器（使用中文分词配置）
CREATE OR REPLACE FUNCTION knowledge_chunk_tsv_trigger() RETURNS trigger AS $$
BEGIN
  -- 优先使用中文分词，回退到 simple
  BEGIN
    NEW.content_tsv := setweight(to_tsvector('chinese_zh', COALESCE(NEW.chunk, '')), 'A');
  EXCEPTION WHEN undefined_function THEN
    -- 如果中文配置不可用，使用 simple
    NEW.content_tsv := setweight(to_tsvector('pg_catalog.simple', COALESCE(NEW.chunk, '')), 'A');
  END;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- 删除旧触发器（如果存在）
DROP TRIGGER IF EXISTS tsvector_update ON quka_knowledge_chunk;

-- 创建新触发器
CREATE TRIGGER tsvector_update BEFORE INSERT OR UPDATE
ON quka_knowledge_chunk FOR EACH ROW
EXECUTE FUNCTION knowledge_chunk_tsv_trigger();
```

**设计说明**：

| 配置项 | 选择 | 理由 |
|--------|------|------|
| **中文分词** | `chinese_zh` (zhparser) | 专门为中文设计，支持多语言 |
| **降级方案** | `pg_catalog.simple` | 扩展不可用时回退 |
| **权重** | 全部 'A' | chunk 内容重要性一致 |
| **索引类型** | GIN | 全文检索标准索引，查询性能优秀 |

#### 2.1.3 BM25 算法说明

✅ **BM25 不需要额外插件**

PostgreSQL 的 `ts_rank()` 函数内置了 BM25 算法实现：

```sql
-- ts_rank 使用默认 BM25 参数
-- 计算公式: BM25 = Σ IDF(qi) × (f(qi,D) × (k1 + 1)) / (f(qi,D) + k1 × (1 - b + b × |D| / avgdl))
-- 其中:
--   - f(qi,D): 词 qi 在文档 D 中的频率
--   - |D|: 文档长度
--   - avgdl: 平均文档长度
--   - k1 = 1.2 (PostgreSQL 默认)
--   - b = 0.75 (PostgreSQL 默认)

-- 查询示例
SELECT
  id,
  chunk,
  ts_rank(content_tsv, plainto_tsquery('chinese_zh', 'PostgreSQL 数据库')) as bm25_score
FROM quka_knowledge_chunk
WHERE content_tsv @@ plainto_tsquery('chinese_zh', 'PostgreSQL 数据库')
ORDER BY bm25_score DESC
LIMIT 10;
```

**仅需要 `pgvector` 用于向量检索**，BM25 完全依赖 PostgreSQL 内置功能。

#### 2.1.4 兼容性方案（可选）

如果希望避免修改现有表，可创建**物化视图**：

```sql
-- 创建物化视图（适合存量数据迁移）
CREATE MATERIALIZED VIEW IF NOT EXISTS quka_knowledge_chunk_fts AS
SELECT
  id, knowledge_id, space_id, user_id,
  chunk, original_length, updated_at, created_at,
  setweight(to_tsvector('chinese_zh', chunk), 'A') as content_tsv
FROM quka_knowledge_chunk;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_knowledge_chunk_fts_tsv
ON quka_knowledge_chunk_fts USING GIN(content_tsv);

-- 唯一索引（用于刷新）
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_chunk_fts_id
ON quka_knowledge_chunk_fts(id);

-- 定期刷新（可配置定时任务）
REFRESH MATERIALIZED VIEW CONCURRENTLY quka_knowledge_chunk_fts;
```

---

### 2.2 Store 层实现

#### 2.2.1 新增 BM25 检索方法

**文件**: `app/store/sqlstore/knowledge_chunk.go`

```go
// FullTextSearch BM25 全文检索（支持中文分词）
func (s *KnowledgeChunkStore) FullTextSearch(
    ctx context.Context,
    opts types.GetKnowledgeChunkOptions,
    queryText string,
    limit uint64,
) ([]types.BM25Result, error) {
    // 尝试使用中文分词配置，如果不可用则回退到 simple
    tsConfig := s.getTSConfig(ctx)

    // PostgreSQL BM25 查询
    // ts_rank 基于 BM25 算法计算相关性
    sqlQuery := sq.Select(
        "id",
        "knowledge_id",
        "original_length",
        fmt.Sprintf("ts_rank(content_tsv, plainto_tsquery('%s', ?)) as score", tsConfig),
    ).From(s.GetTable()).
        Where(fmt.Sprintf("content_tsv @@ plainto_tsquery('%s', ?)", tsConfig)).
        Limit(limit).
        OrderBy("score DESC")

    opts.Apply(&sqlQuery)

    queryString, args, err := sqlQuery.ToSql()
    if err != nil {
        return nil, ErrorSqlBuild(err)
    }

    var results []types.BM25Result
    if err = s.GetReplica(ctx).Select(&results, queryString, args...); err != nil {
        return nil, err
    }

    return results, nil
}
```

**新增类型定义**：

**文件**: `pkg/types/knowledge_chunk.go`

```go
// BM25Result BM25 检索结果
type BM25Result struct {
    ID             string  `json:"id" db:"id"`
    KnowledgeID    string  `json:"knowledge_id" db:"knowledge_id"`
    OriginalLength int     `json:"original_length" db:"original_length"`
    Score          float32 `json:"score" db:"score"` // BM25 分数
}

// GetKnowledgeChunkOptions 知识分片查询选项
type GetKnowledgeChunkOptions struct {
    SpaceID  string
    UserID   string
    Resource *ResourceQuery
}

func (opts GetKnowledgeChunkOptions) Apply(query *sq.SelectBuilder) {
    if opts.SpaceID != "" {
        *query = query.Where(sq.Eq{"space_id": opts.SpaceID})
    }
    if opts.UserID != "" {
        *query = query.Where(sq.Eq{"user_id": opts.UserID})
    }
    if opts.Resource != nil {
        *query = query.Where(opts.Resource.ToQuery())
    }
}
```

**辅助方法**（检测中文分词配置）：

```go
// getTSConfig 获取可用的文本搜索配置（优先使用中文分词）
func (s *KnowledgeChunkStore) getTSConfig(ctx context.Context) string {
    // 检查 chinese_zh 配置是否可用
    var count int
    err := s.GetReplica(ctx).Get(&count,
        `SELECT COUNT(*) FROM pg_ts_config WHERE cfgname = 'chinese_zh'`)

    if err == nil && count > 0 {
        return "chinese_zh"
    }
    return "pg_catalog.simple" // 回退到默认配置
}
```

---

### 2.3 RAG 层混合检索实现

#### 2.3.1 RRF 融合算法

**文件**: `pkg/ai/agents/rag/hybrid.go` (新建)

```go
package rag

import (
    "sort"

    "github.com/quka-ai/quka-ai/pkg/types"
)

// RRFConst Reciprocal Rank Fusion 常数 K
const RRFConst = 60

// FusionResult 融合结果
type FusionResult struct {
    KnowledgeID    string
    VectorScore    float32 // 向量余弦相似度
    BM25Score      float32 // BM25 分数
    FusionScore    float32 // 融合后分数
}

// RRF Reciprocal Rank Fusion 融合算法
func RRF(vectorResults []types.QueryResult, bm25Results []types.BM25Result) []FusionResult {
    // 构建分数映射
    vectorScores := make(map[string]float32)
    for i, result := range vectorResults {
        // RRF: K / (K + rank)
        vectorScores[result.KnowledgeID] = RRFConst / (RRFConst + float32(i+1))
    }

    bm25Scores := make(map[string]float32)
    for i, result := range bm25Results {
        bm25Scores[result.KnowledgeID] = RRFConst / (RRFConst + float32(i+1))
    }

    // 合并所有 knowledge_id
    allIDs := make(map[string]bool)
    for _, result := range vectorResults {
        allIDs[result.KnowledgeID] = true
    }
    for _, result := range bm25Results {
        allIDs[result.KnowledgeID] = true
    }

    // 计算融合分数
    var fusionResults []FusionResult
    for id := range allIDs {
        vectorScore := vectorScores[id]
        bm25Score := bm25Scores[id]

        fusionResults = append(fusionResults, FusionResult{
            KnowledgeID: id,
            VectorScore: vectorScore,
            BM25Score:   bm25Score,
            FusionScore: vectorScore + bm25Score,
        })
    }

    // 按融合分数降序排序
    sort.Slice(fusionResults, func(i, j int) bool {
        return fusionResults[i].FusionScore > fusionResults[j].FusionScore
    })

    return fusionResults
}

// WeightedRRF 加权 RRF 融合（可选）
func WeightedRRF(
    vectorResults []types.QueryResult,
    bm25Results []types.BM25Result,
    vectorWeight float32, // 向量权重 (0-1)
    bm25Weight float32,   // BM25 权重 (0-1)
) []FusionResult {
    // 计算排名分数
    vectorScores := make(map[string]float32)
    for i, result := range vectorResults {
        vectorScores[result.KnowledgeID] = RRFConst / (RRFConst + float32(i+1))
    }

    bm25Scores := make(map[string]float32)
    for i, result := range bm25Results {
        bm25Scores[result.KnowledgeID] = RRFConst / (RRFConst + float32(i+1))
    }

    // 合并并计算加权分数
    allIDs := make(map[string]bool)
    for _, result := range vectorResults {
        allIDs[result.KnowledgeID] = true
    }
    for _, result := range bm25Results {
        allIDs[result.KnowledgeID] = true
    }

    var fusionResults []FusionResult
    for id := range allIDs {
        vectorScore := vectorScores[id]
        bm25Score := bm25Scores[id]

        fusionResults = append(fusionResults, FusionResult{
            KnowledgeID: id,
            VectorScore: vectorScore,
            BM25Score:   bm25Score,
            FusionScore: vectorScore*vectorWeight + bm25Score*bm25Weight,
        })
    }

    sort.Slice(fusionResults, func(i, j int) bool {
        return fusionResults[i].FusionScore > fusionResults[j].FusionScore
    })

    return fusionResults
}
```

#### 2.3.2 混合检索主函数

**文件**: `pkg/ai/agents/rag/rag.go` (修改)

```go
// GetQueryRelevanceKnowledgesWithHybrid 混合检索版本
func GetQueryRelevanceKnowledgesWithHybrid(
    core *core.Core,
    spaceID, userID, query string,
    resource *types.ResourceQuery,
) (types.RAGDocs, []ai.UsageItem, error) {
    var (
        result types.RAGDocs
        usages []ai.UsageItem
    )

    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()

    // 并行执行向量检索和 BM25 检索
    var (
        vectorResults []types.QueryResult
        bm25Results   []types.BM25Result
        vectorErr     error
        bm25Err       error
        wg            sync.WaitGroup
    )

    wg.Add(2)

    // 向量检索
    go func() {
        defer wg.Done()
        vector, err := core.Srv().AI().EmbeddingForQuery(ctx, []string{query})
        if err != nil || len(vector.Data) == 0 {
            vectorErr = fmt.Errorf("failed to get embedding for query: %w", err)
            return
        }

        vectorResults, vectorErr = core.Store().VectorStore().Query(
            ctx,
            types.GetVectorsOptions{
                SpaceID:  spaceID,
                UserID:   userID,
                Resource: resource,
            },
            pgvector.NewVector(vector.Data[0]),
            50, // 混合检索时减少向量召回数量
        )
    }()

    // BM25 检索
    go func() {
        defer wg.Done()
        bm25Results, bm25Err = core.Store().KnowledgeChunkStore().FullTextSearch(
            ctx,
            types.GetKnowledgeChunkOptions{
                SpaceID:  spaceID,
                UserID:   userID,
                Resource: resource,
            },
            query,
            50, // BM25 召回数量
        )
    }()

    wg.Wait()

    // 处理错误（允许部分失败）
    if vectorErr != nil {
        slog.Error("vector search failed", slog.String("error", vectorErr.Error()))
    }
    if bm25Err != nil {
        slog.Error("bm25 search failed", slog.String("error", bm25Err.Error()))
    }

    // 如果都失败，返回错误
    if vectorErr != nil && bm25Err != nil {
        return types.RAGDocs{}, nil, fmt.Errorf("both vector and bm25 search failed")
    }

    // RRF 融合
    fusionResults := RRF(vectorResults, bm25Results)

    // 取 Top-N 融合结果
    topN := 20
    if len(fusionResults) < topN {
        topN = len(fusionResults)
    }
    fusionResults = fusionResults[:topN]

    // 获取知识详情
    var knowledgeIDs []string
    for _, v := range fusionResults {
        knowledgeIDs = append(knowledgeIDs, v.KnowledgeID)
    }

    knowledges, err := core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
        IDs:      knowledgeIDs,
        SpaceID:  spaceID,
        UserID:   userID,
        Resource: resource,
    }, 1, 100)
    // ... 后续处理逻辑与原版一致
}
```

---

### 2.4 配置层支持

**文件**: `pkg/config/hybrid_search.go` (新建)

```go
package config

type HybridSearchConfig struct {
    // 是否启用混合检索（默认开启）
    Enabled bool `toml:"enabled" json:"enabled"`

    // 向量检索权重 (0-1)
    VectorWeight float32 `toml:"vector_weight" json:"vector_weight"`

    // BM25 检索权重 (0-1)
    BM25Weight float32 `toml:"bm25_weight" json:"bm25_weight"`

    // RRF 常数 K
    RRFK int `toml:"rrf_k" json:"rrf_k"`

    // 向量召回数量
    VectorTopK int `toml:"vector_top_k" json:"vector_top_k"`

    // BM25 召回数量
    BM25TopK int `toml:"bm25_top_k" json:"bm25_top_k"`

    // 最终返回数量
    FinalTopK int `toml:"final_top_k" json:"final_top_k"`

    // 文本搜索配置（默认使用中文分词）
    TSConfig string `toml:"ts_config" json:"ts_config"`
}

func DefaultHybridSearchConfig() HybridSearchConfig {
    return HybridSearchConfig{
        Enabled:      true,  // 默认开启
        VectorWeight: 0.5,
        BM25Weight:   0.5,
        RRFK:         60,
        VectorTopK:   50,
        BM25TopK:     50,
        FinalTopK:    20,
        TSConfig:     "chinese_zh", // 优先使用中文分词
    }
}
```

---

## 三、监控指标

```go
// pkg/monitor/metrics.go (新建)
package monitor

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 混合检索延迟
    hybridSearchDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "rag_hybrid_search_duration_seconds",
            Help:    "Hybrid search latency",
            Buckets: []float64{0.05, 0.1, 0.2, 0.5, 1.0},
        },
        []string{"method"}, // vector, bm25, fusion
    )

    // 召回结果数量
    hybridSearchResults = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "rag_hybrid_search_results_count",
            Help:    "Number of results returned",
            Buckets: []float64{0, 5, 10, 20, 50, 100},
        },
        []string{"method"},
    )

    // 中文分词使用率
    chineseTokenUsage = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rag_chinese_tokenizer_usage_total",
            Help: "Chinese tokenizer usage count",
        },
        []string{"config"}, // chinese_zh, simple
    )
)
```

---

## 四、实施计划

### 4.1 阶段划分

| 阶段 | 任务 | 预计工作量 |
|------|------|-----------|
| **P0: 环境准备** | 安装 zhparser 扩展、配置中文分词 | 0.5 天 |
| **P0: 基础设施** | 数据库表结构改造、触发器 | 0.5 天 |
| **P0: Store 层** | BM25 检索实现、类型定义 | 1 天 |
| **P1: RAG 层** | RRF 融合、混合检索主函数 | 2 天 |
| **P1: 配置层** | 配置文件 | 0.5 天 |
| **P2: 测试** | 单元测试、集成测试、性能测试 | 2 天 |
| **P2: 监控** | Prometheus 指标、日志 | 0.5 天 |
| **P3: 优化** | 性能调优、参数配置 | 1 天 |
| **P3: 文档** | API 文档、使用指南 | 0.5 天 |

**总计**: 约 8 个工作日

### 4.2 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **zhparser 安装失败** | 无法使用中文分词 | 回退到 `simple` 配置 |
| **GIN 索引空间占用** | 存储增长 | 监控使用，可考虑 partial index |
| **查询延迟增加** | 用户体验受影响 | 并行执行 |
| **RRF 参数不理想** | 融合效果不达预期 | 可配置权重 |

### 4.3 回滚方案

- 保留原有 `GetQueryRelevanceKnowledges` 函数
- 出现问题可快速切换回纯向量检索

---

## 五、性能优化

### 5.1 查询优化

```sql
-- 1. 使用 partial index 减少索引大小（可选）
CREATE INDEX idx_knowledge_chunk_content_tsv_active
ON quka_knowledge_chunk USING GIN(content_tsv)
WHERE created_at > EXTRACT(EPOCH FROM TIMESTAMP '2024-01-01') * 1000;

-- 2. 考虑使用 rum 索引（更高级的全文检索索引，需额外扩展）
-- CREATE INDEX idx_knowledge_chunk_content_rum
-- ON quka_knowledge_chunk USING RUM(content_tsv);

-- 3. 合并查询（一次性获取 vector + bm25 结果）
-- 通过 UNION ALL 实现单次查询获取两种结果
```

### 5.2 缓存策略

```go
// 对热门查询的 BM25 结果进行缓存
type BM25Cache struct {
    cache *lru.Cache
}

func (c *BM25Cache) Get(query string) ([]types.BM25Result, bool) {
    if val, ok := c.cache.Get(query); ok {
        return val.([]types.BM25Result), true
    }
    return nil, false
}
```

### 5.3 性能指标

| 指标 | 目标值 | 监控方式 |
|------|--------|---------|
| **BM25 检索延迟** | < 50ms (p50) | Prometheus histogram |
| **混合检索总延迟** | < 200ms (p50) | 端到端追踪 |
| **召回率提升** | > 15% | 离线评估 |
| **存储增长** | < 30% | 数据库监控 |

---

## 六、测试方案

### 6.1 单元测试

```go
func TestRRF(t *testing.T) {
    vectorResults := []types.QueryResult{
        {KnowledgeID: "a", Cos: 0.9},
        {KnowledgeID: "b", Cos: 0.8},
        {KnowledgeID: "c", Cos: 0.7},
    }

    bm25Results := []types.BM25Result{
        {KnowledgeID: "b", Score: 10.0},
        {KnowledgeID: "d", Score: 9.0},
        {KnowledgeID: "a", Score: 8.0},
    }

    results := RRF(vectorResults, bm25Results)

    // 验证融合结果
    assert.Equal(t, "b", results[0].KnowledgeID) // b 在两个列表中都靠前
    assert.True(t, results[0].FusionScore > results[1].FusionScore)
}
```

### 6.2 集成测试

```go
func TestHybridSearchEndToEnd(t *testing.T) {
    // 准备测试数据
    core := setupTestCore(t)
    createTestKnowledges(t, core)

    // 执行混合检索
    docs, _, err := rag.GetQueryRelevanceKnowledgesWithHybrid(
        core, "test_space", "test_user", "PostgreSQL", nil,
    )

    assert.NoError(t, err)
    assert.NotEmpty(t, docs.Refs)
}
```

### 6.3 A/B 测试框架

```go
// 支持按用户或空间进行 A/B 测试
func ShouldUseHybridSearch(userID, spaceID string) bool {
    // 基于 hash 的一致性分流
    hash := fnv.New32()
    hash.Write([]byte(userID + spaceID))
    return hash.Sum32()%100 < 50 // 50% 流量
}
```

---

## 七、后续优化方向

### 7.1 中期优化

1. **Query Expansion**: 基于 BM25 结果扩展查询词
2. **动态权重**: 根据查询类型自动调整向量/BM25 权重
3. **学习排序**: 使用机器学习模型优化融合策略

### 7.2 长期优化

1. **多路召回**: 增加 Metadata 过滤、图关联等召回路径
2. **向量压缩**: 使用 Product Quantization 减少向量存储
3. **分布式检索**: 支持大规模数据量的分片检索

---

## 七、待确认问题

- [x] 支持多语言分词（zhparser 中文分词）
- [x] BM25 检索仅支持 `chunk` 字段，不含 `title`
- [x] 混合检索作为默认模式
- [x] 存量数据不迁移，功能上线后仅新数据支持
- [ ] RRF 参数是否需要支持在线动态调整？

---

## 九、相关文件清单

### 需要修改的文件

| 文件路径 | 修改类型 | 说明 |
|---------|---------|------|
| `app/store/sqlstore/knowledge_chunk.go` | 修改 | 新增 `FullTextSearch` 方法 |
| `app/store/sqlstore/knowledge_chunk.sql` | 新增 | 添加全文检索列和索引 |
| `pkg/ai/agents/rag/rag.go` | 修改 | 新增混合检索函数 |
| `pkg/ai/agents/rag/hybrid.go` | 新增 | RRF 融合算法 |
| `pkg/types/knowledge.go` | 修改 | 新增 `BM25Result` 类型 |
| `pkg/config/hybrid_search.go` | 新增 | 混合检索配置 |
| `scripts/migration/add_fulltext_search.sql` | 新增 | 数据库迁移脚本 |

### 新增测试文件

| 文件路径 | 说明 |
|---------|------|
| `app/store/sqlstore/knowledge_chunk_test.go` | BM25 检索测试 |
| `pkg/ai/agents/rag/hybrid_test.go` | RRF 融合测试 |
| `pkg/ai/agents/rag/integration_test.go` | 端到端测试 |

---

## 十、参考资料

- [PostgreSQL 全文检索文档](https://www.postgresql.org/docs/current/textsearch.html)
- [BM25 算法原理](https://en.wikipedia.org/wiki/Okapi_BM25)
- [Reciprocal Rank Fusion](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)
- [pgvector 使用指南](https://github.com/pgvector/pgvector)
- [混合检索最佳实践](https://www.pinecone.io/learn/hybrid-search-intro/)
