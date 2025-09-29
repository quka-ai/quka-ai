# Centrifuge 前端集成完整指南

## 概述

QukaAI已从Firetower迁移到Centrifuge实时通信框架。本文档提供完整的前端集成指南，包括基础用法、代码示例、API变更说明和迁移指导。

## 第一部分：快速开始

### 1. 安装和引入

```bash
# npm
npm install centrifuge

# yarn  
yarn add centrifuge

# pnpm
pnpm add centrifuge
```

```javascript
// ES6 模块
import { Centrifuge } from 'centrifuge';

// CommonJS
const { Centrifuge } = require('centrifuge');

// 浏览器直接引入
<script src="https://unpkg.com/centrifuge@4.1.4/dist/centrifuge.min.js"></script>
```

### 2. 基础连接代码

```javascript
// 获取现有的JWT Token（复用项目现有认证）
function getAuthToken() {
    return localStorage.getItem('access_token') || 
           sessionStorage.getItem('auth_token') ||
           getCurrentUserToken(); // 项目现有的获取Token方法
}

// 创建Centrifuge连接
const centrifuge = new Centrifuge('ws://localhost:8080/connection/websocket', {
    token: getAuthToken(),
    debug: process.env.NODE_ENV === 'development'
});

// 连接事件处理
centrifuge.on('connected', function(ctx) {
    console.log('已连接到服务器', ctx);
});

centrifuge.on('disconnected', function(ctx) {
    console.log('与服务器断开连接', ctx);
});

centrifuge.on('error', function(error) {
    console.error('连接错误:', error);
});

// 建立连接
centrifuge.connect();
```

### 3. 订阅聊天会话

```javascript
// 订阅会话频道
const sessionId = 'your-session-id';
const subscription = centrifuge.newSubscription(`session:${sessionId}`);

subscription.on('publication', function(ctx) {
    const message = ctx.data;
    handleMessage(message);
});

subscription.on('subscribed', function(ctx) {
    console.log('订阅会话成功', ctx);
});

subscription.subscribe();
```

## 第二部分：消息格式说明（基于实际服务端实现）

### 重要更新：服务端消息格式

根据最新的服务端实现，消息格式保持了与原Firetower的兼容性：

```javascript
// AI流式消息格式
{
    "subject": "on_message",        // 消息主题
    "version": "v1",                // 版本号
    "type": "2",                    // WsEventType数值(assistant_continue)
    "data": {                       // 消息数据
        "message_id": "msg_123",
        "session_id": "session_456",
        "message": "AI回复内容",
        "start_at": 0,
        "msg_type": 1,
        "complete": 0               // 0=进行中, 1=完成
    }
}

// AI消息初始化
{
    "subject": "on_message_init",
    "version": "v1", 
    "type": "1",                    // WsEventType数值(assistant_init)
    "data": {
        "message_id": "msg_123",
        "session_id": "session_456",
        // 初始化数据
    }
}

// 工具调用消息
{
    "subject": "on_message",
    "version": "v1",
    "type": "6",                    // WsEventType数值(tool_continue)
    "data": {
        "message_id": "msg_123",
        "session_id": "session_456",
        "tool_tips": "工具调用信息",
        "msg_type": 2,
        "complete": 0
    }
}

// 会话重命名
{
    "subject": "session_rename",
    "version": "v1", 
    "type": "400",                  // WsEventType数值(others)
    "data": {
        "session_id": "session_123",
        "name": "新会话名称"
    }
}

// 知识库状态更新
{
    "subject": "stage_changed",
    "version": "v1",
    "type": "400",
    "data": {
        "knowledge_id": "knowledge_123",
        "stage": "DONE"
    }
}
```

### WsEventType 类型映射

```javascript
// 重要：type字段现在是数值字符串
const WS_EVENT_TYPES = {
    UNKNOWN: "0",
    ASSISTANT_INIT: "1",      // AI消息初始化
    ASSISTANT_CONTINUE: "2",  // AI消息继续
    ASSISTANT_DONE: "3",      // AI消息完成
    ASSISTANT_FAILED: "4",    // AI消息失败
    TOOL_INIT: "5",          // 工具调用初始化
    TOOL_CONTINUE: "6",      // 工具调用继续
    TOOL_DONE: "7",          // 工具调用完成
    TOOL_FAILED: "8",        // 工具调用失败
    MESSAGE_PUBLISH: "100",   // 用户消息发布
    SYSTEM_ONSUBSCRIBE: "300", // 订阅成功
    SYSTEM_UNSUBSCRIBE: "301", // 取消订阅
    OTHERS: "400"            // 其他事件
};
```

## 第三部分：完整的消息处理示例

### React Hook 实现

```jsx
import React, { useState, useEffect, useRef } from 'react';
import { Centrifuge } from 'centrifuge';

// 自定义Hook
function useCentrifuge(url, token) {
    const [centrifuge, setCentrifuge] = useState(null);
    const [connected, setConnected] = useState(false);

    useEffect(() => {
        if (!token) return;

        const client = new Centrifuge(url, {
            token: token,
            debug: process.env.NODE_ENV === 'development'
        });

        client.on('connected', () => {
            setConnected(true);
        });

        client.on('disconnected', () => {
            setConnected(false);
        });

        client.connect();
        setCentrifuge(client);

        return () => {
            client.disconnect();
        };
    }, [url, token]);

    return { centrifuge, connected };
}

// 聊天组件
function ChatRoom({ sessionId, token }) {
    const [messages, setMessages] = useState([]);
    const [newMessage, setNewMessage] = useState('');
    const [onlineUsers, setOnlineUsers] = useState(0);
    const { centrifuge, connected } = useCentrifuge(
        'ws://localhost:8080/connection/websocket', 
        token
    );
    const subscriptionRef = useRef(null);

    useEffect(() => {
        if (!centrifuge || !connected || !sessionId) return;

        const channelName = `session:${sessionId}`;
        const subscription = centrifuge.newSubscription(channelName);

        // 消息处理 - 基于实际服务端格式
        subscription.on('publication', (ctx) => {
            const data = ctx.data;
            
            switch (data.subject) {
                case 'on_message_init':
                    if (data.type === WS_EVENT_TYPES.ASSISTANT_INIT) {
                        handleAIMessageInit(data.data);
                    } else if (data.type === WS_EVENT_TYPES.TOOL_INIT) {
                        handleToolInit(data.data);
                    }
                    break;
                    
                case 'on_message':
                    if (data.type === WS_EVENT_TYPES.ASSISTANT_CONTINUE) {
                        handleAIStream(data.data);
                    } else if (data.type === WS_EVENT_TYPES.TOOL_CONTINUE) {
                        handleToolContinue(data.data);
                    }
                    break;
                    
                case 'session_rename':
                    handleSessionRename(data.data);
                    break;
                    
                case 'stage_changed':
                    handleKnowledgeStageChange(data.data);
                    break;
            }
        });

        // 订阅成功后获取在线用户数
        subscription.on('subscribed', async () => {
            try {
                const stats = await centrifuge.presenceStats(channelName);
                setOnlineUsers(stats.numUsers);
            } catch (error) {
                console.error('获取在线用户数失败:', error);
            }
        });

        subscription.subscribe();
        subscriptionRef.current = subscription;

        return () => {
            subscription.unsubscribe();
        };
    }, [centrifuge, connected, sessionId]);

    const handleAIMessageInit = (data) => {
        const { message_id, session_id } = data;
        
        setMessages(prev => [...prev, {
            id: message_id,
            content: '',
            role: 'assistant',
            timestamp: Date.now(),
            complete: 0,
            typing: true
        }]);
    };

    const handleAIStream = (streamData) => {
        const { message_id, message, complete } = streamData;
        
        setMessages(prev => {
            const existingIndex = prev.findIndex(msg => msg.id === message_id);
            
            if (existingIndex >= 0) {
                // 更新现有消息
                const updated = [...prev];
                updated[existingIndex] = {
                    ...updated[existingIndex],
                    content: message,
                    complete: complete,
                    typing: complete === 0
                };
                return updated;
            } else {
                // 新消息
                return [...prev, {
                    id: message_id,
                    content: message,
                    role: 'assistant',
                    timestamp: Date.now(),
                    complete: complete,
                    typing: complete === 0
                }];
            }
        });
    };

    const handleToolInit = (data) => {
        console.log('工具调用初始化:', data);
    };

    const handleToolContinue = (data) => {
        const { message_id, tool_tips, complete } = data;
        console.log('工具调用:', { message_id, tool_tips, complete });
    };

    const handleSessionRename = (data) => {
        const { session_id, name } = data;
        console.log('会话重命名:', { session_id, name });
        // 更新会话标题UI
    };

    const handleKnowledgeStageChange = (data) => {
        const { knowledge_id, stage } = data;
        console.log('知识库状态变更:', { knowledge_id, stage });
        // 更新知识库状态UI
    };

    const sendMessage = async () => {
        if (!newMessage.trim()) return;

        try {
            const response = await fetch(`/api/sessions/${sessionId}/messages`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    message: newMessage,
                    message_type: 'text'
                })
            });

            if (response.ok) {
                setNewMessage('');
            } else {
                console.error('发送消息失败');
            }
        } catch (error) {
            console.error('发送消息错误:', error);
        }
    };

    return (
        <div className="chat-room">
            <div className="chat-header">
                <h3>会话: {sessionId}</h3>
                <div className="status">
                    {connected ? (
                        <span className="connected">已连接 ({onlineUsers} 人在线)</span>
                    ) : (
                        <span className="disconnected">连接中...</span>
                    )}
                </div>
            </div>

            <div className="messages">
                {messages.map(message => (
                    <div key={message.id} className={`message ${message.role}`}>
                        <div className="content">
                            {message.content}
                            {message.typing && (
                                <span className="typing-indicator">...</span>
                            )}
                        </div>
                        <div className="timestamp">
                            {new Date(message.timestamp).toLocaleTimeString()}
                        </div>
                    </div>
                ))}
            </div>

            <div className="message-input">
                <input
                    type="text"
                    value={newMessage}
                    onChange={(e) => setNewMessage(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && sendMessage()}
                    placeholder="输入消息..."
                    disabled={!connected}
                />
                <button onClick={sendMessage} disabled={!connected || !newMessage.trim()}>
                    发送
                </button>
            </div>
        </div>
    );
}

export default ChatRoom;
```

### Vue 3 组合式API 实现

```vue
<template>
  <div class="chat-container">
    <div class="connection-status" :class="{ connected, disconnected: !connected }">
      {{ connected ? '已连接' : '连接中...' }}
      <span v-if="connected">({{ onlineUsers }} 人在线)</span>
    </div>

    <div class="messages-container" ref="messagesContainer">
      <div
        v-for="message in messages"
        :key="message.id"
        :class="['message', message.role]"
      >
        <div class="message-content">
          {{ message.content }}
          <span v-if="message.typing" class="typing">...</span>
        </div>
        <div class="message-time">
          {{ formatTime(message.timestamp) }}
        </div>
      </div>
    </div>

    <div class="input-area">
      <input
        v-model="newMessage"
        @keyup.enter="sendMessage"
        :disabled="!connected"
        placeholder="输入消息..."
        class="message-input"
      />
      <button @click="sendMessage" :disabled="!connected || !newMessage.trim()">
        发送
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue';
import { Centrifuge } from 'centrifuge';

const props = defineProps({
  sessionId: String,
  token: String
});

// WsEventType 映射
const WS_EVENT_TYPES = {
    ASSISTANT_INIT: "1",
    ASSISTANT_CONTINUE: "2",
    TOOL_INIT: "5",
    TOOL_CONTINUE: "6",
    OTHERS: "400"
};

// 响应式数据
const connected = ref(false);
const messages = ref([]);
const newMessage = ref('');
const onlineUsers = ref(0);
const messagesContainer = ref(null);

// Centrifuge相关
let centrifuge = null;
let subscription = null;

// 连接Centrifuge
const connectCentrifuge = () => {
  if (!props.token) return;

  centrifuge = new Centrifuge('ws://localhost:8080/connection/websocket', {
    token: props.token,
    debug: import.meta.env.DEV
  });

  centrifuge.on('connected', (ctx) => {
    connected.value = true;
    console.log('Centrifuge连接成功', ctx);
  });

  centrifuge.on('disconnected', (ctx) => {
    connected.value = false;
    console.log('Centrifuge断开连接', ctx);
  });

  centrifuge.on('error', (error) => {
    console.error('Centrifuge错误:', error);
  });

  centrifuge.connect();
};

// 订阅频道
const subscribeToChannel = () => {
  if (!centrifuge || !props.sessionId) return;

  const channelName = `session:${props.sessionId}`;
  subscription = centrifuge.newSubscription(channelName);

  subscription.on('publication', (ctx) => {
    handleMessage(ctx.data);
  });

  subscription.on('subscribed', async (ctx) => {
    console.log('订阅成功', ctx);
    
    try {
      const stats = await centrifuge.presenceStats(channelName);
      onlineUsers.value = stats.numUsers;
    } catch (error) {
      console.error('获取在线用户数失败:', error);
    }
  });

  subscription.subscribe();
};

// 处理接收到的消息 - 基于实际服务端格式
const handleMessage = (data) => {
  switch (data.subject) {
    case 'on_message_init':
      if (data.type === WS_EVENT_TYPES.ASSISTANT_INIT) {
        handleAIMessageInit(data.data);
      } else if (data.type === WS_EVENT_TYPES.TOOL_INIT) {
        handleToolInit(data.data);
      }
      break;
      
    case 'on_message':
      if (data.type === WS_EVENT_TYPES.ASSISTANT_CONTINUE) {
        handleAIStream(data.data);
      } else if (data.type === WS_EVENT_TYPES.TOOL_CONTINUE) {
        handleToolContinue(data.data);
      }
      break;
      
    case 'session_rename':
      handleSessionRename(data.data);
      break;
      
    case 'stage_changed':
      handleKnowledgeStageChange(data.data);
      break;
      
    default:
      console.log('未知消息类型:', data);
  }
};

// AI消息初始化
const handleAIMessageInit = (data) => {
  const { message_id } = data;
  
  messages.value.push({
    id: message_id,
    content: '',
    role: 'assistant',
    timestamp: Date.now(),
    complete: 0,
    typing: true
  });
  scrollToBottom();
};

// 处理AI流式消息
const handleAIStream = (streamData) => {
  const { message_id, message, complete } = streamData;
  
  const existingIndex = messages.value.findIndex(msg => msg.id === message_id);
  
  if (existingIndex >= 0) {
    messages.value[existingIndex] = {
      ...messages.value[existingIndex],
      content: message,
      complete: complete,
      typing: complete === 0
    };
  } else {
    messages.value.push({
      id: message_id,
      content: message,
      role: 'assistant',
      timestamp: Date.now(),
      complete: complete,
      typing: complete === 0
    });
  }
  
  if (complete === 1) {
    scrollToBottom();
  }
};

// 处理工具调用
const handleToolInit = (data) => {
  console.log('工具调用初始化:', data);
};

const handleToolContinue = (data) => {
  const { message_id, tool_tips, complete } = data;
  console.log('工具调用:', { message_id, tool_tips, complete });
};

// 处理会话重命名
const handleSessionRename = (data) => {
  const { session_id, name } = data;
  console.log('会话重命名:', { session_id, name });
  // 更新会话标题
};

// 处理知识库状态变更
const handleKnowledgeStageChange = (data) => {
  const { knowledge_id, stage } = data;
  console.log('知识库状态变更:', { knowledge_id, stage });
  // 更新知识库状态UI
};

// 发送消息
const sendMessage = async () => {
  if (!newMessage.value.trim()) return;

  try {
    const response = await fetch(`/api/sessions/${props.sessionId}/messages`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${props.token}`
      },
      body: JSON.stringify({
        message: newMessage.value,
        message_type: 'text'
      })
    });

    if (response.ok) {
      newMessage.value = '';
    } else {
      console.error('发送消息失败');
    }
  } catch (error) {
    console.error('发送消息错误:', error);
  }
};

// 滚动到底部
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
    }
  });
};

// 格式化时间
const formatTime = (timestamp) => {
  return new Date(timestamp).toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit'
  });
};

// 监听sessionId变化
watch(() => props.sessionId, (newSessionId, oldSessionId) => {
  if (oldSessionId && subscription) {
    subscription.unsubscribe();
  }
  
  if (newSessionId) {
    messages.value = [];
    subscribeToChannel();
  }
});

// 生命周期
onMounted(() => {
  connectCentrifuge();
});

onUnmounted(() => {
  if (subscription) {
    subscription.unsubscribe();
  }
  if (centrifuge) {
    centrifuge.disconnect();
  }
});

// 监听连接状态
watch(connected, (isConnected) => {
  if (isConnected && props.sessionId) {
    subscribeToChannel();
  }
});
</script>

<style scoped>
.chat-container {
  display: flex;
  flex-direction: column;
  height: 500px;
  border: 1px solid #ddd;
  border-radius: 8px;
  overflow: hidden;
}

.connection-status {
  padding: 8px 16px;
  background: #f5f5f5;
  border-bottom: 1px solid #ddd;
  font-size: 14px;
}

.connection-status.connected {
  background: #e8f5e8;
  color: #2d5a2d;
}

.connection-status.disconnected {
  background: #ffe8e8;
  color: #a52a2a;
}

.messages-container {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.message {
  margin-bottom: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  max-width: 80%;
}

.message.user {
  background: #007bff;
  color: white;
  margin-left: auto;
}

.message.assistant {
  background: #f1f1f1;
  color: #333;
}

.message-content {
  margin-bottom: 4px;
}

.typing {
  color: #999;
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.message-time {
  font-size: 12px;
  opacity: 0.7;
}

.input-area {
  display: flex;
  padding: 16px;
  border-top: 1px solid #ddd;
  background: white;
}

.message-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-right: 8px;
}

.message-input:disabled {
  background: #f5f5f5;
  cursor: not-allowed;
}

button {
  padding: 8px 16px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

button:disabled {
  background: #ccc;
  cursor: not-allowed;
}

button:hover:not(:disabled) {
  background: #0056b3;
}
</style>
```

## 第四部分：API变更对比

### 连接方式对比

**旧方式（原生WebSocket）:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

**新方式（Centrifuge）:**
```javascript
const centrifuge = new Centrifuge('ws://localhost:8080/connection/websocket', {
    token: getAuthToken()
});
```

### 消息订阅对比

**旧方式:**
```javascript
ws.send(JSON.stringify({
    type: 'subscribe',
    topic: 'session:123'
}));
```

**新方式:**
```javascript
const subscription = centrifuge.newSubscription('session:123');
subscription.subscribe();
```

### 消息处理对比

**旧方式:**
```javascript
ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    // 手动处理消息格式
};
```

**新方式:**
```javascript
subscription.on('publication', (ctx) => {
    const message = ctx.data;
    // 自动解析，直接处理业务逻辑
});
```

## 第五部分：错误处理和重连

### 自动重连配置

```javascript
const centrifuge = new Centrifuge('ws://localhost:8080/connection/websocket', {
    token: getAuthToken(),
    // 重连配置
    minReconnectDelay: 1000,      // 最小重连间隔：1秒
    maxReconnectDelay: 20000,     // 最大重连间隔：20秒
    reconnectJitter: 0.1,         // 重连抖动：10%
    maxServerPingDelay: 10000,    // 服务器ping超时：10秒
});

// 监听重连事件
centrifuge.on('connecting', (ctx) => {
    console.log('正在重连...', ctx);
    showReconnectingIndicator();
});

centrifuge.on('connected', (ctx) => {
    console.log('重连成功', ctx);
    hideReconnectingIndicator();
});
```

### 错误处理

```javascript
// 全局错误处理
centrifuge.on('error', (error) => {
    console.error('Centrifuge错误:', error);
    
    switch (error.type) {
        case 'transport':
            showError('网络连接错误，请检查网络');
            break;
        case 'unauthorized':
            showError('认证失败，请重新登录');
            redirectToLogin();
            break;
        case 'permission_denied':
            showError('权限不足');
            break;
        default:
            showError('连接出现问题，正在尝试重连...');
    }
});

// Token刷新处理
centrifuge.on('error', async (error) => {
    if (error.code === 'token_expired') {
        try {
            const newToken = await refreshToken();
            centrifuge.setToken(newToken);
        } catch (refreshError) {
            redirectToLogin();
        }
    }
});
```

## 第六部分：性能优化

### 订阅管理器

```javascript
class SubscriptionManager {
    constructor(centrifuge) {
        this.centrifuge = centrifuge;
        this.subscriptions = new Map();
    }

    subscribe(channel, handlers) {
        if (this.subscriptions.has(channel)) {
            const sub = this.subscriptions.get(channel);
            this.addHandlers(sub, handlers);
            return sub;
        }

        const subscription = this.centrifuge.newSubscription(channel);
        this.addHandlers(subscription, handlers);
        subscription.subscribe();
        
        this.subscriptions.set(channel, subscription);
        return subscription;
    }

    unsubscribe(channel) {
        const subscription = this.subscriptions.get(channel);
        if (subscription) {
            subscription.unsubscribe();
            subscription.removeAllListeners();
            this.subscriptions.delete(channel);
        }
    }

    addHandlers(subscription, handlers) {
        Object.entries(handlers).forEach(([event, handler]) => {
            subscription.on(event, handler);
        });
    }

    cleanup() {
        for (const [channel, subscription] of this.subscriptions) {
            subscription.unsubscribe();
            subscription.removeAllListeners();
        }
        this.subscriptions.clear();
    }
}
```

### 消息批处理

```javascript
class MessageBatcher {
    constructor(processFunc, delay = 100) {
        this.processFunc = processFunc;
        this.delay = delay;
        this.batch = [];
        this.timer = null;
    }

    add(message) {
        this.batch.push(message);
        
        if (this.timer) {
            clearTimeout(this.timer);
        }

        this.timer = setTimeout(() => {
            this.flush();
        }, this.delay);
    }

    flush() {
        if (this.batch.length > 0) {
            this.processFunc(this.batch);
            this.batch = [];
        }
        this.timer = null;
    }
}

// 使用示例
const messageBatcher = new MessageBatcher((messages) => {
    updateMessagesInBatch(messages);
}, 50);

subscription.on('publication', (ctx) => {
    messageBatcher.add(ctx.data);
});
```

## 第七部分：TypeScript 支持

```typescript
// types.ts
export interface CentrifugeConfig {
  url: string;
  token: string;
  debug?: boolean;
}

export interface Message {
  id: string;
  content: string;
  role: 'user' | 'assistant' | 'system';
  timestamp: number;
  complete?: number;
  typing?: boolean;
}

export interface StreamData {
  message_id: string;
  session_id: string;
  message: string;
  complete: number;
  start_at?: number;
  msg_type?: number;
  tool_tips?: string;
}

export interface CentrifugeMessage {
  subject: string;
  version: string;
  type: string;
  data: any;
}

// React Hook with TypeScript
import { useState, useEffect, useRef } from 'react';
import { Centrifuge, Subscription } from 'centrifuge';

export function useCentrifuge(config: CentrifugeConfig) {
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const centrifugeRef = useRef<Centrifuge | null>(null);

  useEffect(() => {
    const centrifuge = new Centrifuge(config.url, {
      token: config.token,
      debug: config.debug
    });

    centrifuge.on('connected', () => {
      setConnected(true);
      setError(null);
    });

    centrifuge.on('disconnected', () => {
      setConnected(false);
    });

    centrifuge.on('error', (error) => {
      setError(error.message);
    });

    centrifuge.connect();
    centrifugeRef.current = centrifuge;

    return () => {
      centrifuge.disconnect();
    };
  }, [config.url, config.token]);

  const subscribe = (
    channel: string,
    onMessage: (data: CentrifugeMessage) => void
  ): Subscription | null => {
    if (!centrifugeRef.current) return null;

    const subscription = centrifugeRef.current.newSubscription(channel);
    subscription.on('publication', (ctx) => {
      onMessage(ctx.data);
    });
    subscription.subscribe();

    return subscription;
  };

  return {
    connected,
    error,
    subscribe,
    centrifuge: centrifugeRef.current
  };
}
```

## 第八部分：测试

### Jest 单元测试

```javascript
import { Centrifuge } from 'centrifuge';
import { CentrifugeManager } from './centrifuge-manager';

// Mock Centrifuge
jest.mock('centrifuge');

describe('CentrifugeManager', () => {
  let manager;
  let mockCentrifuge;

  beforeEach(() => {
    mockCentrifuge = {
      on: jest.fn(),
      connect: jest.fn(),
      disconnect: jest.fn(),
      newSubscription: jest.fn(),
      presenceStats: jest.fn()
    };

    Centrifuge.mockImplementation(() => mockCentrifuge);

    manager = new CentrifugeManager({
      url: 'ws://test.com/centrifuge',
      token: 'test-token'
    });
  });

  describe('消息处理', () => {
    it('应该正确处理AI流式消息', () => {
      const mockSubscription = {
        on: jest.fn(),
        subscribe: jest.fn()
      };

      mockCentrifuge.newSubscription.mockReturnValue(mockSubscription);

      const subscription = manager.subscribe('session:123', {
        onMessage: jest.fn()
      });

      // 模拟接收AI消息
      const messageHandler = mockSubscription.on.mock.calls
        .find(call => call[0] === 'publication')[1];

      messageHandler({
        data: {
          subject: 'on_message',
          type: '2',
          data: {
            message_id: 'msg_123',
            message: 'AI回复',
            complete: 0
          }
        }
      });

      expect(subscription).toBe(mockSubscription);
    });
  });
});

// Cypress E2E 测试
describe('Centrifuge 集成测试', () => {
  beforeEach(() => {
    cy.visit('/chat');
    cy.intercept('GET', '/api/auth/token', { 
      fixture: 'auth-token.json' 
    });
  });

  it('应该成功连接并接收AI消息', () => {
    // 检查连接状态
    cy.get('[data-testid="connection-status"]')
      .should('contain', '已连接');

    // 发送消息
    cy.get('[data-testid="message-input"]')
      .type('Hello AI{enter}');

    // 验证用户消息显示
    cy.get('[data-testid="messages"]')
      .should('contain', 'Hello AI');

    // 验证AI回复（流式）
    cy.get('[data-testid="messages"]')
      .should('contain', '...', { timeout: 1000 }) // 打字指示器
      .should('not.contain', '...', { timeout: 10000 }); // AI完成回复
  });

  it('应该处理断线重连', () => {
    // 模拟网络断开
    cy.window().then((win) => {
      win.navigator.onLine = false;
      win.dispatchEvent(new Event('offline'));
    });

    cy.get('[data-testid="connection-status"]')
      .should('contain', '连接中');

    // 恢复网络
    cy.window().then((win) => {
      win.navigator.onLine = true;
      win.dispatchEvent(new Event('online'));
    });

    cy.get('[data-testid="connection-status"]')
      .should('contain', '已连接', { timeout: 5000 });
  });
});
```

## 第九部分：迁移检查清单

### 前端迁移步骤

- [ ] **安装依赖**: `npm install centrifuge`
- [ ] **更新连接代码**: 使用Centrifuge替代WebSocket
- [ ] **认证集成**: 复用现有JWT Token，无需额外API
- [ ] **消息格式适配**: 根据实际服务端格式处理消息
- [ ] **订阅管理**: 实现频道订阅和取消订阅逻辑
- [ ] **错误处理**: 添加重连和错误处理机制
- [ ] **UI状态管理**: 实现连接状态、在线用户数显示
- [ ] **性能优化**: 实现消息批处理、订阅复用
- [ ] **类型安全**: 添加TypeScript类型定义（可选）
- [ ] **测试验证**: 单元测试和E2E测试
- [ ] **性能测试**: 验证消息延迟和并发能力
- [ ] **生产部署**: 完成测试后部署到生产环境

### 关键配置项

```javascript
// 生产环境推荐配置
const centrifuge = new Centrifuge('wss://yourdomain.com/connection/websocket', {
    token: getAuthToken(),
    debug: false,
    minReconnectDelay: 1000,
    maxReconnectDelay: 20000,
    reconnectJitter: 0.1,
    maxServerPingDelay: 10000
});
```

## 第十部分：常见问题

### Q1: 如何处理Token过期？
A: 复用现有的Token刷新逻辑，使用`centrifuge.setToken(newToken)`更新Token。

### Q2: 消息格式和之前不一样怎么办？
A: 服务端保持了兼容性，消息格式基本不变，只是`type`字段现在是数值字符串。

### Q3: 如何获取在线用户数？
A: 使用`centrifuge.presenceStats(channel)`获取频道统计信息。

### Q4: 支持多个会话同时订阅吗？
A: 支持，可以为每个会话创建独立的订阅。

### Q5: 如何处理网络切换？
A: Centrifuge会自动处理重连，也可以监听浏览器的`online/offline`事件。

---

**注意事项：**
1. 请在开发环境充分测试后再部署到生产环境
2. 建议保留旧的WebSocket代码作为备份，渐进式迁移
3. 密切关注控制台日志，及时发现和解决问题
4. 如有疑问，请联系后端开发团队

**支持渠道：**
- 查看[Centrifuge官方文档](https://centrifugal.dev/docs)
- 项目内部技术讨论群
- GitHub Issues