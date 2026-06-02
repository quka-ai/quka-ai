# Chrome Extension Session Chat 优化计划

## 问题背景

Chrome 插件当前主要覆盖网页总结和记忆保存能力。登录态恢复依赖 `/user/info` 返回后才更新界面，已保存 token 的用户打开插件时会短暂看到登录表单；同时插件虽然在总结流程中调用了 chat session API，但没有提供可继续对话的 session chat 界面。

## 改造目标

1. 已保存有效登录凭证时，插件启动默认进入已登录工作区，不展示登录表单。
2. 登录态恢复中展示轻量加载态，凭证失效后再展示登录表单。
3. 插件增加 session chat 功能，支持选择已有会话、新建会话、加载历史和发送消息。
4. 继续复用现有 REST API，不引入新的后端接口。

## 实施方案

1. 扩展本地设置结构，增加 `selectedChatSessionId`，用于记住当前会话。
2. 在 API 层增加 chat session 封装：
   - `GET /:spaceid/chat/list`
   - `POST /:spaceid/chat`
   - `GET /:spaceid/chat/:session/history/list`
   - `POST /:spaceid/chat/:session/message/id`
   - `POST /:spaceid/chat/:session/message`
3. 调整 popup 登录状态：
   - 启动读取到 token 后进入 `checking` 状态。
   - `checking` 和 `authed` 状态均不渲染登录表单。
   - 凭证校验失败后进入未登录状态并展示登录表单。
4. 新增“对话”tab：
   - 会话选择和新建按钮。
   - 消息历史展示。
   - 文本输入和发送按钮。
   - 发送后轮询历史直到 AI 回复完成。
5. 更新插件 README 的 API 映射。

## 关键考虑点

- 使用 polling 复用现有总结流程的稳定路径，不在本次变更引入 WebSocket/Centrifuge 连接复杂度。
- 已登录用户仍可通过退出按钮切换账号，登录表单不作为默认已登录界面展示。
- Space 切换时同步刷新 resource 和 chat session，避免会话跨空间误用。

## 状态追踪

- [x] 需求分析
- [x] 方案记录
- [x] API 层实现
- [x] Popup UI 实现
- [x] 构建验证

## 相关文件

- `chrome-extension/src/App.tsx`
- `chrome-extension/src/lib/api.ts`
- `chrome-extension/src/lib/types.ts`
- `chrome-extension/src/lib/storage.ts`
- `chrome-extension/src/components/ui/tabs.tsx`
- `chrome-extension/README.md`
