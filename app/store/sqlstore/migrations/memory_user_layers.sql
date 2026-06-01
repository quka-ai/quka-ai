-- Migration: add indexes for user-global, user-space, and space-shared layers.
-- user_global: space_id='-', scope='user', user_id=<owner>
-- user_space:  space_id=<current_space>, scope='user', user_id=<owner>
-- space_shared: space_id=<current_space>, scope='space'

CREATE INDEX IF NOT EXISTS idx_quka_memory_user_layer_access
ON quka_memory (user_id, scope, space_id, status, memory_type, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_quka_memory_user_layer_entity
ON quka_memory (user_id, scope, space_id, entity_key, status);

CREATE INDEX IF NOT EXISTS idx_quka_vectors_user_space_knowledge
ON quka_vectors (user_id, space_id, knowledge_id);

COMMENT ON COLUMN quka_memory.space_id IS 'memory namespace: "-" for user-global memory, otherwise the owning space id for user-space memory';
COMMENT ON COLUMN quka_memory.scope IS 'memory scope: user for private user layers, space for shared memory visible to space members';
