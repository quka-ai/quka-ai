-- 创建 AI Token 使用情况记录表
CREATE TABLE IF NOT EXISTS quka_ai_token_usage (
    id SERIAL PRIMARY KEY,                      -- 自增主键
    space_id VARCHAR(32) NOT NULL,             -- 空间 ID
    user_id VARCHAR(32) NOT NULL,              -- 用户 ID
    type VARCHAR(32) NOT NULL,                 -- 主类别
    sub_type VARCHAR(32) NOT NULL,             -- 子类别
    object_id VARCHAR(32) NOT NULL,            -- 对象 ID
    model VARCHAR(32) NOT NULL,                 -- ai模型名称
    usage_prompt INTEGER NOT NULL,              -- 使用的提示词令牌数
    usage_output INTEGER NOT NULL,              -- 使用的输出令牌数
    created_at BIGINT NOT NULL                  -- 记录创建时间
);

-- 索引设计
CREATE INDEX IF NOT EXISTS idx_quka_ai_token_usage_space_id ON quka_ai_token_usage (space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_quka_ai_token_usage_user_id ON quka_ai_token_usage (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_quka_ai_token_usage_created_at ON quka_ai_token_usage (created_at);
