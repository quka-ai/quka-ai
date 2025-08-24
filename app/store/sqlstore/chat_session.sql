-- 创建 quka_chat_session 表，表示会话的基本信息
CREATE TABLE IF NOT EXISTS quka_chat_session (
    id VARCHAR(32) PRIMARY KEY, -- 会话的唯一标识
    space_id VARCHAR(32) NOT NULL, -- 空间ID，表示session属于哪个space 
    user_id VARCHAR(32) NOT NULL, -- 用户ID，表示属于哪个用户
    title VARCHAR(255) NOT NULL, -- 会话的标题
    session_type SMALLINT NOT NULL, -- 会话类型，1表示私聊，2表示群聊
    status SMALLINT NOT NULL, -- 会话状态，1表示活跃，2表示已结束
    created_at BIGINT NOT NULL, -- 会话创建时间，存储为Unix时间戳（秒）
    latest_access_time BIGINT NOT NULL -- 最近一次访问时间，存储为Unix时间戳（秒）
);

-- 为 quka_chat_session 表添加索引
CREATE INDEX IF NOT EXISTS idx_quka_chat_session_space_id_user_id ON quka_chat_session (space_id,user_id); -- 用户ID索引，提升用户相关的查询速度
CREATE INDEX IF NOT EXISTS idx_quka_chat_session_created_at ON quka_chat_session (created_at); -- 创建时间索引，优化按时间排序的查询
CREATE INDEX IF NOT EXISTS idx_quka_chat_session_latest_access_time ON quka_chat_session (latest_access_time); -- 最近访问时间索引，优化按最近访问查询

-- 添加字段注释
COMMENT ON COLUMN quka_chat_session.id IS '会话的唯一标识，使用字符串形式的 ID';
COMMENT ON COLUMN quka_chat_session.space_id IS '空间ID，表示session属于哪个space ';
COMMENT ON COLUMN quka_chat_session.user_id IS '用户ID，表示会话属于哪个用户';
COMMENT ON COLUMN quka_chat_session.title IS '会话的标题，描述该会话的主题或名称';
COMMENT ON COLUMN quka_chat_session.session_type IS '会话类型，1表示私聊，2表示群聊';
COMMENT ON COLUMN quka_chat_session.status IS '会话状态，1表示活跃，2表示已结束';
COMMENT ON COLUMN quka_chat_session.created_at IS '会话创建时间，Unix时间戳，表示秒';
COMMENT ON COLUMN quka_chat_session.latest_access_time IS '最近一次访问时间，Unix时间戳，表示秒';
