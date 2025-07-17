-- 创建 quka_knowledge_rel_meta 表
-- 用于保存长文 chunk 的 meta 信息
CREATE TABLE IF NOT EXISTS quka_knowledge_rel_meta (
    knowledge_id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    meta_id VARCHAR(32) NOT NULL, -- 关联的 meta 的主键
    chunk_index INT NOT NULL DEFAULT 1, -- chunk 的顺序编号，默认从1开始
    created_at BIGINT NOT NULL -- 创建时间，使用 UNIX 时间戳表示
);

-- 索引定义
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_rel_meta_space_id ON quka_knowledge_rel_meta (space_id);
-- 为 meta_id 添加索引
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_rel_meta_meta_id ON quka_knowledge_rel_meta (meta_id);
-- 为 chunk_index 添加索引，便于按顺序检索
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_rel_meta_chunk_index ON quka_knowledge_rel_meta (chunk_index);

-- 为 quka_knowledge_rel_meta 表字段添加注释
COMMENT ON COLUMN quka_knowledge_rel_meta.knowledge_id IS '长文的唯一标识，关联 metadata 表的主键';
COMMENT ON COLUMN quka_knowledge_rel_meta.space_id IS '所属空间ID';
COMMENT ON COLUMN quka_knowledge_rel_meta.meta_id IS '关联的 meta 的主键';
COMMENT ON COLUMN quka_knowledge_rel_meta.chunk_index IS 'chunk 的顺序编号，默认从1开始';
COMMENT ON COLUMN quka_knowledge_rel_meta.created_at IS '创建时间，使用 UNIX 时间戳表示';