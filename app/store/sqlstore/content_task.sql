CREATE TABLE IF NOT EXISTS quka_content_task (
    task_id VARCHAR(32) PRIMARY KEY, -- 任务ID，32字符字符串类型，唯一标识任务
    space_id VARCHAR(32) NOT NULL, -- 空间ID，标识任务归属的空间
    user_id VARCHAR(32) NOT NULL, -- 用户ID，标识发起任务的用户
    resource VARCHAR(32) NOT NULL,
    meta_info TEXT NOT NULL,
    file_url TEXT NOT NULL, -- 文件URL，任务需要处理的文件路径
    file_name VARCHAR(255) NOT NULL, -- 文件名，任务需要处理的文件名称
    ai_file_id VARCHAR(255) NOT NULL, -- ai 服务中该文件对应的id
    step INT NOT NULL, -- 任务的当前阶段，例如：1-待处理，2-处理中，3-已完成等
    retry_times INT NOT NULL, -- 任务重试次数
    task_type VARCHAR(255) NOT NULL, -- 任务类型，表示任务的目的或用途，例如：'文本切割'，'数据清洗'等
    created_at BIGINT NOT NULL, -- 任务创建时间，时间戳格式
    updated_at BIGINT NOT NULL, -- 任务更新时间，时间戳格式
    CONSTRAINT bw_content_task_unique UNIQUE (task_id) -- 确保task_id唯一
);

-- 添加字段注释
COMMENT ON COLUMN bw_content_task.task_id IS '任务ID，32字符字符串类型，唯一标识任务';
COMMENT ON COLUMN bw_content_task.space_id IS '空间ID，标识任务归属的空间';
COMMENT ON COLUMN bw_content_task.user_id IS '用户ID，标识发起任务的用户';
COMMENT ON COLUMN bw_knowledge.resource IS 'knowledge的资源类型';
COMMENT ON COLUMN bw_knowledge.meta_info IS '用户自定义meta信息，空则使用文件名填充';
COMMENT ON COLUMN bw_content_task.file_url IS '文件URL，任务需要处理的文件路径';
COMMENT ON COLUMN bw_content_task.file_name IS '文件名，任务需要处理的文件名称';
COMMENT ON COLUMN bw_content_task.ai_file_id IS 'ai 服务中该文件对应的id';
COMMENT ON COLUMN bw_content_task.step IS '任务的当前阶段，例如：1-待处理，2-处理中，3-已完成等';
COMMENT ON COLUMN bw_content_task.task_type IS '任务类型，表示任务的目的或用途，例如：\'文本切割\'，\'数据清洗\'等';
COMMENT ON COLUMN bw_content_task.retry_times IS '失败重试次数';
COMMENT ON COLUMN bw_content_task.created_at IS '任务创建时间，时间戳格式';
COMMENT ON COLUMN bw_content_task.updated_at IS '任务更新时间，时间戳格式';

-- 创建索引加速查询
CREATE INDEX IF NOT EXISTS idx_quka_content_task_space_user ON quka_content_task (space_id, user_id);
CREATE INDEX IF NOT EXISTS idx_quka_content_task_step ON quka_content_task (step);
CREATE INDEX IF NOT EXISTS idx_quka_content_task_created_at ON quka_content_task (created_at);
