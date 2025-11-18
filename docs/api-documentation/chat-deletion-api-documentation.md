# 聊天记录删除 API 文档

## 📋 文档说明

本文档详细描述了 QukaAI 系统中聊天记录删除相关 API 的接口规范、使用方法和前端对接方式。

## 🎯 API 概览

| 接口     | 方法   | 路径                             | 功能             |
| -------- | ------ | -------------------------------- | ---------------- |
| 单个删除 | DELETE | `/api/v1/:spaceid/chat/:session` | 删除单个聊天记录 |

## 🔑 认证方式

所有 API 都需要 JWT Token 认证，在请求头中添加：

```
Authorization: your-jwt-token
```

## 📖 详细接口文档

### 单个聊天记录删除

**接口信息**

- **URL**: `/api/v1/:spaceid/chat/:session`
- **方法**: `DELETE`
- **功能**: 删除单个聊天记录及其所有关联数据（消息、扩展信息、摘要等）

**路径参数**
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| spaceid | string | 是 | 空间 ID |
| session | string | 是 | 聊天记录 ID |

**请求示例**

```bash
curl -X DELETE 'https://api.example.com/api/v1/space123/chat/session456' \
  -H 'Authorization: Bearer your-jwt-token'
```

**成功响应**

```json
{
  "meta": {
    "code": 0,
    "message": "",
    "request_id": "abc123def456"
  },
  "data": null
}
```

**错误响应**

```json
{
  "meta": {
    "code": 404,
    "message": "Chat session not found",
    "request_id": "abc123def456"
  },
  "data": null
}
```

**状态码说明**

- `200`: 删除成功
- `400`: 参数错误
- `403`: 权限不足（非会话所有者）
- `404`: 聊天记录不存在

---

## 📱 前端对接指南

### 1. 单个删除操作

```javascript
/**
 * 删除单个聊天记录
 * @param {string} spaceId - 空间ID
 * @param {string} sessionId - 聊天记录ID
 * @returns {Promise<boolean>} - 是否删除成功
 */
async function deleteSingleChatSession(spaceId, sessionId) {
  try {
    const response = await fetch(
      `${API_BASE_URL}/${spaceId}/chat/${sessionId}`,
      {
        method: "DELETE",
        headers: getHeaders(),
      }
    );

    const result = await response.json();

    if (response.ok) {
      console.log("聊天记录删除成功");
      return true;
    } else {
      throw new Error(result.meta?.message || "删除失败");
    }
  } catch (error) {
    console.error("删除失败:", error);
    throw error;
  }
}

// 使用示例
try {
  await deleteSingleChatSession("space123", "session456");
  // 更新本地状态
  refreshChatList();
} catch (error) {
  // 错误处理
  showToast(error.message);
}
```

## ⚠️ 注意事项

1. **权限控制**: 用户只能删除自己创建的聊天记录
2. **数据删除**: 删除操作会同时删除聊天记录下的所有消息、扩展信息、摘要等
3. **单个删除**: 当前只支持单个删除，批量删除需要通过循环调用实现
4. **性能考虑**: 大量删除时建议分批处理，避免同时删除过多记录
5. **撤销操作**: 删除操作不可撤销，请在前端进行二次确认
6. **并发限制**: 建议控制并发数量，避免对服务器造成过大压力

## 🐛 常见问题

1. **403 错误**: 用户没有权限删除该聊天记录
2. **404 错误**: 聊天记录不存在或已被删除
3. **400 错误**: 请求参数格式错误或 session_ids 为空
4. **网络错误**: 检查网络连接和 Token 有效性

## 📞 技术支持

如有问题，请联系后端开发团队。

---

**文档版本**: v1.1
**更新日期**: 2025-11-16
**更新内容**: 修正响应格式为实际的 meta/data 结构
**作者**: 后端开发团队
