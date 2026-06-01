CREATE TABLE IF NOT EXISTS quka_memory_binding (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL DEFAULT '',
    memory_id VARCHAR(32) NOT NULL,
    context_type VARCHAR(20) NOT NULL,
    context_id VARCHAR(64) NOT NULL,
    binding_type VARCHAR(20) NOT NULL,
    pinned_by VARCHAR(20) NOT NULL DEFAULT 'system',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CONSTRAINT chk_quka_memory_binding_user_id CHECK (user_id <> '')
);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_user_context
ON quka_memory_binding (space_id, user_id, context_type, context_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_memory
ON quka_memory_binding (space_id, memory_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_user_memory
ON quka_memory_binding (user_id, memory_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_memory_binding_unique_context_memory
ON quka_memory_binding (space_id, user_id, memory_id, context_type, context_id, binding_type);

COMMENT ON COLUMN quka_memory_binding.context_type IS 'runtime上下文类型';
COMMENT ON COLUMN quka_memory_binding.context_id IS 'runtime上下文实例ID';
COMMENT ON COLUMN quka_memory_binding.binding_type IS 'binding类型';
COMMENT ON COLUMN quka_memory_binding.pinned_by IS '由human/agent/system哪方固定';
COMMENT ON COLUMN quka_memory_binding.user_id IS 'binding所属用户，用于中心化服务隔离相同runtime context id';
