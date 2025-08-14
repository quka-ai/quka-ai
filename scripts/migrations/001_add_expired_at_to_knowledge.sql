-- 添加 expired_at 字段到 quka_knowledge 表
-- Migration: 001_add_expired_at_to_knowledge
-- Date: 2025-01-17
-- Description: 为 Knowledge 表添加过期时间字段支持资源内容有效期功能

BEGIN;

-- 1. 添加 expired_at 字段
ALTER TABLE quka_knowledge 
ADD COLUMN expired_at BIGINT NOT NULL DEFAULT 0;

-- 2. 添加字段注释
COMMENT ON COLUMN quka_knowledge.expired_at IS '过期时间戳，0表示永不过期';

-- 3. 创建过期时间索引（查询性能关键）
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_expired_at 
ON quka_knowledge(expired_at);

-- 4. 创建复合索引用于按resource和过期状态查询
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_resource_expired_at 
ON quka_knowledge(resource, expired_at);

-- 5. 为现有数据计算并设置过期时间
-- 根据关联的resource.cycle计算expired_at
UPDATE quka_knowledge k 
SET expired_at = (
    SELECT CASE 
        WHEN r.cycle > 0 THEN k.created_at + r.cycle * 86400
        ELSE 0 
    END
    FROM quka_resource r 
    WHERE r.id = k.resource
)
WHERE k.resource IS NOT NULL 
  AND k.resource != '' 
  AND k.resource != 'knowledge';  -- 排除默认resource

COMMIT;