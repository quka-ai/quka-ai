-- RSS 订阅源表
CREATE TABLE IF NOT EXISTS quka_rss_subscriptions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    space_id VARCHAR(32) NOT NULL,
    resource_id VARCHAR(32) NOT NULL,
    url VARCHAR(512) NOT NULL,
    title VARCHAR(255),
    description TEXT,
    category VARCHAR(100),
    update_frequency INT DEFAULT 3600,
    last_fetched_at BIGINT DEFAULT 0,
    enabled BOOLEAN DEFAULT TRUE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(user_id, url)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_subscriptions_user_space ON quka_rss_subscriptions(user_id, space_id);
CREATE INDEX IF NOT EXISTS idx_rss_subscriptions_resource ON quka_rss_subscriptions(resource_id);
CREATE INDEX IF NOT EXISTS idx_rss_subscriptions_enabled ON quka_rss_subscriptions(enabled);

-- 字段注释
COMMENT ON TABLE quka_rss_subscriptions IS 'RSS订阅源表';
COMMENT ON COLUMN quka_rss_subscriptions.user_id IS '用户ID';
COMMENT ON COLUMN quka_rss_subscriptions.space_id IS '空间ID';
COMMENT ON COLUMN quka_rss_subscriptions.resource_id IS '内容存储的resource ID';
COMMENT ON COLUMN quka_rss_subscriptions.url IS 'RSS源URL';
COMMENT ON COLUMN quka_rss_subscriptions.update_frequency IS '更新频率（秒）';
COMMENT ON COLUMN quka_rss_subscriptions.last_fetched_at IS '上次抓取时间戳';
COMMENT ON COLUMN quka_rss_subscriptions.enabled IS '是否启用';
COMMENT ON COLUMN quka_rss_subscriptions.created_at IS '创建时间戳';
COMMENT ON COLUMN quka_rss_subscriptions.updated_at IS '更新时间戳';
