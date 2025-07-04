CREATE TABLE IF NOT EXISTS bw_resource (
    id VARCHAR(32) NOT NULL, -- 资源的唯一标识
    title VARCHAR(255) NOT NULL,         -- 资源标题
    user_id VARCHAR(32) NOT NULL         -- 用户id
    space_id VARCHAR(32) NOT NULL,       -- 资源所属空间ID
    cycle int NOT NULL,                  -- cycle
    prompt TEXT NOT NULL,                -- 自定义prompt
    description TEXT,                    -- 资源描述信息
    created_at BIGINT NOT NULL          -- 资源创建时间，UNIX时间戳
);

-- 为 space_id，id 创建唯一索引
CREATE UNIQUE INDEX IF NOT EXISTS bw_resource_unique_idx ON bw_resource (space_id, id);

-- 添加字段注释
COMMENT ON COLUMN bw_resource.id IS '资源的唯一标识';
COMMENT ON COLUMN bw_resource.title IS '资源标题';
COMMENT ON COLUMN bw_resource.user_id IS '用户ID';
COMMENT ON COLUMN bw_resource.space_id IS '资源所属空间ID';
COMMENT ON COLUMN bw_resource.description IS '资源描述信息';
COMMENT ON COLUMN bw_resource.cycle IS '资源周期';
COMMENT ON COLUMN bw_resource.prompt IS '自定义prompt';
COMMENT ON COLUMN bw_resource.created_at IS '资源创建时间，UNIX时间戳';

-- 添加表注释
COMMENT ON TABLE bw_resource IS '资源类型表';