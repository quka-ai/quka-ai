-- 创建表 quka_chat_summary
CREATE TABLE IF NOT EXISTS quka_chat_summary (
    id VARCHAR(32) PRIMARY KEY,                  -- 主键，消息摘要唯一标识符
    space_id VARCHAR(32) NOT NULL,             -- 消息ID，关联具体的消息
    sequence BIGINT NOT NULL,             -- 消息ID，关联具体的消息
    session_id VARCHAR(32) NOT NULL,             -- 会话ID，关联具体的会话
    content TEXT NOT NULL,                -- 消息摘要的内容
    created_at BIGINT NOT NULL            -- 创建时间，使用UNIX时间戳
);

CREATE INDEX IF NOT EXISTS idx_quka_chat_summary_session_id_sequence ON quka_chat_summary (session_id,sequence); -- 用户ID索引，提升用户相关的查询速度
CREATE INDEX IF NOT EXISTS idx_quka_chat_summary_space_id ON quka_chat_summary (space_id);

-- 添加字段备注
COMMENT ON COLUMN quka_chat_summary.id IS '主键，消息摘要唯一标识符';
COMMENT ON COLUMN quka_chat_summary.sequence IS '总结到的消息对应的sequence';
COMMENT ON COLUMN quka_chat_summary.session_id IS '会话ID，关联具体的会话';
COMMENT ON COLUMN quka_chat_summary.space_id IS '空间ID，关联具体的空间';
COMMENT ON COLUMN quka_chat_summary.content IS '消息摘要的内容';
COMMENT ON COLUMN quka_chat_summary.created_at IS '创建时间，使用UNIX时间戳';
