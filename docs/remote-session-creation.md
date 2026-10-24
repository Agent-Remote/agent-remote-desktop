# 远程会话创建 GUI 合同

本文描述当前 desktop、Rust CLI 和 Server 已实现的远程会话创建边界。历史规划不作为实现依据；当前发布状态和跨端证据以 `implementation-status.md` 为准。

## 所有权

desktop 负责选择、审阅、确认和展示。Rust CLI 是 caller key、journal、凭据和远程请求的唯一持久所有者。Server 保存按用户隔离的创建请求回执，Node 只执行已受理的任务。desktop 只保存项目、pane、公开会话上下文和不含秘密的恢复指针。

创建请求不会把 raw argv、环境、凭据或终端输出写入 desktop。CLI journal 只保存用于恢复的摘要、digest、上下文和公开 ID。

## 当前流程

1. 用户选择项目、工具、账户、工作区及可选的 interrupted replacement。
2. desktop 通过 CLI 检查准确的 home、Server、device、workspace、sync target、Node 和 runtime backend。
3. 用户确认完整上下文后，CLI 生成规范非零 UUID caller key，冻结意图摘要并提交一次 `sessions.create`。
4. Server 在 `(user_id, request_id)` 下幂等受理；同键同意图返回原回执，同键不同意图返回冲突。
5. 响应丢失时，desktop 只能调用 `sessions.create.request` 查询原 key。它不会按 latest session、默认账户或新建命令猜测结果。
6. 取得准确 session UUID 后，连接仍由独立的 exact attach 确认完成；创建成功不等于 attach 成功。

## 状态和安全边界

`prepared`、`admission_pending`、`accepted`、`running`、`failed`、`unknown` 和 `deleted` 是请求、任务或资源的观察状态，不能合并成“全部完成”。同步 flush、Skill 保存/发布、进程生命周期和终端连接分别展示。

Server 重新校验用户、账户、workspace、sync target、Node、backend 和 replacement 上下文。目标漂移、能力缺失、认证失败、旧 CLI/Server 不支持 caller key 或观察不确定时均 fail-closed。未知结果保留原 key，必须显式查询或重新审阅。

## 当前验收范围

Go/UI、CLI、Server、Node 的合同测试和本地恢复测试已通过。正式发布仍需一次性真实 Server/Node/SSH/tmux/fclaude 创建、丢响应恢复、exact attach、停止/删除及附件清理回执，并需 Windows/Linux 原生 desktop 证据。