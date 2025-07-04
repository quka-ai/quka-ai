-- 创建 bw_space_application 表
CREATE TABLE IF NOT EXISTS bw_space_application (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,  -- 空间ID，用于区分不同的空间
    user_id VARCHAR(32) NOT NULL,     -- 用户ID
    description TEXT NOT NULL,     -- 申请描述
    status VARCHAR(32) NOT NULL,   -- 状态
    updated_at BIGINT NOT NULL,      -- 状态更新时间，单位为秒
    created_at BIGINT NOT NULL     -- 创建时间戳，单位为秒
);

-- 为 token 字段创建索引，加快查找速度
CREATE INDEX IF NOT EXISTS idx_space_user ON bw_space_application(space_id,user_id);

-- 为每个字段添加注释
COMMENT ON COLUMN bw_space_application.id IS '唯一标识';
COMMENT ON COLUMN bw_space_application.space_id IS '空间ID，用于区分不同的空间';
COMMENT ON COLUMN bw_space_application.user_id IS '用户ID';
COMMENT ON COLUMN bw_space_application.description IS '申请描述';
COMMENT ON COLUMN bw_space_application.status IS '状态';
COMMENT ON COLUMN bw_space_application.updated_at IS '状态更新时间，单位为秒';
COMMENT ON COLUMN bw_space_application.created_at IS '创建时间戳，单位为秒';
