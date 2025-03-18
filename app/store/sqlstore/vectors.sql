-- 创建表
CREATE TABLE bw_vectors (
    id VARCHAR(32) PRIMARY KEY,
    knowledge_id VARCHAR(32) NOT NULL,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    resource VARCHAR(32) NOT NULL,
    embedding vector(1024) NOT NULL,
    original_length INT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 添加字段注释
COMMENT ON COLUMN bw_vectors.id IS '主键，自增ID';
COMMENT ON COLUMN bw_vectors.space_id IS '空间ID，用于标识所属空间';
COMMENT ON COLUMN bw_vectors.user_id IS '用户ID，用于标识向量所属用户';
COMMENT ON COLUMN bw_vectors.embedding IS '文本向量，存储经过编码后的文本向量表示';
COMMENT ON COLUMN bw_vectors.resource IS '资源类型';
COMMENT ON COLUMN bw_knowledge_chunk.original_length IS '关联知识点长度';
COMMENT ON COLUMN bw_vectors.created_at IS '创建时间，UNIX时间戳';
COMMENT ON COLUMN bw_vectors.updated_at IS '更新时间，UNIX时间戳';


CREATE INDEX idx_vectors_space_id_resource_knowledge_id ON bw_vectors (space_id, resource, knowledge_id);
CREATE INDEX idx_vectors_embedding ON bw_vectors USING hnsw (embedding vector_ip_ops) WITH (m = 32, ef_construction = 128);