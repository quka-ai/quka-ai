CREATE TABLE IF NOT EXISTS quka_memory (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    knowledge_id VARCHAR(32) NOT NULL,
    memory_type VARCHAR(20) NOT NULL,
    scope VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    importance SMALLINT NOT NULL DEFAULT 50,
    confidence NUMERIC(4,3) NOT NULL DEFAULT 0.700,
    author_type VARCHAR(20) NOT NULL DEFAULT 'agent',
    epistemic_status VARCHAR(20) NOT NULL DEFAULT 'summarized',
    source_kind VARCHAR(20) NOT NULL,
    source_ref VARCHAR(64) NOT NULL DEFAULT '',
    entity_key VARCHAR(128) NOT NULL DEFAULT '',
    dedupe_key VARCHAR(128) NOT NULL DEFAULT '',
    conflict_state VARCHAR(20) NOT NULL DEFAULT 'none',
    valid_from BIGINT NOT NULL DEFAULT 0,
    valid_to BIGINT NOT NULL DEFAULT 0,
    last_accessed_at BIGINT NOT NULL DEFAULT 0,
    access_count BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CONSTRAINT chk_quka_memory_layer_scope CHECK (
        (scope = 'user' AND user_id <> '')
        OR (scope = 'space' AND space_id <> '-' AND space_id <> '')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_memory_knowledge_id
ON quka_memory (knowledge_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_space_type_status
ON quka_memory (space_id, memory_type, status);

CREATE INDEX IF NOT EXISTS idx_quka_memory_entity
ON quka_memory (space_id, entity_key);

CREATE INDEX IF NOT EXISTS idx_quka_memory_dedupe_key
ON quka_memory (space_id, dedupe_key);

CREATE INDEX IF NOT EXISTS idx_quka_memory_user_layer_access
ON quka_memory (user_id, scope, space_id, status, memory_type, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_quka_memory_user_layer_entity
ON quka_memory (user_id, scope, space_id, entity_key, status);

COMMENT ON COLUMN quka_memory.knowledge_id IS '关联的knowledge id';
COMMENT ON COLUMN quka_memory.memory_type IS '记忆类型，如core/episodic/semantic/working';
COMMENT ON COLUMN quka_memory.space_id IS 'memory namespace: "-" for user-global memory, otherwise the owning space id for user-space or space-shared memory';
COMMENT ON COLUMN quka_memory.scope IS 'memory scope: user for private user layers, space for shared memory visible to space members';
COMMENT ON COLUMN quka_memory.status IS '记忆状态';
COMMENT ON COLUMN quka_memory.importance IS '业务重要度';
COMMENT ON COLUMN quka_memory.confidence IS '记忆可信度';
COMMENT ON COLUMN quka_memory.author_type IS '写入主体';
COMMENT ON COLUMN quka_memory.epistemic_status IS '认知状态';
COMMENT ON COLUMN quka_memory.source_kind IS '来源类型';
COMMENT ON COLUMN quka_memory.source_ref IS '来源引用';
COMMENT ON COLUMN quka_memory.entity_key IS '实体归档键';
COMMENT ON COLUMN quka_memory.dedupe_key IS '去重键';
COMMENT ON COLUMN quka_memory.conflict_state IS '冲突状态';
