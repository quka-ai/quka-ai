-- 创建 quka_knowledge 表
CREATE TABLE IF NOT EXISTS quka_knowledge (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    stage SMALLINT NOT NULL,
    resource VARCHAR(32) NOT NULL,
    title TEXT NOT NULL,
    tags TEXT[],
    content TEXT NOT NULL,
    content_type VARCHAR(30) NOT NULL,
    summary TEXT NOT NULL,
    maybe_date VARCHAR(20) NOT NULL,
    retry_times SMALLINT NOT NULL DEFAULT 0,
    rel_doc_id VARCHAR(32) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    expired_at BIGINT NOT NULL DEFAULT 0
);

-- 添加字段注释
COMMENT ON COLUMN quka_knowledge.id IS '唯一标识';
COMMENT ON COLUMN quka_knowledge.space_id IS '空间ID';
COMMENT ON COLUMN quka_knowledge.user_id IS '作者ID';
COMMENT ON COLUMN quka_knowledge.kind IS '知识类型';
COMMENT ON COLUMN quka_knowledge.stage IS 'ai flow stage';
COMMENT ON COLUMN quka_knowledge.tags IS '标签列表，使用数组存储';
COMMENT ON COLUMN quka_knowledge.resource IS '资源类型/knowledge/context';
COMMENT ON COLUMN quka_knowledge.title IS '内容标题';
COMMENT ON COLUMN quka_knowledge.content IS '知识内容';
COMMENT ON COLUMN quka_knowledge.content_type IS '内容格式';
COMMENT ON COLUMN quka_knowledge.summary IS 'summary顾虑条件';
COMMENT ON COLUMN quka_knowledge.maybe_date IS 'AI分析出的事件发生时间 / 创建时间';
COMMENT ON COLUMN quka_knowledge.retry_times IS '流水线相关动作重试次数';
COMMENT ON COLUMN quka_knowledge.rel_doc_id IS '关联的文档任务ID，如果是用户直接录入则为空字符串';
COMMENT ON COLUMN quka_knowledge.created_at IS '创建时间';
COMMENT ON COLUMN quka_knowledge.updated_at IS '更新时间';
COMMENT ON COLUMN quka_knowledge.expired_at IS '过期时间戳，0表示永不过期';

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_main ON quka_knowledge (space_id, resource);
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_retry ON quka_knowledge (stage, retry_times);
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_expired_at ON quka_knowledge(expired_at);
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_rel_doc_id ON quka_knowledge(rel_doc_id);