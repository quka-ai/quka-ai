-- 用户兴趣模型表
CREATE TABLE IF NOT EXISTS quka_rss_user_interests (
    id BIGINT PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    weight FLOAT DEFAULT 1.0,
    source VARCHAR(50),
    last_updated_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(user_id, topic)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_user_interests_user_id ON quka_rss_user_interests(user_id);
CREATE INDEX IF NOT EXISTS idx_rss_user_interests_weight ON quka_rss_user_interests(user_id, weight DESC);

-- 字段注释
COMMENT ON TABLE quka_rss_user_interests IS '用户兴趣模型表';
COMMENT ON COLUMN quka_rss_user_interests.user_id IS '用户ID';
COMMENT ON COLUMN quka_rss_user_interests.topic IS '主题/话题';
COMMENT ON COLUMN quka_rss_user_interests.weight IS '兴趣权重 0.0-1.0';
COMMENT ON COLUMN quka_rss_user_interests.source IS 'explicit(用户明确表示)或implicit(行为推断)';
COMMENT ON COLUMN quka_rss_user_interests.last_updated_at IS '最后更新时间戳';
COMMENT ON COLUMN quka_rss_user_interests.created_at IS '创建时间戳';
