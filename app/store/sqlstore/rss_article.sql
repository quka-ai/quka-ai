-- RSS 文章表（用于去重）
CREATE TABLE IF NOT EXISTS quka_rss_articles (
    id BIGINT PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    guid VARCHAR(512) NOT NULL,
    title VARCHAR(512),
    link VARCHAR(1024),
    description TEXT,
    content TEXT,
    author VARCHAR(255),
    published_at BIGINT,
    fetched_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(subscription_id, guid)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_articles_subscription_id ON quka_rss_articles(subscription_id);
CREATE INDEX IF NOT EXISTS idx_rss_articles_published_at ON quka_rss_articles(published_at DESC);

-- 字段注释
COMMENT ON TABLE quka_rss_articles IS 'RSS文章表（用于去重）';
COMMENT ON COLUMN quka_rss_articles.subscription_id IS '所属订阅ID';
COMMENT ON COLUMN quka_rss_articles.guid IS 'RSS item guid（用于去重）';
COMMENT ON COLUMN quka_rss_articles.title IS '文章标题';
COMMENT ON COLUMN quka_rss_articles.link IS '文章链接';
COMMENT ON COLUMN quka_rss_articles.content IS '文章内容';
COMMENT ON COLUMN quka_rss_articles.author IS '作者';
COMMENT ON COLUMN quka_rss_articles.published_at IS '发布时间戳';
COMMENT ON COLUMN quka_rss_articles.fetched_at IS '抓取时间戳';
COMMENT ON COLUMN quka_rss_articles.created_at IS '创建时间戳';
