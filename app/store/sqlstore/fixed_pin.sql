CREATE TABLE IF NOT EXISTS quka_fixed_pin (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(30) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_fixed_pin_space_user ON quka_fixed_pin (space_id, user_id);

COMMENT ON COLUMN quka_fixed_pin.id IS '唯一标识';
COMMENT ON COLUMN quka_fixed_pin.space_id IS '空间ID';
COMMENT ON COLUMN quka_fixed_pin.user_id IS '用户ID';
COMMENT ON COLUMN quka_fixed_pin.content IS '加密后的首页固定内容';
COMMENT ON COLUMN quka_fixed_pin.content_type IS '内容格式，当前为 BlockNote blocks_v2';
COMMENT ON COLUMN quka_fixed_pin.created_at IS '创建时间';
COMMENT ON COLUMN quka_fixed_pin.updated_at IS '更新时间';
