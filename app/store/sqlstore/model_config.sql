-- 创建模型配置表
CREATE TABLE IF NOT EXISTS quka_model_config (
    id VARCHAR(64) PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    model_type VARCHAR(50) NOT NULL,
    is_multi_modal BOOLEAN NOT NULL DEFAULT FALSE,
    status INTEGER NOT NULL DEFAULT 1,
    config JSONB,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_model_config_provider_id ON quka_model_config (provider_id);
CREATE INDEX IF NOT EXISTS idx_quka_model_config_model_type ON quka_model_config (model_type);
CREATE INDEX IF NOT EXISTS idx_quka_model_config_status ON quka_model_config (status);
CREATE INDEX IF NOT EXISTS idx_quka_model_config_model_name ON quka_model_config (model_name);
CREATE INDEX IF NOT EXISTS idx_quka_model_config_created_at ON quka_model_config (created_at); 