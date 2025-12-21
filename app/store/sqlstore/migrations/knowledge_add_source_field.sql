-- 添加 source 和 source_ref 字段到 quka_knowledge 表
ALTER TABLE quka_knowledge ADD COLUMN IF NOT EXISTS source VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE quka_knowledge ADD COLUMN IF NOT EXISTS source_ref VARCHAR(100) NOT NULL DEFAULT '';

-- 添加字段注释
COMMENT ON COLUMN quka_knowledge.source IS 'knowledge来源类型，空字符串表示平台内部创建，可选值: rss, podcast, mcp, chat';
COMMENT ON COLUMN quka_knowledge.source_ref IS 'knowledge来源引用ID，如chat_session_id、subscription_id等，空字符串表示无引用';
