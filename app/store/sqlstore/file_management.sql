CREATE TABLE IF NOT EXISTS bw_file_management (
    id BIGSERIAL PRIMARY KEY,                      -- 文件记录的唯一标识
    space_id VARCHAR(32) NOT NULL,             -- 关联空间id
    user_id VARCHAR NOT NULL,                       -- 关联的用户ID，用于区分每个用户的文件数据
    file VARCHAR(255) NOT NULL,                      -- 存储文件的路径，用于定位文件内容
    file_size BIGINT NOT NULL,                    -- 文件大小，单位为字节
    object_type VARCHAR(50) NOT NULL,             -- 文件所属的功能模块，例如“用户头像”
    kind VARCHAR(20) NOT NULL,                    -- 文件类型，例如“image”、“file”
    status SMALLINT NOT NULL DEFAULT 1,           -- 文件的状态，1表示可用，2表示已删除
    created_at BIGINT NOT NULL                    -- 记录文件的上传时间
);

-- 为表和字段添加注释
COMMENT ON TABLE bw_file_management IS '管理用户上传的文件信息';
COMMENT ON COLUMN bw_file_management.id IS '文件记录的唯一标识';
COMMENT ON COLUMN bw_file_management.space_id IS '关联空间id';
COMMENT ON COLUMN bw_file_management.user_id IS '关联的用户ID，用于区分每个用户的文件数据';
COMMENT ON COLUMN bw_file_management.file IS '文件的路径及名称';
COMMENT ON COLUMN bw_file_management.file_size IS '文件大小，单位为字节';
COMMENT ON COLUMN bw_file_management.object_type IS '文件所属的功能模块，例如“knowledge”';
COMMENT ON COLUMN bw_file_management.kind IS '文件类型，例如“image”、“file”';
COMMENT ON COLUMN bw_file_management.status IS '文件的状态，1表示可用，2表示已删除';
COMMENT ON COLUMN bw_file_management.created_at IS '记录文件的上传时间';

-- 添加唯一约束
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_id_file_name ON bw_file_management (user_id, file);