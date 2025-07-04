-- 创建表 bw_chat_message_ext
CREATE TABLE IF NOT EXISTS bw_chat_message_ext (
    message_id VARCHAR(32) PRIMARY KEY,      -- 关联消息的唯一标识符
    session_id VARCHAR(32) NOT NULL,      -- 关联会话的唯一标识符
    space_id VARCHAR(32) NOT NULL, -- 空间ID，表示消息所属的空间
    evaluate SMALLINT NOT NULL,                 -- 评价状态，使用 EvaluateType 枚举
    generation_status SMALLINT NOT NULL,        -- 生成状态，使用 GenerationStatusType 枚举
    rel_docs TEXT[],              -- 相关文档数组，存储多个文档标识符
    created_at BIGINT NOT NULL,            -- 创建时间，Unix 时间戳
    updated_at BIGINT NOT NULL             -- 更新时间，Unix 时间戳
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_chat_message_ext_space_session_message ON bw_chat_message_ext (space_id, session_id, message_id); -- 空间ID索引，提升按空间查询的速度

-- 为字段添加注释
COMMENT ON COLUMN bw_chat_message_ext.message_id IS '关联消息的唯一标识符';
COMMENT ON COLUMN bw_chat_message_ext.session_id IS '关联会话的唯一标识符';
COMMENT ON COLUMN bw_chat_message_ext.space_id IS '空间ID，表示消息所属的空间';
COMMENT ON COLUMN bw_chat_message_ext.evaluate IS '评价状态，使用 EvaluateType 枚举';
COMMENT ON COLUMN bw_chat_message_ext.generation_status IS '生成状态，使用 GenerationStatusType 枚举';
COMMENT ON COLUMN bw_chat_message_ext.rel_docs IS '相关文档数组，存储多个文档标识符';
COMMENT ON COLUMN bw_chat_message_ext.created_at IS '创建时间，Unix 时间戳';
COMMENT ON COLUMN bw_chat_message_ext.updated_at IS '更新时间，Unix 时间戳';