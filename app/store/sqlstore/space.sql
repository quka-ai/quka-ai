CREATE TABLE IF NOT EXISTS quka_space (
    space_id VARCHAR(32) NOT NULL,  -- 空间的唯一标识
    title VARCHAR(32) NOT NULL,  -- 空间的唯一标识
    description TEXT NOT NULL, -- 用户在空间中的角色
    base_prompt TEXT NOT NULL, -- 基础prompt
    chat_prompt TEXT NOT NULL, -- 聊天prompt
    created_at BIGINT NOT NULL, -- 记录创建时间
    UNIQUE (space_id) -- 确保每个空间只有一个记录
);

-- 为每个字段添加注释
COMMENT ON COLUMN quka_space.space_id IS '空间ID';
COMMENT ON COLUMN quka_space.title IS '空间标题';
COMMENT ON COLUMN quka_space.base_prompt IS '基础prompt';
COMMENT ON COLUMN quka_space.chat_prompt IS '聊天prompt';
COMMENT ON COLUMN quka_space.description IS '简介';
COMMENT ON COLUMN quka_space.created_at IS '创建时间，存储为时间戳';

-- 创建 user_id 和 space_id 索引
CREATE INDEX IF NOT EXISTS idx_space_id ON quka_space (space_id);