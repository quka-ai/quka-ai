# 首页 Fixed Pin 接口改造计划

## 问题描述和背景

用户需要在自己的首页固定一些高频访问内容，例如当前工作中正在管理的 GitHub milestone 链接。当前项目已有 chat session pin 和 memory pin，但它们面向会话或 agent runtime context，不适合作为用户首页的长期固定入口。

## 改造目标

- 新增服务端 fixed pin 数据表，用于保存用户在某个 space 下的首页固定内容。
- fixed pin 内容按当前项目的数据加密方式保存。
- 用户输入和返回内容使用 BlockNote blocks JSON，对应 `content_type = blocks_v2`。
- 数据按 `space_id + user_id` 隔离，每个用户在每个 space 下维护一份首页固定内容。
- 不接入 knowledge 摘要、分块、向量化和 memory 生命周期，避免污染知识库与 agent memory。

## 实施方案

1. 新增 `quka_fixed_pin` 表，字段包括 `id`、`space_id`、`user_id`、`content`、`content_type`、`created_at`、`updated_at`。
2. 在 `pkg/types` 中定义 `FixedPin` 类型和表名常量。
3. 在 store 层新增 `FixedPinStore`，提供 `Create`、`Get`、`Upsert`、`Delete`、`DeleteAll` 方法。
4. 在 logic 层新增 `FixedPinLogic`：
   - 写入前校验 `content_type` 必须为 `blocks_v2`。
   - 解析 BlockNote JSON，并移除文件块 URL host。
   - 使用 `core.EncryptData` 加密内容后写入。
   - 读取时解密内容，并给 BlockNote 文件块补 presigned URL。
5. 在 handler/router 层新增接口：
   - `GET /api/v1/:spaceid/fixed-pin`
   - `PUT /api/v1/:spaceid/fixed-pin`
   - `DELETE /api/v1/:spaceid/fixed-pin`

## 关键考虑点

- fixed pin 是用户私有首页数据，不应被同 space 其他用户读取。
- 接口不需要分页，因为设计为一个首页固定面板的整体覆盖保存。
- 当前目标是 BlockNote 格式，因此暂不开放 markdown/editorjs 多格式写入。
- 文件资源 URL 处理复用 knowledge 的 BlockNote 工具函数，保持前端展示行为一致。

## 时间线和状态追踪

- 2026-06-09: 创建改造计划并开始实现服务端接口。

## 需要确认的问题

- 后续如果需要多个首页区域，可以在表中增加 `scope` 或 `slot` 字段，而不是新建多张表。
- 前端是否希望 `PUT` 返回保存后的完整对象；本次实现会返回保存结果，方便前端直接刷新本地状态。

## 相关文件列表

- `pkg/types/tables.go`
- `pkg/types/fixed_pin.go`
- `app/store/store.go`
- `app/store/sqlstore/fixed_pin.go`
- `app/store/sqlstore/fixed_pin.sql`
- `app/logic/v1/fixed_pin.go`
- `cmd/service/handler/fixed_pin.go`
- `cmd/service/router.go`
