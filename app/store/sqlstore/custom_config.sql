-- 创建自定义配置表
CREATE TABLE IF NOT EXISTS bw_custom_config (
    name VARCHAR(255) PRIMARY KEY,
    description TEXT,
    value JSONB,
    category VARCHAR(100),
    status INTEGER DEFAULT 1,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_custom_config_category ON bw_custom_config(category);
CREATE INDEX IF NOT EXISTS idx_custom_config_status ON bw_custom_config(status);
CREATE INDEX IF NOT EXISTS idx_custom_config_created_at ON bw_custom_config(created_at); 