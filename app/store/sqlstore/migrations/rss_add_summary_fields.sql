-- Migration: 为 RSS 文章表添加 AI 摘要相关字段
-- Date: 2025-12-11
-- Description: 添加共享的 AI 摘要字段，所有订阅用户共享，节省 AI 成本

-- 添加摘要相关字段
ALTER TABLE quka_rss_articles
ADD COLUMN IF NOT EXISTS user_id VARCHAR(32),              -- 最初订阅该文章的用户ID
ADD COLUMN IF NOT EXISTS summary TEXT,
ADD COLUMN IF NOT EXISTS keywords TEXT[],
ADD COLUMN IF NOT EXISTS summary_generated_at BIGINT,
ADD COLUMN IF NOT EXISTS ai_model VARCHAR(128),
ADD COLUMN IF NOT EXISTS summary_retry_times INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_summary_error TEXT;

-- 创建索引（用于快速查询没有摘要的文章）
CREATE INDEX IF NOT EXISTS idx_rss_articles_summary
ON quka_rss_articles(subscription_id)
WHERE summary IS NULL OR summary = '';

-- 创建索引（用于查询需要重试的文章）
CREATE INDEX IF NOT EXISTS idx_rss_articles_summary_retry
ON quka_rss_articles(summary_retry_times)
WHERE summary IS NULL AND summary_retry_times < 3;

-- 添加字段注释
COMMENT ON COLUMN quka_rss_articles.user_id IS '最初订阅该文章的用户ID（用于Token使用量归属）';
COMMENT ON COLUMN quka_rss_articles.summary IS 'AI生成的摘要（所有订阅用户共享，节省成本）';
COMMENT ON COLUMN quka_rss_articles.keywords IS 'AI提取的关键词';
COMMENT ON COLUMN quka_rss_articles.summary_generated_at IS '摘要生成时间戳';
COMMENT ON COLUMN quka_rss_articles.ai_model IS '生成摘要使用的AI模型';
COMMENT ON COLUMN quka_rss_articles.summary_retry_times IS '摘要生成重试次数';
COMMENT ON COLUMN quka_rss_articles.last_summary_error IS '最后一次摘要生成错误信息';

-- 更新表注释
COMMENT ON TABLE quka_rss_articles IS 'RSS文章表（用于去重和共享摘要）';
