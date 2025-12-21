-- RSS 文章表（用于去重和共享摘要）
CREATE TABLE IF NOT EXISTS quka_rss_articles (
    id VARCHAR(32) PRIMARY KEY,
    subscription_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL DEFAULT '',                    -- 最初订阅该文章的用户ID（用于Token归属）
    guid VARCHAR(512) NOT NULL,
    title VARCHAR(512) NOT NULL DEFAULT '',
    link VARCHAR(1024) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    author VARCHAR(255) NOT NULL DEFAULT '',

    -- AI 生成的摘要（所有订阅用户共享）
    summary TEXT NOT NULL DEFAULT '',
    keywords TEXT[] NOT NULL DEFAULT '{}',
    summary_generated_at BIGINT NOT NULL DEFAULT 0,
    ai_model VARCHAR(128) NOT NULL DEFAULT '',

    -- 摘要生成重试相关
    summary_retry_times INT NOT NULL DEFAULT 0,     -- 摘要生成重试次数
    last_summary_error TEXT NOT NULL DEFAULT '',     -- 最后一次摘要生成错误

    published_at BIGINT NOT NULL DEFAULT 0,
    fetched_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(subscription_id, guid)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_articles_guid ON quka_rss_articles(guid);
CREATE INDEX IF NOT EXISTS idx_rss_articles_subscription_id ON quka_rss_articles(subscription_id);
CREATE INDEX IF NOT EXISTS idx_rss_articles_published_at ON quka_rss_articles(published_at DESC);

-- 字段注释
COMMENT ON TABLE quka_rss_articles IS 'RSS文章表（用于去重和共享摘要）';
COMMENT ON COLUMN quka_rss_articles.subscription_id IS '所属订阅ID';
COMMENT ON COLUMN quka_rss_articles.user_id IS '最初订阅该文章的用户ID（用于Token使用量归属），非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.guid IS 'RSS item guid（用于去重）';
COMMENT ON COLUMN quka_rss_articles.title IS '文章标题，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.link IS '文章链接，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.description IS '文章描述，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.content IS '文章内容，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.author IS '作者，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.summary IS 'AI生成的摘要（所有订阅用户共享，节省成本），非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.keywords IS 'AI提取的关键词，非空，默认空数组';
COMMENT ON COLUMN quka_rss_articles.summary_generated_at IS '摘要生成时间戳，非空，默认0';
COMMENT ON COLUMN quka_rss_articles.ai_model IS '生成摘要使用的AI模型，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.summary_retry_times IS '摘要生成重试次数，非空，默认0';
COMMENT ON COLUMN quka_rss_articles.last_summary_error IS '最后一次摘要生成错误信息，非空，默认空字符串';
COMMENT ON COLUMN quka_rss_articles.published_at IS '发布时间戳，非空，默认0';
COMMENT ON COLUMN quka_rss_articles.fetched_at IS '抓取时间戳';
COMMENT ON COLUMN quka_rss_articles.created_at IS '创建时间戳';
