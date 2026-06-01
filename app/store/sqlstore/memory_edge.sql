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

COMMENT ON COLUMN quka_memory_edge.from_memory_id IS '来源memory id';
COMMENT ON COLUMN quka_memory_edge.to_memory_id IS '目标memory id';
COMMENT ON COLUMN quka_memory_edge.relation IS 'memory关系';
COMMENT ON COLUMN quka_memory_edge.weight IS '关系权重';
