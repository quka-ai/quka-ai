-- 创建 bw_chat_message 表，表示消息的基本信息
CREATE TABLE bw_chat_message (
    id VARCHAR(32) PRIMARY KEY, -- 消息的唯一标识，使用字符串形式的 ID
    space_id VARCHAR(32) NOT NULL, -- 空间ID，表示消息所属的空间
    session_id VARCHAR(32) NOT NULL, -- 会话ID，表示消息所属的会话
    user_id VARCHAR(64) NOT NULL, -- 用户ID，表示消息发送者的用户
    role SMALLINT NOT NULL, -- 角色，1 表示用户，2 表示系统
    message TEXT NOT NULL, -- 消息内容
    msg_type SMALLINT NOT NULL, -- 消息类型，1 表示文本，2 表示图片等
    send_time BIGINT NOT NULL, -- 消息发送时间，存储为 Unix 时间戳
    complete SMALLINT NOT NULL, -- 数据是否完整，1 表示完整，2 表示不完整
    sequence BIGINT NOT NULL, -- 消息的顺序，用于排序
    msg_block BIGINT NOT NULL, -- 消息所属的块编号，用于大消息的分块处理
    attach TEXT NOT NULL, -- 消息内容
    is_encrypt INT NOT NULL DEFAULT 0 -- 消息是否已加密
);

-- 为 bw_chat_message 表添加索引
CREATE INDEX idx_bw_chat_message_space_id ON bw_chat_message (space_id); -- 空间ID索引，提升按空间查询的速度
CREATE INDEX idx_bw_chat_message_session_id_message_id ON bw_chat_message (session_id, id); -- 会话ID索引，提升按会话查询的效率
CREATE INDEX idx_bw_chat_message_user_id ON bw_chat_message (user_id); -- 用户ID索引，优化按用户查询
CREATE INDEX idx_bw_chat_message_sequence ON bw_chat_message (sequence); -- 消息顺序索引，优化消息顺序查询
CREATE INDEX idx_bw_chat_message_encrypt ON bw_chat_message (complete, is_encrypt); -- 消息加密状态

-- 添加字段注释
COMMENT ON COLUMN bw_chat_message.id IS '消息的唯一标识，使用字符串形式的 ID';
COMMENT ON COLUMN bw_chat_message.space_id IS '空间ID，表示消息所属的空间';
COMMENT ON COLUMN bw_chat_message.dialog_id IS '会话ID，表示消息所属的会话';
COMMENT ON COLUMN bw_chat_message.user_id IS '用户ID，表示消息发送者的用户';
COMMENT ON COLUMN bw_chat_message.role IS '角色，1 表示用户，2 表示系统';
COMMENT ON COLUMN bw_chat_message.message IS '消息内容';
COMMENT ON COLUMN bw_chat_message.msg_type IS '消息类型，1 表示文本，2 表示图片等';
COMMENT ON COLUMN bw_chat_message.send_time IS '消息发送时间，存储为 Unix 时间戳，表示秒';
COMMENT ON COLUMN bw_chat_message.complete IS '数据是否完整，1 表示完整，2 表示不完整';
COMMENT ON COLUMN bw_chat_message.sequence IS '消息的顺序，用于排序';
COMMENT ON COLUMN bw_chat_message.msg_block IS '消息所属的块编号，用于大消息的分块处理';
COMMENT ON COLUMN bw_chat_message.attach IS '附件列表';
COMMENT ON COLUMN bw_chat_message.is_encrypt IS '消息是否已加密';
