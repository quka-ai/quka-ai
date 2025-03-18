-- 创建 bw_chat_session_pin 表
CREATE TABLE bw_chat_session_pin (
    session_id VARCHAR(32) NOT NULL,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    content JSONB NOT NULL,
    version VARCHAR(6) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (space_id, session_id, user_id)
);

-- 添加索引
CREATE INDEX idx_bw_chat_session_pin_session_id ON bw_chat_session_pin (session_id);
CREATE INDEX idx_bw_chat_session_pin_user_id ON bw_chat_session_pin (user_id);
CREATE INDEX idx_bw_chat_session_pin_created_at ON bw_chat_session_pin (created_at);

-- 添加字段注释（PostgreSQL 不支持直接在字段定义中使用 COMMENT）
COMMENT ON COLUMN bw_chat_session_pin.session_id IS '唯一标识一个会话';
COMMENT ON COLUMN bw_chat_session_pin.space_id IS '所属空间的标识';
COMMENT ON COLUMN bw_chat_session_pin.user_id IS '用户的唯一标识';
COMMENT ON COLUMN bw_chat_session_pin.content IS '与会话关联的内容，支持存储knowledge、journal等';
COMMENT ON COLUMN bw_chat_session_pin.version IS 'JSON内容格式版本号，向前兼容';
COMMENT ON COLUMN bw_chat_session_pin.created_at IS '记录的创建时间，时间戳格式';
COMMENT ON COLUMN bw_chat_session_pin.updated_at IS '记录的更新时间，时间戳格式';
