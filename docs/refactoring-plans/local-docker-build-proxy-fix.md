# 本地 Docker 构建代理修复计划

## 问题描述和背景

执行 `./scripts/build-local.sh v0.2.20` 时，`Dockerfile-local` 中的 `go mod download` 报错：`proxyconnect tcp: dial tcp 127.0.0.1:7777: connect: connection refused`。

`build-local.sh` 调用 `build-multiarch.sh`。后者把宿主机的 `HTTP_PROXY` 和 `HTTPS_PROXY` 原样作为 Docker 构建参数传入。目前宿主机的两个变量都指向 `http://127.0.0.1:7777`。构建容器中的 `127.0.0.1` 指向容器自身，无法访问宿主机代理。宿主机代理端口可用，且宿主机直连 `goproxy.cn` 返回 HTTP 200。

## 改造目标和步骤

1. 避免将仅供宿主机使用的回环地址代理直接传入构建容器。
2. 保留显式配置、且容器可访问的代理地址的传递能力。
3. 更新构建说明，说明代理变量的影响和本地构建用法。
4. 用脚本检查与最小 Docker 构建验证代理行为；不执行镜像推送。

## 详细实施方案

- 在 `scripts/build-multiarch.sh` 组装构建参数时识别 `localhost`、`127.0.0.1`、`::1` 形式的代理地址。默认跳过这类代理并给出可读提示，避免构建容器误用回环地址。
- 对其他代理地址保留现有传递行为。构建参数改用 Bash 数组，防止代理 URL 中的特殊字符触发单词拆分。
- 在 `scripts/README.md` 中修正 `build-local.sh` 的参数说明，并提供需要代理时指定容器可访问代理地址的示例。
- 临时解决命令：`env -u HTTP_PROXY -u HTTPS_PROXY ./scripts/build-local.sh v0.2.20`。如果构建器自身另有代理配置，还需检查构建器环境。

## 关键考虑点

- 本次错误发生在依赖下载阶段，与 `cloud.google.com/go@v0.115.0` 模块版本无关。
- 跳过回环代理要求构建容器能直接访问模块源；若网络必须走代理，需提供构建容器可访问的代理地址。
- 当前 `build-local.sh` 会使用 `--push`，完整运行会推送镜像；验证阶段只做本地、最小范围的检查。
- 工作区的 `go.sum` 已有未提交改动，本计划不修改它。

## 时间线和状态追踪

| 阶段 | 状态 |
| --- | --- |
| 排查脚本与代理来源 | 已完成，2026-10-09 |
| 计划文档 | 已完成，2026-10-09 |
| 用户 review | 待确认 |
| 脚本和文档修改 | 待开始 |
| 验证 | 待开始 |

## 需要确认的问题

- 是否同意默认忽略回环地址代理，并在需要代理时手动设置容器可访问的代理地址？

## 相关文件列表

- `scripts/build-local.sh`
- `scripts/build-multiarch.sh`
- `scripts/README.md`
- `Dockerfile-local`
