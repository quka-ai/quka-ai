-- 移除播客表的 TTS 配置字段
-- 移除 tts_voice, tts_speed, tts_language 字段

-- 删除字段注释
COMMENT ON COLUMN quka_podcasts.tts_voice IS NULL;
COMMENT ON COLUMN quka_podcasts.tts_speed IS NULL;
COMMENT ON COLUMN quka_podcasts.tts_language IS NULL;

-- 移除字段
ALTER TABLE quka_podcasts DROP COLUMN IF EXISTS tts_voice;
ALTER TABLE quka_podcasts DROP COLUMN IF EXISTS tts_speed;
ALTER TABLE quka_podcasts DROP COLUMN IF EXISTS tts_language;