-- Migration: isolate runtime memory bindings per user.
-- Center-hosted agents may reuse the same agent_run/task/workspace id across users,
-- so bindings must carry user_id instead of relying on space/context alone.

ALTER TABLE quka_memory_binding
ADD COLUMN IF NOT EXISTS user_id VARCHAR(32) NOT NULL DEFAULT '';

UPDATE quka_memory_binding AS b
SET user_id = m.user_id
FROM quka_memory AS m
WHERE b.memory_id = m.id
  AND b.user_id = '';

DELETE FROM quka_memory_binding b
USING (
    SELECT ctid
    FROM (
        SELECT ctid,
               ROW_NUMBER() OVER (
                   PARTITION BY space_id, user_id, memory_id, context_type, context_id, binding_type
                   ORDER BY updated_at DESC, created_at DESC
               ) AS rn
        FROM quka_memory_binding
    ) ranked
    WHERE ranked.rn > 1
) dup
WHERE b.ctid = dup.ctid;

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_user_context
ON quka_memory_binding (space_id, user_id, context_type, context_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_user_memory
ON quka_memory_binding (user_id, memory_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_memory_binding_unique_context_memory
ON quka_memory_binding (space_id, user_id, memory_id, context_type, context_id, binding_type);
