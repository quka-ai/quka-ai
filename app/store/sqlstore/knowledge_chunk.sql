-- 创建表 quka_knowledge_chunk
CREATE TABLE IF NOT EXISTS quka_knowledge_chunk (
    id VARCHAR(32) PRIMARY KEY, -- 主键，自增ID
    knowledge_id VARCHAR(32) NOT NULL, -- 知识点ID
    space_id VARCHAR(32) NOT NULL, -- 空间ID
    user_id VARCHAR(32) NOT NULL, -- 用户ID
    chunk TEXT NOT NULL, -- 知识片段
    original_length INT NOT NULL DEFAULT 0, -- 关联知识点长度
    updated_at BIGINT NOT NULL DEFAULT 0, -- 更新时间
    created_at BIGINT NOT NULL DEFAULT 0 -- 创建时间
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_chunk_space_id_knowledge ON quka_knowledge_chunk (space_id,knowledge_id);
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_chunk_space_user_id ON quka_knowledge_chunk (space_id,user_id);

-- 为字段添加注释
COMMENT ON COLUMN quka_knowledge_chunk.id IS '主键，自增ID';
COMMENT ON COLUMN quka_knowledge_chunk.knowledge_id IS '知识点ID';
COMMENT ON COLUMN quka_knowledge_chunk.space_id IS '空间ID';
COMMENT ON COLUMN quka_knowledge_chunk.user_id IS '用户ID';
COMMENT ON COLUMN quka_knowledge_chunk.chunk IS '知识片段';
COMMENT ON COLUMN quka_knowledge_chunk.original_length IS '关联知识点长度';
COMMENT ON COLUMN quka_knowledge_chunk.updated_at IS '创建时间';
COMMENT ON COLUMN quka_knowledge_chunk.created_at IS '创建时间';
