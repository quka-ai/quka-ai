CREATE TABLE IF NOT EXISTS quka_resource (
    id VARCHAR(32) NOT NULL, -- 资源的唯一标识
    title VARCHAR(255) NOT NULL,         -- 资源标题
    user_id VARCHAR(32) NOT NULL,         -- 用户id
    space_id VARCHAR(32) NOT NULL,       -- 资源所属空间ID
    tag VARCHAR(255) NOT NULL,            -- 资源标签
    cycle int NOT NULL,                  -- cycle
    description TEXT,                    -- 资源描述信息
    created_at BIGINT NOT NULL          -- 资源创建时间，UNIX时间戳
);

-- 为 space_id，id 创建唯一索引
CREATE UNIQUE INDEX IF NOT EXISTS quka_resource_unique_idx ON quka_resource (space_id, id);

-- 添加字段注释
COMMENT ON COLUMN quka_resource.id IS '资源的唯一标识';
COMMENT ON COLUMN quka_resource.title IS '资源标题';
COMMENT ON COLUMN quka_resource.user_id IS '用户ID';
COMMENT ON COLUMN quka_resource.space_id IS '资源所属空间ID';
COMMENT ON COLUMN quka_resource.description IS '资源描述信息';
COMMENT ON COLUMN quka_resource.cycle IS '资源周期';
COMMENT ON COLUMN quka_resource.created_at IS '资源创建时间，UNIX时间戳';

-- 添加表注释
COMMENT ON TABLE quka_resource IS '资源类型表';