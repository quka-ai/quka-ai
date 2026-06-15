-- Migration: allow working memory to store short-lived content inline instead
-- of forcing every memory to create hidden backing knowledge.

ALTER TABLE quka_memory
ADD COLUMN IF NOT EXISTS title VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE quka_memory
ADD COLUMN IF NOT EXISTS content TEXT NOT NULL DEFAULT '';

ALTER TABLE quka_memory
ADD COLUMN IF NOT EXISTS content_type VARCHAR(20) NOT NULL DEFAULT 'markdown';

ALTER TABLE quka_memory
ALTER COLUMN knowledge_id SET DEFAULT '';

DROP INDEX IF EXISTS idx_quka_memory_knowledge_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_memory_knowledge_id
ON quka_memory (knowledge_id)
WHERE knowledge_id <> '';

ALTER TABLE quka_memory
DROP CONSTRAINT IF EXISTS chk_quka_memory_content_backing;

ALTER TABLE quka_memory
ADD CONSTRAINT chk_quka_memory_content_backing CHECK (
    knowledge_id <> ''
    OR (memory_type = 'working' AND content <> '')
) NOT VALID;
