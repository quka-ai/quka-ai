-- RSS 每日摘要表
CREATE TABLE IF NOT EXISTS quka_rss_daily_digests (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    space_id VARCHAR(255) NOT NULL,
    date VARCHAR(10) NOT NULL, -- YYYY-MM-DD 格式
    content TEXT NOT NULL,
    article_ids BIGINT[] NOT NULL DEFAULT ARRAY[]::BIGINT[],
    article_count INT NOT NULL DEFAULT 0,
    ai_model VARCHAR(128),
    generated_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE(user_id, space_id, date)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_daily_digests_user_space ON quka_rss_daily_digests(user_id, space_id);
CREATE INDEX IF NOT EXISTS idx_rss_daily_digests_date ON quka_rss_daily_digests(date DESC);
CREATE INDEX IF NOT EXISTS idx_rss_daily_digests_user_date ON quka_rss_daily_digests(user_id, date DESC);

-- 字段注释
COMMENT ON TABLE quka_rss_daily_digests IS 'RSS每日摘要表（AI整合后的每日报告）';
COMMENT ON COLUMN quka_rss_daily_digests.user_id IS '用户ID';
COMMENT ON COLUMN quka_rss_daily_digests.space_id IS '空间ID';
COMMENT ON COLUMN quka_rss_daily_digests.date IS '日期（YYYY-MM-DD格式）';
COMMENT ON COLUMN quka_rss_daily_digests.content IS '整合后的摘要内容（Markdown格式）';
COMMENT ON COLUMN quka_rss_daily_digests.article_ids IS '包含的文章ID列表';
COMMENT ON COLUMN quka_rss_daily_digests.article_count IS '文章总数';
COMMENT ON COLUMN quka_rss_daily_digests.ai_model IS '生成摘要使用的AI模型';
COMMENT ON COLUMN quka_rss_daily_digests.generated_at IS '生成时间戳';
COMMENT ON COLUMN quka_rss_daily_digests.created_at IS '创建时间戳';
