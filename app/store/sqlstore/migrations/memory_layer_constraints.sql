-- Migration: enforce center-hosted memory layer invariants for new writes.
-- NOT VALID avoids blocking deployments with historical rows, while PostgreSQL
-- still checks new and updated rows after the constraint is added.

ALTER TABLE quka_memory
DROP CONSTRAINT IF EXISTS chk_quka_memory_layer_scope;

ALTER TABLE quka_memory
ADD CONSTRAINT chk_quka_memory_layer_scope CHECK (
    (scope = 'user' AND user_id <> '')
    OR (scope = 'space' AND space_id <> '-' AND space_id <> '')
) NOT VALID;

ALTER TABLE quka_memory_binding
DROP CONSTRAINT IF EXISTS chk_quka_memory_binding_user_id;

ALTER TABLE quka_memory_binding
ADD CONSTRAINT chk_quka_memory_binding_user_id CHECK (user_id <> '') NOT VALID;
