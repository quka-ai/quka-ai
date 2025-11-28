-- 为 knowledge 表添加 rel_doc_id 字段
-- 用于存储关联的文档任务ID，如果是用户直接录入则为空

ALTER TABLE quka_knowledge
ADD COLUMN IF NOT EXISTS rel_doc_id VARCHAR(32) NOT NULL DEFAULT '';

-- 为 rel_doc_id 字段添加索引，提升查询性能
-- 使用部分索引，只索引非空值，节省存储空间
CREATE INDEX IF NOT EXISTS idx_knowledge_rel_doc_id ON quka_knowledge(rel_doc_id) WHERE rel_doc_id != '';

-- 添加注释
COMMENT ON COLUMN quka_knowledge.rel_doc_id IS '关联的文档任务ID，如果是用户直接录入则为空字符串';
