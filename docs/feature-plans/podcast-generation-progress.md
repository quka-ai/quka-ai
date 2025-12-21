# Podcast 生成进度追踪功能设计

## 背景

当前 podcast 生成过程中，前端无法获知具体的生成进度。Volcengine TTS 服务通过 WebSocket 以 round 为单位生成音频，每个 round 代表一段对话。我们需要添加进度字段，让前端能够实时展示生成进度。

## 目标

1. 在 `quka_podcasts` 表中添加生成进度相关字段
2. 在 TTS 生成过程中实时更新进度
3. 前端可以通过轮询获取当前生成进度

## 设计方案

### 1. 数据库变更

在 `quka_podcasts` 表中添加以下字段：

```sql
-- 生成进度时间戳
generation_last_updated BIGINT DEFAULT 0  -- 进度最后更新时间戳
```

#### 字段说明：
- `generation_last_updated`: 进度最后更新时间戳，用于前端判断生成是否仍在进行中

#### 设计理念：
采用**最简化设计**，只通过一个时间戳字段来表示生成进度：
- 当 `status = 'processing'` 且 `generation_last_updated` 持续更新时，说明生成正在进行
- 前端通过比对当前时间与 `generation_last_updated` 的差值，判断是否卡死
- 避免复杂的进度百分比计算，减少字段冗余

### 2. 进度更新逻辑

#### 2.1 初始化阶段
- 当状态变为 `processing` 时，初始化进度时间戳：
  - `generation_last_updated = current_timestamp`

#### 2.2 Round 处理阶段
在 `pkg/ai/volcengine/voice/podcast.go` 中，每当收到 `PodcastRoundStart` 事件时：
- 更新 `generation_last_updated = current_timestamp`
- 通过回调函数将时间戳写入数据库

#### 2.3 完成阶段
- 当状态变为 `completed` 时，`generation_last_updated` 不再更新

### 3. 实施步骤

1. **数据库迁移**
   - 创建迁移脚本 `app/store/sqlstore/migrations/podcast_add_generation_progress.sql`
   - 添加 `generation_last_updated` 字段

2. **更新数据类型**
   - 在 `pkg/types/podcast.go` 中的 `Podcast` 结构体添加 `GenerationLastUpdated` 字段

3. **更新存储层**
   - 在 `app/store/sqlstore/podcast_store.go` 添加更新进度的方法：
     - `UpdateGenerationProgress(ctx, podcastID)` - 只更新时间戳

4. **修改生成逻辑**
   - 在 `app/logic/v1/process/podcast_consumer.go` 中添加回调函数
   - 在 `pkg/ai/volcengine/voice/podcast.go` 的 `Gen` 方法中每个 round 开始时调用回调

5. **API 响应**
   - 确保 API 返回的 Podcast 对象包含 `generation_last_updated` 字段
   - 前端可以通过轮询 `/api/v1/podcasts/:id` 获取最新时间戳

### 4. 技术方案

#### 4.1 回调函数设计

使用回调函数方式更新数据库：

```go
type ProgressCallback func(currentRound, totalRounds, progress int)

func (p *PodCaster) Gen(ctx context.Context, inputID, text string,
    flagUseHeadMusic, flagUseTailMusic bool,
    progressCallback ProgressCallback) (*Result, error)
```

优点：
- 清晰、可测试性好
- TTS 服务层不依赖数据库
- 灵活性高，可以在回调中做任何操作

### 5. 前端集成

前端轮询逻辑示例：

```typescript
// 每 3 秒轮询一次
const pollProgress = async (podcastId: string) => {
  const response = await fetch(`/api/v1/podcasts/${podcastId}`)
  const podcast = await response.json()

  if (podcast.status === 'processing') {
    const now = Date.now() / 1000  // 转换为秒
    const timeSinceLastUpdate = now - podcast.generation_last_updated

    if (timeSinceLastUpdate < 30) {
      // 最近 30 秒内有更新，显示"正在生成中..."
      showStatus('正在生成中...')
    } else {
      // 超过 30 秒没更新，可能卡住了
      showStatus('生成可能遇到问题，请稍候...')
    }
  } else if (podcast.status === 'completed') {
    // 生成完成
    clearInterval(pollingInterval)
    showCompleted()
  } else if (podcast.status === 'failed') {
    // 生成失败
    clearInterval(pollingInterval)
    showError(podcast.error_message)
  }
}
```

### 6. 注意事项

1. **性能考虑**: 每个 round 更新一次数据库，对于正常的 podcast 生成（通常 10-30 个 round），性能影响可忽略
2. **错误处理**: 如果更新失败，只记录日志，不影响 TTS 生成流程
3. **时间戳精度**: 使用 Unix 时间戳（秒），足够前端判断进度

### 7. 实施状态

- [x] 完成设计文档
- [x] 创建数据库迁移脚本
- [x] 更新类型定义
- [x] 更新存储层
- [x] 修改 TTS 生成逻辑
- [ ] 运行数据库迁移
- [ ] 测试验证
- [ ] 前端对接

## 相关文件

- `/app/store/sqlstore/podcast.sql` - 表定义
- `/pkg/types/podcast.go` - 类型定义
- `/app/store/sqlstore/podcast_store.go` - 存储层
- `/app/logic/v1/process/podcast_consumer.go` - 消费者逻辑
- `/pkg/ai/volcengine/voice/podcast.go` - TTS 服务
