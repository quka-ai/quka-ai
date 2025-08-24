-- 创建quka_user表
CREATE TABLE IF NOT EXISTS quka_user (
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
COMMENT ON COLUMN quka_user.id IS '用户ID，主键';
COMMENT ON COLUMN quka_user.appid IS '租户id';
COMMENT ON COLUMN quka_user.name IS '用户名';
COMMENT ON COLUMN quka_user.avatar IS '用户头像URL';
COMMENT ON COLUMN quka_user.email IS '用户邮箱，唯一约束';
COMMENT ON COLUMN quka_user.password IS '用户密码';
COMMENT ON COLUMN quka_user.salt IS '用户密码盐值';
COMMENT ON COLUMN quka_user.source IS '用户注册来源';
COMMENT ON COLUMN quka_user.plan_id IS '会员方案id';
COMMENT ON COLUMN quka_user.updated_at IS '更新时间，Unix时间戳';
COMMENT ON COLUMN quka_user.created_at IS '创建时间，Unix时间戳';


-- ================================
-- 索引设计 - 针对搜索场景优化
-- ================================

-- 原有索引：租户邮箱唯一约束
CREATE UNIQUE INDEX IF NOT EXISTS quka_user_appid_email ON quka_user (appid, email);

-- 核心索引：用户列表查询和排序优化 (简化版本)
-- 优化查询: WHERE appid = ? ORDER BY created_at DESC  
-- 说明：移除source字段，减少索引大小，appid过滤后再用source过滤成本较低
CREATE INDEX IF NOT EXISTS idx_user_appid_created 
ON quka_user (appid, created_at DESC);

-- 用户名搜索索引：支持前缀和组合搜索
-- 优化查询: WHERE appid = ? AND name LIKE '张%'
CREATE INDEX IF NOT EXISTS idx_user_appid_name 
ON quka_user (appid, name);

-- 邮箱搜索索引：支持邮箱前缀和精确搜索
-- 优化查询: WHERE appid = ? AND email LIKE 'user@%' 
CREATE INDEX IF NOT EXISTS idx_user_appid_email_search 
ON quka_user (appid, email);

-- PostgreSQL 三元组索引 - 高级模糊搜索优化（可选）
-- 显著提升 LIKE '%keyword%' 类型查询性能
-- 注意：需要先安装 pg_trgm 扩展
-- CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- CREATE INDEX IF NOT EXISTS idx_user_name_gin ON quka_user USING gin (name gin_trgm_ops);  
-- CREATE INDEX IF NOT EXISTS idx_user_email_gin ON quka_user USING gin (email gin_trgm_ops);

-- ================================
-- 索引使用说明 
-- ================================
-- 
-- 1. idx_user_appid_created: 
--    - 租户用户列表分页查询 (通用，覆盖所有source)
--    - 自动优化 ORDER BY created_at DESC
--    - source过滤在应用层或WHERE子句中处理
--
-- 2. idx_user_appid_name:
--    - 用户名搜索: name LIKE '张%' (前缀匹配高效)  
--    - 组合查询: appid + name 双字段精确定位
--
-- 3. idx_user_appid_email_search:  
--    - 邮箱域名搜索: email LIKE '%@gmail.com' 
--    - 邮箱前缀搜索: email LIKE 'user@%'
--    - 避免与唯一索引 quka_user_appid_email 重复
--
-- 4. GIN 三元组索引 (PostgreSQL):
--    - 任意位置模糊搜索: name/email LIKE '%keyword%'  
--    - 空间开销较大，但查询性能显著提升
--
-- 5. 查询优化策略:
--    - 先用 appid 大幅减少数据集
--    - 再用 source/name/email 进一步过滤  
--    - 索引更简洁，缓存命中率更高