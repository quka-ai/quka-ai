# 聊天记录删除API文档

## 📋 文档说明
本文档详细描述了QukaAI系统中聊天记录删除相关API的接口规范、使用方法和前端对接方式。

## 🎯 API概览

| 接口 | 方法 | 路径 | 功能 |
|---|---|---|---|
| 单个删除 | DELETE | `/api/v1/:spaceid/chat/:session` | 删除单个聊天记录 |
| 获取列表 | GET | `/api/v1/:spaceid/chat/list` | 获取聊天记录列表 |

## 🔑 认证方式
所有API都需要JWT Token认证，在请求头中添加：
```
Authorization: your-jwt-token
```

## 📖 详细接口文档

### 1. 单个聊天记录删除

**接口信息**
- **URL**: `/api/v1/:spaceid/chat/:session`
- **方法**: `DELETE`
- **功能**: 删除单个聊天记录及其所有关联数据（消息、扩展信息、摘要等）

**路径参数**
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| spaceid | string | 是 | 空间ID |
| session | string | 是 | 聊天记录ID |

**请求示例**
```bash
curl -X DELETE 'https://api.example.com/api/v1/space123/chat/session456' \
  -H 'Authorization: Bearer your-jwt-token'
```

**成功响应**
```json
{
  "success": true,
  "data": null
}
```

**错误响应**
```json
{
  "success": false,
  "error": {
    "code": "ERROR_NOT_FOUND",
    "message": "Chat session not found"
  }
}
```

**状态码说明**
- `200`: 删除成功
- `400`: 参数错误
- `403`: 权限不足（非会话所有者）
- `404`: 聊天记录不存在

---


### 2. 获取聊天记录列表

**接口信息**
- **URL**: `/api/v1/:spaceid/chat/list`
- **方法**: `GET`
- **功能**: 获取用户聊天记录列表，支持分页

**路径参数**
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| spaceid | string | 是 | 空间ID |

**查询参数**
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| page | number | 是 | 页码，从1开始 |
| pagesize | number | 是 | 每页数量，最大50 |

**请求示例**
```bash
curl -X GET 'https://api.example.com/api/v1/space123/chat/list?page=1&pagesize=20' \
  -H 'Authorization: Bearer your-jwt-token'
```

**成功响应**
```json
{
  "success": true,
  "data": {
    "list": [
      {
        "id": "session1",
        "user_id": "user123",
        "space_id": "space123",
        "title": "关于Go语言的问题",
        "type": 1,
        "status": 1,
        "created_at": 1699123456,
        "latest_access_time": 1699123890
      }
    ],
    "total": 25
  }
}
```

**响应字段说明**
| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 聊天记录ID |
| title | string | 聊天记录标题 |
| type | number | 会话类型：1=私聊，2=群聊 |
| status | number | 状态：1=活跃，2=已结束 |
| created_at | number | 创建时间戳 |
| latest_access_time | number | 最后访问时间戳 |

## 📱 前端对接指南

### 基础配置

```javascript
// 基础请求配置
const API_BASE_URL = '/api/v1';

const getHeaders = () => ({
  'Authorization': `Bearer ${getToken()}`,
  'Content-Type': 'application/json'
});
```

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
    const response = await fetch(`${API_BASE_URL}/${spaceId}/chat/${sessionId}`, {
      method: 'DELETE',
      headers: getHeaders()
    });
    
    const result = await response.json();
    
    if (result.success) {
      console.log('聊天记录删除成功');
      return true;
    } else {
      throw new Error(result.error?.message || '删除失败');
    }
  } catch (error) {
    console.error('删除失败:', error);
    throw error;
  }
}

// 使用示例
try {
  await deleteSingleChatSession('space123', 'session456');
  // 更新本地状态
  refreshChatList();
} catch (error) {
  // 错误处理
  showToast(error.message);
}
```

### 2. 循环删除多个聊天记录

由于目前只支持单个删除，可以通过循环实现批量删除功能：

```javascript
/**
 * 循环删除多个聊天记录（基于现有单个删除接口）
 * @param {string} spaceId - 空间ID
 * @param {string[]} sessionIds - 聊天记录ID数组
 * @returns {Promise<Object>} - 删除结果汇总
 */
async function deleteMultipleChatSessions(spaceId, sessionIds) {
  if (!sessionIds || sessionIds.length === 0) {
    throw new Error('请选择要删除的聊天记录');
  }

  const results = {
    total: sessionIds.length,
    success: 0,
    failed: [],
    errors: []
  };

  // 使用Promise.allSettled来并行处理，但保持错误隔离
  const promises = sessionIds.map(sessionId => 
    deleteSingleChatSession(spaceId, sessionId)
      .then(() => {
        results.success++;
        return { sessionId, success: true };
      })
      .catch(error => {
        results.failed.push(sessionId);
        results.errors.push({ sessionId, error: error.message });
        return { sessionId, success: false, error: error.message };
      })
  );

  await Promise.allSettled(promises);
  
  console.log(`删除完成：成功 ${results.success} 个，失败 ${results.failed.length} 个`);
  return results;
}

// 使用示例（带进度提示）
const handleMultipleDelete = async (selectedSessions) => {
  if (selectedSessions.length === 0) return;
  
  try {
    setLoading(true);
    setProgress(0);
    
    const result = await deleteMultipleChatSessions('space123', selectedSessions);
    
    // 更新UI
    if (result.failed.length === 0) {
      showToast(`成功删除 ${result.success} 个聊天记录`);
    } else {
      showToast(`成功删除 ${result.success} 个，${result.failed.length} 个失败`);
      console.log('失败详情:', result.errors);
    }
    
    // 刷新列表
    refreshChatList();
  } catch (error) {
    showToast(error.message);
  } finally {
    setLoading(false);
    setProgress(0);
  }
};
```

### 3. 获取聊天记录列表

```javascript
/**
 * 获取聊天记录列表
 * @param {string} spaceId - 空间ID
 * @param {number} page - 页码
 * @param {number} pageSize - 每页数量
 * @returns {Promise<Object>} - 聊天记录列表
 */
async function getChatSessions(spaceId, page = 1, pageSize = 20) {
  try {
    const params = new URLSearchParams({
      page: page.toString(),
      pagesize: pageSize.toString()
    });
    
    const response = await fetch(`${API_BASE_URL}/${spaceId}/chat/list?${params}`, {
      headers: getHeaders()
    });
    
    const result = await response.json();
    
    if (result.success) {
      return {
        sessions: result.data.list,
        total: result.data.total,
        page,
        pageSize
      };
    } else {
      throw new Error(result.error?.message || '获取列表失败');
    }
  } catch (error) {
    console.error('获取聊天记录列表失败:', error);
    throw error;
  }
}

// 使用示例
const loadChatList = async () => {
  try {
    setLoading(true);
    const data = await getChatSessions('space123', currentPage, 20);
    setChatSessions(data.sessions);
    setTotalSessions(data.total);
  } catch (error) {
    showToast(error.message);
  } finally {
    setLoading(false);
  }
};
```

### 4. React Hook 示例

```javascript
// 使用React Hook的完整示例
import { useState, useCallback } from 'react';

const useChatDeletion = (spaceId) => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const deleteSessions = useCallback(async (sessionIds) => {
    setLoading(true);
    setError(null);
    
    try {
      const result = await deleteChatSessions(spaceId, sessionIds);
      return result;
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [spaceId]);

  const deleteSingleSession = useCallback(async (sessionId) => {
    setLoading(true);
    setError(null);
    
    try {
      await deleteSingleChatSession(spaceId, sessionId);
      return true;
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [spaceId]);

  return {
    deleteSessions,
    deleteSingleSession,
    loading,
    error
  };
};

// 使用示例
const ChatListComponent = () => {
  const { deleteSessions, deleteSingleSession, loading, error } = useChatDeletion('space123');
  const [selectedSessions, setSelectedSessions] = useState([]);

  const handleDelete = async () => {
    if (selectedSessions.length === 0) return;
    
    try {
      await deleteSessions(selectedSessions);
      setSelectedSessions([]);
      // 刷新列表
    } catch (err) {
      // 错误已处理
    }
  };

  return (
    <div>
      {/* 聊天记录列表和删除按钮 */}
    </div>
  );
};
```

## ⚠️ 注意事项

1. **权限控制**: 用户只能删除自己创建的聊天记录
2. **数据删除**: 删除操作会同时删除聊天记录下的所有消息、扩展信息、摘要等
3. **单个删除**: 当前只支持单个删除，批量删除需要通过循环调用实现
4. **性能考虑**: 大量删除时建议分批处理，避免同时删除过多记录
5. **撤销操作**: 删除操作不可撤销，请在前端进行二次确认
6. **并发限制**: 建议控制并发数量，避免对服务器造成过大压力

## 🐛 常见问题

1. **403错误**: 用户没有权限删除该聊天记录
2. **404错误**: 聊天记录不存在或已被删除
3. **400错误**: 请求参数格式错误或session_ids为空
4. **网络错误**: 检查网络连接和Token有效性

## 📞 技术支持

如有问题，请联系后端开发团队。

---

**文档版本**: v1.0  
**更新日期**: 2025-07-19  
**作者**: 后端开发团队
