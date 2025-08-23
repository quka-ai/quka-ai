-- 添加思考功能支持字段迁移脚本
-- 思考功能支持类型：0-不支持，1-可选，2-强制

-- 添加新字段
ALTER TABLE quka_model_config 
ADD COLUMN IF NOT EXISTS thinking_support INTEGER NOT NULL DEFAULT 0;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_model_config_thinking_support 
ON quka_model_config (thinking_support);

-- 创建复合索引用于优化按模型类型和思考支持查询
CREATE INDEX IF NOT EXISTS idx_quka_model_config_type_thinking 
ON quka_model_config (model_type, thinking_support) 
WHERE model_type = 'chat';

-- 注释说明
COMMENT ON COLUMN quka_model_config.thinking_support IS '思考功能支持类型：0-不支持，1-可选，2-强制';