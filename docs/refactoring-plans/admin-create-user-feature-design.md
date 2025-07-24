# 管理员创建用户功能设计方案

## 🎯 功能概述

设计一个管理员专用的用户创建功能，允许系统管理员通过API创建新用户，并生成可直接使用的AccessToken。新创建的用户可以通过该Token直接登录系统，无需注册流程。

## 📋 需求分析

### 功能需求
- ✅ 仅限管理员调用此API
- ✅ 输入：用户昵称、邮箱
- ✅ 输出：新生成的AccessToken
- ✅ 创建用户后自动生成AccessToken
- ✅ Token可直接用于登录系统

### 非功能需求
- 🔒 严格的权限验证
- 🛡️ 邮箱格式验证和唯一性检查
- 📊 操作日志记录
- 🎯 友好的错误提示
- ⚡ 高性能的Token生成

## 🏗️ 技术方案

### 1. 用户表结构分析

**现有用户表 (`quka_user`)**:
```sql
CREATE TABLE IF NOT EXISTS quka_user (
    id VARCHAR(32) PRIMARY KEY,
    appid VARCHAR(32) NOT NULL,
    name VARCHAR(50) NOT NULL,
    avatar VARCHAR(255),
    email VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    salt VARCHAR(10) NOT NULL,
    source VARCHAR(50) NOT NULL,
    plan_id VARCHAR(20) NOT NULL,
    updated_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL
);
```

### 2. 新用户创建流程

**管理员创建用户流程**:
1. 管理员调用API并提供昵称+邮箱
2. 系统验证管理员权限
3. 验证邮箱格式和唯一性
4. 生成随机密码（无需用户知晓）
5. 生成用户ID和AccessToken
6. 创建用户记录
7. 创建默认空间
8. 返回AccessToken

### 3. API接口设计

#### 接口信息
- **URL**: `/api/v1/admin/users`
- **方法**: `POST`
- **权限**: 仅管理员
- **功能**: 管理员创建新用户

#### 请求参数
```json
{
  "name": "张三",
  "email": "zhangsan@example.com"
}
```

#### 响应格式
```json
{
  "success": true,
  "data": {
    "user_id": "user123456",
    "name": "张三",
    "email": "zhangsan@example.com",
    "access_token": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
    "created_at": 1699123456
  }
}
```

#### 错误响应
```json
{
  "success": false,
  "error": {
    "code": "ERROR_INVALIDARGUMENT",
    "message": "邮箱格式不正确"
  }
}
```

### 4. 数据库扩展设计

#### 新增用户来源类型
在现有`source`字段基础上，新增来源类型：
- `admin_created`: 管理员创建的用户

### 5. 技术实现细节

#### 5.1 权限验证机制
- **管理员权限检查**: 使用现有的`PermissionAdmin`权限验证
- **Token验证**: 使用现有的JWT Token验证机制
- **请求来源**: 仅允许来自管理员账户的请求

#### 5.2 用户创建逻辑
```go
// 伪代码实现
func (l *AdminUserLogic) CreateUser(name, email string) (*CreateUserResult, error) {
    // 1. 验证管理员权限
    if !l.IsAdmin() {
        return nil, errors.New("permission denied")
    }
    
    // 2. 验证邮箱格式
    if !isValidEmail(email) {
        return nil, errors.New("invalid email format")
    }
    
    // 3. 检查邮箱是否已存在
    if userExists(email) {
        return nil, errors.New("email already exists")
    }
    
    // 4. 创建用户
    userID := utils.GenSpecIDStr()
    salt := utils.GenRandomString(10)
    randomPassword := utils.GenRandomString(16) // 随机密码，用户无需知道
    
    user := types.User{
        ID:        userID,
        Appid:     l.GetAppid(),
        Name:      name,
        Email:     email,
        Password:  hashPassword(randomPassword, salt),
        Salt:      salt,
        Source:    "admin_created",
        PlanID:    "basic", // 默认基础方案
        CreatedAt: time.Now().Unix(),
        UpdatedAt: time.Now().Unix(),
    }
    
    // 5. 保存用户
    if err := l.store.UserStore().Create(user); err != nil {
        return nil, err
    }
    
    // 6. 生成AccessToken
    token := generateAccessToken(userID, l.GetAppid())
    
    // 7. 创建默认空间
    spaceID := createDefaultSpace(userID)
    
    return &CreateUserResult{
        UserID:      userID,
        Name:        name,
        Email:       email,
        AccessToken: token,
        CreatedAt:   user.CreatedAt,
    }, nil
}
```

### 6. API路由设计

#### 6.1 路由注册
在现有管理员路由组中添加：
```go
// 在 /cmd/service/router.go 中
admin := authed.Group("/admin")
{
    adminUsers := admin.Group("/users")
    {
        adminUsers.POST("", s.AdminCreateUser) // 管理员创建用户
    }
}
```

#### 6.2 权限中间件
```go
// 使用现有的管理员权限验证
admin.Use(middleware.VerifyAdminPermission(s.Core))
```

### 7. 数据库操作

#### 7.1 用户创建事务
使用数据库事务确保数据一致性：
- 创建用户记录
- 生成AccessToken
- 创建默认空间
- 设置用户角色

#### 7.2 错误处理
- 邮箱重复检查
- 格式验证
- 事务回滚机制

### 8. 安全考虑

#### 8.1 密码安全
- 使用随机生成的强密码
- 密码使用bcrypt加密存储
- 用户无需知道密码，通过Token登录

#### 8.2 Token安全
- 使用足够长度的随机Token
- Token存储在数据库中
- 支持Token失效机制

#### 8.3 权限控制
- 严格的权限验证
- 操作日志记录
- 防止越权访问

### 9. 前端对接方案

#### 9.1 管理界面
- 管理员专用的用户管理界面
- 显示已创建用户列表
- 一键复制AccessToken
- 用户状态管理

#### 9.2 错误提示
- 邮箱格式错误提示
- 邮箱已存在提示
- 权限不足提示
- 网络错误处理

### 10. 扩展功能

#### 10.1 用户管理
- 查看已创建用户列表
- 禁用/启用用户
- 重新生成AccessToken
- 修改用户信息

### 11. 测试方案

#### 11.1 单元测试
- 权限验证测试
- 邮箱格式验证测试
- 重复邮箱检查测试
- Token生成测试

#### 11.2 集成测试
- 完整用户创建流程测试
- 权限边界测试
- 错误处理测试
- 并发创建测试

### 12. 部署考虑

#### 12.1 兼容性
- 不影响现有用户系统
- 向后兼容
- 无破坏性变更