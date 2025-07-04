-- 创建表
CREATE TABLE IF NOT EXISTS bw_access_token (
    id SERIAL PRIMARY KEY,
    appid VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    token VARCHAR(255) NOT NULL,
    version VARCHAR(10) NOT NULL,
    info TEXT
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
);

-- 添加字段注释
COMMENT ON COLUMN bw_access_token.id IS '主键，自增ID';
COMMENT ON COLUMN bw_access_token.user_id IS '用户ID，标识该 token 所属的用户';
COMMENT ON COLUMN bw_access_token.token IS '第三方用户的 access_token';
COMMENT ON COLUMN bw_access_token.version IS 'token存储格式的版本号，不同版本号对应的token claim结构可能不同';
COMMENT ON COLUMN bw_access_token.created_at IS '创建时间，UNIX时间戳';
COMMENT ON COLUMN bw_access_token.expires_at IS '过期时间，UNIX时间戳';
COMMENT ON COLUMN bw_access_token.info IS 'token 描述，描述 token 的用途或其他信息';


-- 为 user_id 字段创建索引
CREATE INDEX IF NOT EXISTS idx_bw_access_token_appid_user_id ON bw_access_token (appid, user_id);

-- 为 token 字段创建索引
CREATE INDEX IF NOT EXISTS idx_bw_access_token_appid_token ON bw_access_token (appid, token);