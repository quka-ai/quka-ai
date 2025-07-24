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
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "req_1234567890"
  },
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

### 2. 获取用户列表

**接口地址**: `GET /api/v1/admin/users`

**权限要求**: 管理员权限

**功能说明**: 管理员可以查看所有用户，支持按来源过滤、用户名搜索、邮箱搜索等

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 示例 |
|--------|------|------|------|------|
| page | int | 是 | 页码，从1开始 | 1 |
| pagesize | int | 是 | 每页数量，最大50 | 20 |
| name | string | 否 | 用户名模糊搜索 | "张" |
| email | string | 否 | 邮箱模糊搜索 | "example" |
| source | string | 否 | 用户来源过滤 | "admin_created" |

**来源类型说明**:
- `admin_created`: 管理员创建的用户
- `register`: 注册用户  
- `oauth_google`: Google登录用户
- `oauth_github`: GitHub登录用户

**请求示例**:
```bash
# 查看所有用户（基本分页）
GET /api/v1/admin/users?page=1&pagesize=20

# 只查看管理员创建的用户
GET /api/v1/admin/users?page=1&pagesize=20&source=admin_created

# 只查看注册用户
GET /api/v1/admin/users?page=1&pagesize=20&source=register

# 按用户名搜索
GET /api/v1/admin/users?page=1&pagesize=20&name=张

# 按邮箱搜索  
GET /api/v1/admin/users?page=1&pagesize=20&email=gmail

# 组合搜索：管理员创建的用户中搜索
GET /api/v1/admin/users?page=1&pagesize=20&source=admin_created&name=张&email=gmail
```

**响应参数**:

| 字段名 | 类型 | 描述 |
|--------|------|------|
| list | array | 用户列表 |
| total | int64 | 总用户数 |

**用户对象字段说明**:

| 字段名 | 类型 | 描述 | 示例 |
|--------|------|------|------|
| id | string | 用户唯一ID | "usr_1234567890abcdef" |
| appid | string | 租户ID | "app_xxx" |
| name | string | 用户昵称 | "张三" |
| email | string | 用户邮箱 | "zhangsan@example.com" |
| plan_id | string | 会员方案ID | "basic" |
| global_role | string | 用户全局角色 | "role-member" |
| created_at | int64 | 创建时间戳 | 1699123456 |
| updated_at | int64 | 更新时间戳 | 1699123456 |

**全局角色说明**:
- `role-chief`: 超级管理员（拥有最高权限）
- `role-admin`: 管理员（可管理其他用户）
- `role-member`: 普通用户（默认角色）

**响应示例**:
```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "req_1234567890"
  },
  "data": {
    "list": [
      {
        "id": "usr_1234567890abcdef",
        "appid": "app_xxx",
        "name": "张三",
        "email": "zhangsan@example.com",
        "plan_id": "basic",
        "created_at": 1699123456,
        "updated_at": 1699123456,
        "global_role": "role-member"
      }
    ],
    "total": 1
  }
}
```

### 3. 删除用户

**接口地址**: `DELETE /api/v1/admin/users`

**权限要求**: 管理员权限

**功能说明**: 管理员可以删除用户及其所有相关数据，包括：

**用户级别数据：**
- 用户基本信息和全局角色记录
- 用户的所有访问令牌
- 用户的个人文件和数据记录

**空间级别数据（如果用户是空间chief）：**
- 空间基本信息
- 空间下所有用户关系

**知识库相关数据：**
- 所有知识库记录 (Knowledge)
- 知识库块数据 (KnowledgeChunk) 
- 知识库向量数据 (Vector)
- 知识库元数据 (KnowledgeMeta)
- 知识库关联元数据 (KnowledgeRelMeta)
- 内容处理任务 (ContentTask)

**聊天相关数据：**
- 所有聊天会话 (ChatSession)
- 聊天消息 (ChatMessage)
- 聊天消息扩展数据 (ChatMessageExt)
- 聊天会话固定记录 (ChatSessionPin)
- 聊天摘要 (ChatSummary)

**其他空间数据：**
- 空间资源 (Resource)
- 空间分享令牌 (ShareToken) 
- 空间静态文件 (通过FileManagement记录删除所有上传的文件)
- 文件管理记录 (FileManagement)
- 空间相关的其他配置和数据

**注意：** 如果用户只是某个空间的member而不是chief，则只删除用户与空间的关系，不删除整个空间。

**⚠️ 重要提醒**: 此操作不可逆，删除后所有数据将无法恢复！

**请求参数**:

| 字段名 | 类型 | 必填 | 描述 | 示例 |
|--------|------|------|------|------|
| user_id | string | 是 | 要删除的用户ID | "usr_1234567890abcdef" |

**请求示例**:
```json
{
  "user_id": "usr_1234567890abcdef"
}
```

**响应参数**:

| 字段名 | 类型 | 描述 |
|--------|------|------|
| user_id | string | 被删除的用户ID |
| message | string | 删除结果信息 |

**响应示例**:
```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "req_1234567890"
  },
  "data": {
    "user_id": "usr_1234567890abcdef",
    "message": "User deleted successfully"
  }
}
```

**错误码说明**:
- 400: 请求参数错误
- 403: 无权限访问，需要管理员权限
- 404: 用户不存在
- 500: 服务器内部错误

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
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "req_1234567890"
  },
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

### 2. 响应格式说明

**所有API接口都使用统一的响应格式**：
```json
{
  "meta": {
    "code": 200,           // HTTP状态码
    "message": "success",  // 响应消息
    "request_id": "req_xxx" // 请求追踪ID，用于问题定位
  },
  "data": {
    // 具体的响应数据
  }
}
```

**错误响应格式**：
```json
{
  "meta": {
    "code": 400,
    "message": "邮箱格式不正确",
    "request_id": "req_xxx"
  },
  "data": null
}
```

### 3. 请求头设置

所有请求都需要包含以下头部信息：
```
Authorization: Bearer <your_jwt_token>
Content-Type: application/json
```

### 4. 前端代码示例

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
    if (result.meta.code === 200) {
      return {
        success: true,
        user: result.data
      };
    } else {
      return {
        success: false,
        error: result.meta.message
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

#### 获取用户列表
```javascript
// 获取管理员创建的用户列表（支持搜索）
async function getUserList(page = 1, pageSize = 20, searchOptions = {}) {
  try {
    const params = new URLSearchParams({
      page: page.toString(),
      pagesize: pageSize.toString()
    });
    
    // 添加可选的搜索参数
    if (searchOptions.name) {
      params.append('name', searchOptions.name);
    }
    if (searchOptions.email) {
      params.append('email', searchOptions.email);
    }
    if (searchOptions.source) {
      params.append('source', searchOptions.source);
    }
    
    const response = await fetch(`/api/v1/admin/users?${params.toString()}`, {
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

// 使用示例
// 查看所有用户
const allUsers = await getUserList(1, 20);

// 只查看管理员创建的用户
const adminCreatedUsers = await getUserList(1, 20, { source: 'admin_created' });

// 只查看注册用户
const registerUsers = await getUserList(1, 20, { source: 'register' });

// 按用户名搜索
const usersByName = await getUserList(1, 20, { name: '张' });

// 按邮箱搜索
const usersByEmail = await getUserList(1, 20, { email: 'gmail' });

// 组合搜索：在管理员创建的用户中搜索
const filteredUsers = await getUserList(1, 20, { 
  source: 'admin_created', 
  name: '张', 
  email: 'gmail' 
});
```

#### 删除用户
```javascript
// 删除用户（谨慎操作）
async function deleteUser(userId) {
  try {
    // 建议先确认
    if (!confirm(`确定要删除用户 ${userId} 吗？此操作不可逆！`)) {
      return { success: false, error: '用户取消操作' };
    }

    const response = await fetch('/api/v1/admin/users', {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${getToken()}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ user_id: userId })
    });
    
    const result = await response.json();
    if (result.meta.code === 200) {
      return {
        success: true,
        data: result.data
      };
    } else {
      return {
        success: false,
        error: result.meta.message
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

### 5. 错误处理

建议实现统一的错误处理机制：

```javascript
function handleApiError(response) {
  const code = response.meta ? response.meta.code : response.code; // 兼容处理
  const message = response.meta ? response.meta.message : response.message;
  
  switch (code) {
    case 400:
      return '请求参数错误，请检查输入';
    case 403:
      return '无权限访问，需要管理员权限';
    case 409:
      return '邮箱已存在，请使用其他邮箱';
    case 500:
      return '服务器内部错误，请稍后重试';
    default:
      return message || '未知错误';
  }
}
```

### 6. 使用场景示例

#### 场景1：管理员创建单个用户
```javascript
// 管理员在后台创建用户
const newUser = await createUser('新用户', 'newuser@example.com');
if (newUser.success) {
  // 将AccessToken提供给用户
  console.log('用户创建成功，AccessToken:', newUser.user.access_token);
  console.log('Request ID:', newUser.user.request_id); // 可用于问题追踪
  // 可以复制到剪贴板或显示给用户
  navigator.clipboard.writeText(newUser.user.access_token);
}
```

### 7. 安全注意事项

1. **Token安全**: 生成的AccessToken等同于用户密码，应安全传输给用户
2. **权限控制**: 确保只有管理员可以调用这些接口
3. **邮箱验证**: 虽然API不做邮箱验证，但建议在实际使用前验证邮箱真实性
4. **Token有效期**: 生成的AccessToken默认有效期为999年（永久有效）

### 8. 集成测试

建议创建以下测试用例：

```javascript
// 测试创建用户
const testCases = [
  {
    name: '正常创建',
    input: { name: '测试用户', email: 'test@example.com' },
    expected: { 
      meta: { code: 200, message: 'success' },
      data: { user_id: 'usr_xxx', access_token: 'tkn_xxx' }
    }
  },
  {
    name: '重复邮箱',
    input: { name: '重复用户', email: 'test@example.com' },
    expected: { 
      meta: { code: 409, message: '邮箱已存在' },
      data: null
    }
  },
  {
    name: '无效邮箱',
    input: { name: '无效用户', email: 'invalid-email' },
    expected: { 
      meta: { code: 400, message: '邮箱格式错误' },
      data: null
    }
  }
];
```

## 版本历史
- v1.0.0: 初始版本，支持管理员创建用户功能