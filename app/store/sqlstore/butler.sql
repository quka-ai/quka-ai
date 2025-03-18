-- 创建 bw_butler 表
CREATE TABLE bw_butler (
    table_id VARCHAR(32) PRIMARY KEY,  -- 记录ID, 自动递增
    user_id VARCHAR(32) NOT NULL, -- 关联用户id
    table_name VARCHAR(255) NOT NULL,  -- 日常事项名称
    table_description TEXT,  -- 事项描述
    table_data TEXT,  -- 事项相关数据，支持结构化数据存储
    created_at BIGINT NOT NULL,  -- 创建时间，使用 Unix 时间戳
    updated_at BIGINT NOT NULL   -- 更新时间，使用 Unix 时间戳
);

-- 为表的字段添加注释
COMMENT ON COLUMN bw_butler.table_id IS '记录ID, 自动递增';
COMMENT ON COLUMN bw_butler.user_id IS '关联用户id';
COMMENT ON COLUMN bw_butler.table_name IS '日常事项的名称';
COMMENT ON COLUMN bw_butler.table_description IS '事项的详细描述';
COMMENT ON COLUMN bw_butler.table_data IS '与事项相关的额外数据，支持结构化存储';
COMMENT ON COLUMN bw_butler.created_at IS '记录的创建时间，Unix时间戳';
COMMENT ON COLUMN bw_butler.updated_at IS '记录的最后更新时间，Unix时间戳';

-- 创建索引，方便通过名称查询
CREATE INDEX idx_bulter_user_id ON bw_butler (user_id);
