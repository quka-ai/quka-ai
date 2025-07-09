-- 创建 quka_share_token 表
CREATE TABLE IF NOT EXISTS quka_share_token (
    id SERIAL PRIMARY KEY, -- 自增ID，主键
    object_id VARCHAR(32) NOT NULL, -- 文章的唯一标识符 (原 knowledge_id 字段)
    space_id VARCHAR(32) NOT NULL,  -- 空间ID，用于区分不同的空间
    appid VARCHAR(32) NOT NULL,     -- 应用ID，标识分享来源的应用
    token VARCHAR(32) NOT NULL,     -- 分享链接的 Token (生成的唯一标识符)
    share_user_id VARCHAR(32) NOT NULL,     -- 分享用户id
    embedding_url TEXT NOT NULL,     -- 实际前端路径
    type VARCHAR(20) NOT NULL,      -- 类型，可能的值如"normal", "restricted" 等
    expire_at BIGINT NOT NULL,      -- 过期时间戳，单位为秒
    created_at BIGINT NOT NULL,     -- 创建时间戳，单位为秒
    CONSTRAINT unique_token UNIQUE (token),            -- 确保 token 唯一
    CONSTRAINT unique_object_id_type UNIQUE (object_id, type) -- 确保 object_id 和 type 的组合唯一
);

-- 为 token 字段创建索引，加快查找速度
CREATE INDEX IF NOT EXISTS idx_token ON quka_share_token(token);

-- 为每个字段添加注释
COMMENT ON COLUMN quka_share_token.id IS '自增ID，主键';
COMMENT ON COLUMN quka_share_token.object_id IS '文章的唯一标识符 (原 knowledge_id 字段)';
COMMENT ON COLUMN quka_share_token.space_id IS '空间ID，用于区分不同的空间';
COMMENT ON COLUMN quka_share_token.appid IS '应用ID，标识分享来源的应用';
COMMENT ON COLUMN quka_share_token.token IS '分享链接的 Token (生成的唯一标识符)';
COMMENT ON COLUMN quka_share_token.share_user_id IS '分享链接的用户id';
COMMENT ON COLUMN quka_share_token.embedding_url IS '实际前端路径';
COMMENT ON COLUMN quka_share_token.type IS '类型，可能的值如"normal", "restricted" 等';
COMMENT ON COLUMN quka_share_token.expire_at IS '过期时间戳，单位为秒';
COMMENT ON COLUMN quka_share_token.created_at IS '创建时间戳，单位为秒';
