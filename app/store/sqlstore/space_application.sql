-- 创建 bw_space_application 表
CREATE TABLE bw_space_application (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,  -- 空间ID，用于区分不同的空间
    user_id VARCHAR(32) NOT NULL,     -- 用户ID
    user_email VARCHAR(100) NOT NULL, -- 用户邮箱
    user_name VARCHAR(50) NOT NULL, -- 用户名称
    desc TEXT NOT NULL,     -- 申请描述
    updated_at BIGINT NOT NULL,      -- 状态更新时间，单位为秒
    created_at BIGINT NOT NULL,     -- 创建时间戳，单位为秒
);

-- 为 token 字段创建索引，加快查找速度
CREATE INDEX idx_space_user ON bw_space_application(space_id,user_id);

-- 为每个字段添加注释
COMMENT ON COLUMN bw_knowledge.id IS '唯一标识';
COMMENT ON COLUMN bw_share_token.space_id IS '空间ID，用于区分不同的空间';
COMMENT ON COLUMN bw_share_token.user_id IS '应用ID';
COMMENT ON COLUMN bw_share_token.user_name IS '用户名称';
COMMENT ON COLUMN bw_share_token.user_email IS '用户邮箱';
COMMENT ON COLUMN bw_share_token.desc IS '申请描述';
COMMENT ON COLUMN bw_share_token.updated_at IS '状态更新时间，单位为秒';
COMMENT ON COLUMN bw_share_token.created_at IS '创建时间戳，单位为秒';
