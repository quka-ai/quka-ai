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

CREATE TABLE IF NOT EXISTS quka_memory_edge (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    from_memory_id VARCHAR(32) NOT NULL,
    to_memory_id VARCHAR(32) NOT NULL,
    relation VARCHAR(20) NOT NULL,
    weight NUMERIC(4,3) NOT NULL DEFAULT 1.000,
    created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quka_memory_edge_from
ON quka_memory_edge (space_id, from_memory_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_edge_to
ON quka_memory_edge (space_id, to_memory_id);

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
