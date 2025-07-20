CREATE TABLE IF NOT EXISTS quka_user_global_role (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    appid VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'role-member',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CONSTRAINT uk_user_global_role_user_appid UNIQUE (user_id, appid)
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_user_global_role_appid ON quka_user_global_role(appid);
CREATE INDEX IF NOT EXISTS idx_user_global_role_role ON quka_user_global_role(role);
CREATE INDEX IF NOT EXISTS idx_user_global_role_user_id ON quka_user_global_role(user_id);