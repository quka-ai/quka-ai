-- 为 quka_podcasts 表添加生成进度时间戳字段

-- 添加生成进度时间戳字段
ALTER TABLE quka_podcasts
ADD COLUMN IF NOT EXISTS generation_last_updated BIGINT DEFAULT 0;

-- 添加字段注释
COMMENT ON COLUMN quka_podcasts.generation_last_updated IS '生成进度最后更新时间戳，用于前端判断生成是否仍在进行';

-- 为已存在的 processing 状态记录设置时间戳
UPDATE quka_podcasts
SET generation_last_updated = updated_at
WHERE status = 'processing' AND generation_last_updated = 0;
