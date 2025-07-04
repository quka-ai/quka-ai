-- 创建表 bw_journal
CREATE TABLE IF NOT EXISTS bw_journal (
    id BIGINT PRIMARY KEY,         -- 自增主键
    space_id VARCHAR(32) NOT NULL, -- 空间ID
    user_id VARCHAR(32) NOT NULL, -- 用户ID
    content TEXT NOT NULL, -- 知识片段
    date VARCHAR(10) NOT NULL DEFAULT 0, -- 关联知识点长度
    updated_at BIGINT NOT NULL DEFAULT 0, -- 更新时间
    created_at BIGINT NOT NULL DEFAULT 0 -- 创建时间
);

-- 创建索引
CREATE UNIQUE INDEX IF NOT EXISTS bw_journal_space_id_user_id_date ON bw_journal (space_id, user_id, date);
CREATE INDEX IF NOT EXISTS bw_journal_date ON bw_journal (date);

-- 为字段添加注释
COMMENT ON COLUMN bw_journal.id IS '主键，自增ID';
COMMENT ON COLUMN bw_journal.space_id IS '空间ID';
COMMENT ON COLUMN bw_journal.user_id IS '用户ID';
COMMENT ON COLUMN bw_journal.content IS '知识片段';
COMMENT ON COLUMN bw_journal.date IS '日期 2006-01-02';
COMMENT ON COLUMN bw_journal.updated_at IS '更新时间';
COMMENT ON COLUMN bw_journal.created_at IS '创建时间';
