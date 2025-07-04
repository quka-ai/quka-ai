CREATE TABLE IF NOT EXISTS bw_user_space (
    user_id VARCHAR(32) NOT NULL,   -- 用户的唯一标识
    space_id VARCHAR(32) NOT NULL,  -- 空间的唯一标识
    role VARCHAR(50) NOT NULL, -- 用户在空间中的角色
    created_at BIGINT NOT NULL, -- 记录创建时间
    UNIQUE (user_id, space_id) -- 确保每个用户与每个空间只有一个记录
);

-- 为每个字段添加注释
COMMENT ON COLUMN bw_user_space.user_id IS '用户ID';
COMMENT ON COLUMN bw_user_space.space_id IS '空间ID';
COMMENT ON COLUMN bw_user_space.role IS '用户在空间中的角色';
COMMENT ON COLUMN bw_user_space.created_at IS '创建时间，存储为时间戳';

-- 创建 user_id 和 space_id 索引
CREATE INDEX IF NOT EXISTS idx_space_user_id ON bw_user_space (user_id);
CREATE INDEX IF NOT EXISTS idx_space_space_id ON bw_user_space (space_id);