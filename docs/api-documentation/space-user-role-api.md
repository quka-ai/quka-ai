# Space 用户角色设置 API 文档

## 接口概述
设置或更新指定 Space 中某个用户的角色权限。

## 基本信息
- **接口路径**: `/api/v1/space/:spaceid/user/role`
- **请求方法**: `PUT`
- **权限要求**: 需要 `modify_space` 权限（即当前用户必须是该 Space 的管理员）
- **认证方式**: JWT Token（Bearer Token）

## 路径参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| spaceid | string | 是 | Space ID，要设置用户角色的空间标识 |

## 请求体参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| user_id | string | 是 | 要设置角色的用户 ID |
| role | string | 是 | 角色类型，可选值：`role-editor` 或 `role-viewer` |

### 角色说明

| 角色值 | 角色名称 | 权限描述 |
|--------|----------|----------|
| role-viewer | 查看者 | 只能查看 Space 内容，无编辑权限 |
| role-editor | 编辑者 | 可以查看和编辑 Space 内容，继承查看者权限 |

> **注意**: 只有 Space 管理员（`role-admin`）才能调用此接口设置其他用户的角色

## 请求示例

### cURL 示例
```bash
curl -X PUT "https://api.example.com/api/v1/space/space_123456/user/role" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_789012",
    "role": "role-editor"
  }'
```

### JavaScript/Fetch 示例
```javascript
const response = await fetch('https://api.example.com/api/v1/space/space_123456/user/role', {
  method: 'PUT',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    user_id: 'user_789012',
    role: 'role-editor'
  })
});

const result = await response.json();
```

### Axios 示例
```javascript
import axios from 'axios';

const response = await axios.put(
  '/api/v1/space/space_123456/user/role',
  {
    user_id: 'user_789012',
    role: 'role-editor'
  },
  {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  }
);
```

## 响应说明

### 成功响应
**状态码**: `200 OK`

```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

### 错误响应

#### 1. 参数错误（400 Bad Request）
```json
{
  "code": 400,
  "msg": "invalid argument",
  "data": null
}
```

**可能原因**:
- `user_id` 或 `role` 参数缺失
- `role` 值不是 `role-editor` 或 `role-viewer`

#### 2. 未授权（401 Unauthorized）
```json
{
  "code": 401,
  "msg": "unauthorized",
  "data": null
}
```

**原因**: Token 无效或未提供

#### 3. 权限不足（403 Forbidden）
```json
{
  "code": 403,
  "msg": "permission denied",
  "data": null
}
```

**原因**: 当前用户不是该 Space 的管理员，无权设置其他用户的角色

#### 4. 服务器错误（500 Internal Server Error）
```json
{
  "code": 500,
  "msg": "internal server error",
  "data": null
}
```

**原因**: 服务器内部错误

## 业务逻辑说明

1. **权限验证**:
   - 接口会验证调用者是否为该 Space 的管理员
   - 只有管理员才能设置其他用户的角色

2. **角色设置逻辑**:
   - 如果目标用户在该 Space 中不存在角色记录，则创建新的用户-空间关联记录
   - 如果目标用户已有角色，则更新为新的角色

3. **角色层级**:
   ```
   role-admin (管理员)
     └── role-editor (编辑者)
           └── role-viewer (查看者)
                 └── role-member (成员)
   ```
   - 编辑者继承查看者的所有权限
   - 管理者继承编辑者的所有权限

## 使用场景

1. **添加新成员**: 邀请新用户加入 Space 并设置其角色
2. **权限升级**: 将查看者升级为编辑者
3. **权限降级**: 将编辑者降级为查看者
4. **协作管理**: 管理团队成员的访问权限

## 注意事项

1. ✅ 只有 Space 管理员可以调用此接口
2. ✅ 不能将用户设置为管理员角色（需要其他接口）
3. ✅ 如果用户之前没有访问该 Space 的权限，此接口会创建新的权限记录
4. ✅ 可以重复调用来更新用户角色
5. ⚠️ 建议在前端添加二次确认，避免误操作

## 相关接口

- `GET /api/v1/space/:spaceid/users` - 获取 Space 的所有用户列表
- `DELETE /api/v1/space/:spaceid/user/:userid` - 移除 Space 中的用户
- `GET /api/v1/space/:spaceid` - 获取 Space 详情

## 更新记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2025-12-18 | v1.0 | 初始版本 |
