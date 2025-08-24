-- 创建 quka_knowledge_meta 表
CREATE TABLE IF NOT EXISTS quka_knowledge_meta (
    id VARCHAR(32) PRIMARY KEY, -- 元数据ID，唯一标识每条记录
    space_id VARCHAR(32) NOT NULL, -- 空间ID，标识任务归属的空间
    meta_info TEXT,              -- 元数据信息
    created_at BIGINT           -- 记录创建时间（Unix时间戳）
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_meta_space_id ON quka_knowledge_meta (space_id);

-- 为表中的字段添加注释
COMMENT ON COLUMN quka_knowledge_meta.id IS '元数据ID，唯一标识每条记录';
COMMENT ON COLUMN quka_knowledge_meta.space_id IS '空间ID，标识任务归属的空间';
COMMENT ON COLUMN quka_knowledge_meta.meta_info IS '元数据信息，存储关于知识的元数据内容';
COMMENT ON COLUMN quka_knowledge_meta.created_at IS '记录创建时间，Unix时间戳';