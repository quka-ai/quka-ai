-- 创建 quka_podcasts 表
CREATE TABLE IF NOT EXISTS quka_podcasts (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    space_id VARCHAR(36) NOT NULL,

    -- 来源信息
    source_type VARCHAR(20) NOT NULL,
    source_id VARCHAR(36) NOT NULL,

    -- 基本信息
    title VARCHAR(500) DEFAULT '',
    description TEXT,
    tags TEXT[],

    -- 音频信息
    audio_url VARCHAR(1000) DEFAULT '',
    audio_duration INTEGER DEFAULT 0,
    audio_size BIGINT DEFAULT 0,
    audio_format VARCHAR(10) DEFAULT '',

    -- TTS 配置
    tts_provider VARCHAR(50) DEFAULT '',
    tts_model VARCHAR(100) DEFAULT '',

    -- 状态信息
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    retry_times INTEGER DEFAULT 0,
    generation_last_updated BIGINT DEFAULT 0,

    -- 时间戳
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    generated_at BIGINT
);

-- 添加字段注释
COMMENT ON COLUMN quka_podcasts.id IS '唯一标识';
COMMENT ON COLUMN quka_podcasts.user_id IS '用户ID';
COMMENT ON COLUMN quka_podcasts.space_id IS '空间ID';
COMMENT ON COLUMN quka_podcasts.source_type IS '来源类型: knowledge/journal/rss_digest';
COMMENT ON COLUMN quka_podcasts.source_id IS '对应源的ID';
COMMENT ON COLUMN quka_podcasts.title IS '播客标题';
COMMENT ON COLUMN quka_podcasts.description IS '播客描述';
COMMENT ON COLUMN quka_podcasts.tags IS '标签数组';
COMMENT ON COLUMN quka_podcasts.audio_url IS 'S3存储的音频文件URL';
COMMENT ON COLUMN quka_podcasts.audio_duration IS '音频时长（秒）';
COMMENT ON COLUMN quka_podcasts.audio_size IS '音频文件大小（字节）';
COMMENT ON COLUMN quka_podcasts.audio_format IS '音频格式 mp3/m4a';
COMMENT ON COLUMN quka_podcasts.tts_provider IS 'TTS服务商';
COMMENT ON COLUMN quka_podcasts.tts_model IS 'TTS模型';
COMMENT ON COLUMN quka_podcasts.status IS '状态: pending/processing/completed/failed';
COMMENT ON COLUMN quka_podcasts.error_message IS '错误信息';
COMMENT ON COLUMN quka_podcasts.retry_times IS '重试次数';
COMMENT ON COLUMN quka_podcasts.generation_last_updated IS '生成进度最后更新时间戳，用于前端判断生成是否仍在进行';
COMMENT ON COLUMN quka_podcasts.created_at IS '创建时间';
COMMENT ON COLUMN quka_podcasts.updated_at IS '更新时间';
COMMENT ON COLUMN quka_podcasts.generated_at IS '音频生成完成时间';

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_podcasts_user_space ON quka_podcasts (user_id, space_id);
CREATE INDEX IF NOT EXISTS idx_quka_podcasts_source ON quka_podcasts (source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_quka_podcasts_status ON quka_podcasts (status);
CREATE INDEX IF NOT EXISTS idx_quka_podcasts_created_at ON quka_podcasts (created_at DESC);
