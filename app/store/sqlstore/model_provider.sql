-- 创建模型提供商表
CREATE TABLE IF NOT EXISTS quka_model_provider (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    api_url VARCHAR(512),
    api_key VARCHAR(512),
    status INTEGER NOT NULL DEFAULT 1,
    config JSONB,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_model_provider_status ON quka_model_provider (status);
CREATE INDEX IF NOT EXISTS idx_quka_model_provider_name ON quka_model_provider (name);
CREATE INDEX IF NOT EXISTS idx_quka_model_provider_created_at ON quka_model_provider (created_at); 