-- 创建bw_user表
CREATE TABLE IF NOT EXISTS bw_user (
    id VARCHAR(32) PRIMARY KEY,             -- 用户ID，主键
    appid VARCHAR(32) NOT NULL,              -- appid，租户id 
    name VARCHAR(50) NOT NULL,              -- 用户名
    avatar VARCHAR(255),                     -- 用户头像URL
    email VARCHAR(100) UNIQUE NOT NULL,      -- 用户邮箱，唯一约束
    password VARCHAR(255) NOT NULL,          -- 用户密码
    salt VARCHAR(10) NOT NULL,              -- 用户密码盐值
    source VARCHAR(50) NOT NULL,            -- 用户注册来源
    plan_id VARCHAR(20) NOT NULL,              -- 会员方案id
    updated_at BIGINT NOT NULL,              -- 更新时间，Unix时间戳
    created_at BIGINT NOT NULL               -- 创建时间，Unix时间戳
);

-- 添加字段注释
COMMENT ON COLUMN bw_user.id IS '用户ID，主键';
COMMENT ON COLUMN bw_user.appid IS '租户id';
COMMENT ON COLUMN bw_user.name IS '用户名';
COMMENT ON COLUMN bw_user.avatar IS '用户头像URL';
COMMENT ON COLUMN bw_user.email IS '用户邮箱，唯一约束';
COMMENT ON COLUMN bw_user.password IS '用户密码';
COMMENT ON COLUMN bw_user.salt IS '用户密码盐值';
COMMENT ON COLUMN bw_user.source IS '用户注册来源';
COMMENT ON COLUMN bw_user.plan_id IS '会员方案id';
COMMENT ON COLUMN bw_user.updated_at IS '更新时间，Unix时间戳';
COMMENT ON COLUMN bw_user.created_at IS '创建时间，Unix时间戳';


CREATE UNIQUE INDEX bw_user_appid_email ON bw_user (appid,email);