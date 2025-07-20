# 管理员创建用户API文档

## 功能概述
管理员可以通过这些API接口创建用户并管理用户访问令牌(AccessToken)。创建后的用户可以直接使用生成的AccessToken登录系统，无需注册流程。

## 接口列表

### 1. 创建单个用户

**接口地址**: `POST /api/v1/admin/users`

**权限要求**: 管理员权限

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 示例 |
|--------|------|------|------|------|
| name | string | 是 | 用户昵称，1-50字符 | "张三" |
| email | string | 是 | 用户邮箱地址，需唯一 | "zhangsan@example.com" |

**请求示例**:
```json
{
  "name": "张三",
  "email": "zhangsan@example.com"
}
```

**响应参数**:

| 字段名 | 类型 | 描述 | 示例 |
|--------|------|------|------|
| user_id | string | 新创建用户的唯一ID | "usr_1234567890abcdef" |
| name | string | 用户昵称 | "张三" |
| email | string | 用户邮箱 | "zhangsan@example.com" |
| access_token | string | 可直接使用的访问令牌 | "tkn_abcdef1234567890" |
| created_at | int64 | 创建时间戳 | 1699123456 |

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "user_id": "usr_1234567890abcdef",
    "name": "张三",
    "email": "zhangsan@example.com",
    "access_token": "tkn_abcdef1234567890",
    "created_at": 1699123456
  }
}
```

**错误码说明**:
- 400: 请求参数错误，如邮箱格式不正确
- 403: 无权限访问，需要管理员权限
- 409: 邮箱已存在
- 500: 服务器内部错误

### 2. 批量创建用户

**接口地址**: `POST /api/v1/admin/users/batch`

**权限要求**: 管理员权限

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 限制 |
|--------|------|------|------|------|
| users | array | 是 | 用户列表 | 最多100个用户 |

**请求示例**:
```json
{
  "users": [
    {
      "name": "张三",
      "email": "zhangsan@example.com"
    },
    {
      "name": "李四",
      "email": "lisi@example.com"
    }
  ]
}
```

**响应参数**:

| 字段名 | 类型 | 描述 |
|--------|------|------|
| success_count | int | 成功创建的用户数量 |
| failed_count | int | 创建失败的用户数量 |
| results | array | 成功创建的用户详情 |
| errors | array | 创建失败的用户错误信息 |

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "success_count": 2,
    "failed_count": 0,
    "results": [
      {
        "name": "张三",
        "email": "zhangsan@example.com",
        "user_id": "usr_1234567890abcdef",
        "access_token": "tkn_abcdef1234567890",
        "status": "success"
      },
      {
        "name": "李四",
        "email": "lisi@example.com",
        "user_id": "usr_abcdef1234567890",
        "access_token": "tkn_1234567890abcdef",
        "status": "success"
      }
    ],
    "errors": []
  }
}
```

### 3. 获取管理员创建的用户列表

**接口地址**: `GET /api/v1/admin/users`

**权限要求**: 管理员权限

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 示例 |
|--------|------|------|------|------|
| page | int | 是 | 页码，从1开始 | 1 |
| pagesize | int | 是 | 每页数量，最大50 | 20 |

**请求示例**:
```
GET /api/v1/admin/users?page=1&pagesize=20
```

**响应参数**:

| 字段名 | 类型 | 描述 |
|--------|------|------|
| list | array | 用户列表 |
| total | int64 | 总用户数 |

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "usr_1234567890abcdef",
        "appid": "app_xxx",
        "name": "张三",
        "email": "zhangsan@example.com",
        "source": "admin_created",
        "created_at": 1699123456,
        "updated_at": 1699123456
      }
    ],
    "total": 1
  }
}
```

### 4. 重新生成用户AccessToken

**接口地址**: `POST /api/v1/admin/users/token`

**权限要求**: 管理员权限

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 示例 |
|--------|------|------|------|------|
| user_id | string | 是 | 用户ID | "usr_1234567890abcdef" |

**请求示例**:
```json
{
  "user_id": "usr_1234567890abcdef"
}
```

**响应参数**:

| 字段名 | 类型 | 描述 |
|--------|------|------|
| user_id | string | 用户ID |
| access_token | string | 新生成的访问令牌 |

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "user_id": "usr_1234567890abcdef",
    "access_token": "tkn_newtoken1234567890"
  }
}
```

## 前端对接指南

### 1. 准备工作

在调用管理员API之前，确保：
- 当前用户已获得管理员权限
- 已获取有效的JWT访问令牌
- 了解相关的错误处理机制

### 2. 请求头设置

所有请求都需要包含以下头部信息：
```
Authorization: Bearer <your_jwt_token>
Content-Type: application/json
```

### 3. 前端代码示例

#### 创建单个用户
```javascript
// 创建单个用户
async function createUser(name, email) {
  try {
    const response = await fetch('/api/v1/admin/users', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${getToken()}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ name, email })
    });
    
    const result = await response.json();
    if (result.code === 200) {
      return {
        success: true,
        user: result.data
      };
    } else {
      return {
        success: false,
        error: result.message
      };
    }
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
}
```

#### 批量创建用户
```javascript
// 批量创建用户
async function batchCreateUsers(users) {
  try {
    const response = await fetch('/api/v1/admin/users/batch', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${getToken()}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ users })
    });
    
    const result = await response.json();
    return result.data;
  } catch (error) {
    console.error('批量创建用户失败:', error);
    throw error;
  }
}
```

#### 获取用户列表
```javascript
// 获取管理员创建的用户列表
async function getUserList(page = 1, pageSize = 20) {
  try {
    const response = await fetch(`/api/v1/admin/users?page=${page}&pagesize=${pageSize}`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${getToken()}`
      }
    });
    
    const result = await response.json();
    return result.data;
  } catch (error) {
    console.error('获取用户列表失败:', error);
    throw error;
  }
}
```

#### 重新生成AccessToken
```javascript
// 重新生成用户AccessToken
async function regenerateToken(userId) {
  try {
    const response = await fetch('/api/v1/admin/users/token', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${getToken()}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ user_id: userId })
    });
    
    const result = await response.json();
    return result.data;
  } catch (error) {
    console.error('重新生成Token失败:', error);
    throw error;
  }
}
```

### 4. 错误处理

建议实现统一的错误处理机制：

```javascript
function handleApiError(response) {
  switch (response.code) {
    case 400:
      return '请求参数错误，请检查输入';
    case 403:
      return '无权限访问，需要管理员权限';
    case 409:
      return '邮箱已存在，请使用其他邮箱';
    case 500:
      return '服务器内部错误，请稍后重试';
    default:
      return response.message || '未知错误';
  }
}
```

### 5. 使用场景示例

#### 场景1：管理员创建单个用户
```javascript
// 管理员在后台创建用户
const newUser = await createUser('新用户', 'newuser@example.com');
if (newUser.success) {
  // 将AccessToken提供给用户
  console.log('用户创建成功，AccessToken:', newUser.user.access_token);
  // 可以复制到剪贴板或显示给用户
  navigator.clipboard.writeText(newUser.user.access_token);
}
```

#### 场景2：管理员批量创建用户
```javascript
// 从Excel导入用户列表
const users = [
  { name: '张三', email: 'zhangsan@example.com' },
  { name: '李四', email: 'lisi@example.com' }
];

const result = await batchCreateUsers(users);
console.log(`成功创建 ${result.success_count} 个用户，失败 ${result.failed_count} 个`);

// 显示结果
result.results.forEach(user => {
  console.log(`用户 ${user.name} 创建成功，Token: ${user.access_token}`);
});

if (result.errors.length > 0) {
  console.log('创建失败的用户:', result.errors);
}
```

### 6. 安全注意事项

1. **Token安全**: 生成的AccessToken等同于用户密码，应安全传输给用户
2. **权限控制**: 确保只有管理员可以调用这些接口
3. **邮箱验证**: 虽然API不做邮箱验证，但建议在实际使用前验证邮箱真实性
4. **Token有效期**: 生成的AccessToken默认有效期为999年（永久有效）

### 7. 集成测试

建议创建以下测试用例：

```javascript
// 测试创建用户
const testCases = [
  {
    name: '正常创建',
    input: { name: '测试用户', email: 'test@example.com' },
    expected: { success: true }
  },
  {
    name: '重复邮箱',
    input: { name: '重复用户', email: 'test@example.com' },
    expected: { success: false, error: '邮箱已存在' }
  },
  {
    name: '无效邮箱',
    input: { name: '无效用户', email: 'invalid-email' },
    expected: { success: false, error: '邮箱格式错误' }
  }
];
```

## 版本历史
- v1.0.0: 初始版本，支持管理员创建用户功能