# 开发规范：国际化与测试布局

本规范适用于整个 `agent-remote-desktop` 项目，是开发及正式版验收要求。产品范围与阶段顺序以[产品方案](../../agent-remote/docs/agent-remote-desktop-design.zh-CN.md)为准；实现进度见[实施状态](implementation-status.md)。

## 官方 i18n

使用 mygo 官方 `github.com/egoist/mygo/i18n` 和生成式 Go catalog，不另建翻译框架。已核实当前固定版本提供这些能力，见[官方文档](https://github.com/egoist/mygo/blob/269e2f9aca82b742f67497e467a95f6adb54bf5d/docs/i18n.md)。

当前语言为英文 `en` 和简体中文 `zh-Hans`，英文作为源语言。`mygo.json` 已配置：

```json
"i18n": { "dir": "locales", "source": "en" }
```

- `locales/en.json` 保存完整源文案，`locales/zh-Hans.json` 保存简体中文翻译；使用官方 ARB 风格、ICU MessageFormat、语义键及类型化占位符。句子作为完整消息翻译，不能拼接自然语言片段。
- 用与 `go.mod` 一致的 mygo CLI 生成 `locales/catalog_gen.go`；提交 catalog 和生成文件，不手改生成代码。普通 Go 构建、测试不应要求预先安装全局 mygo CLI。
- 所有 desktop 自有用户可见文案必须调用 catalog，包括 UI、菜单、托盘、通知、状态、校验、错误摘要、恢复建议、无障碍标签及主题展示名称。新增 CLI 管理页面也必须覆盖。
- 业务层保留稳定错误码与结构化参数，在呈现层翻译。原始诊断可保留供排查，但不能替代本地化错误说明。不翻译 Agent 输出、用户输入、代码、路径、命令参数与协议标识。
- 默认跟随系统语言；通过 `mygo.App.SetLocale(tag)` 切换并由框架保存偏好，`SetLocale("")` 恢复跟随系统。不要维护第二套语言偏好来源。
- 原生 UI 按当前语言重绘；通过 `OnLocaleChanged` 刷新自定义菜单、托盘和窗口标题，不能重启 Agent 或终端连接。系统原生组件按平台实际能力生效，不承诺全部即时更新。
- 使用官方区域格式能力处理日期、时间、数字和复数；语言选项显示语言自称。生成 catalog 会引入相应 mygo 控件翻译，须验证控件与业务界面一致。

接入后的检查命令（在 desktop 仓库根运行）：

```sh
go run github.com/egoist/mygo/cmd/mygo generate
go run github.com/egoist/mygo/cmd/mygo i18n check
```

官方检查对漏译仅警告。项目门禁须另行拒绝已支持语言的缺失消息及不兼容占位符，并检查生成代码是否与 catalog 一致。相关测试全部放在 `tests/i18n/`；同时验证中英文切换、未知语言回退、区域格式，以及 `en-XA` 长文本布局。全局 locale 测试不能并行修改同一进程的语言状态。

## 测试与主程序分离

`tests/` 指 **本 Git 仓库根目录**，即 `agent-remote-desktop/tests/`。主程序位于 `main.go`、`internal/`、`cmd/` 等位置，任何自有测试文件均不得与其混放。

| 内容 | 规定位置 |
| --- | --- |
| 单元测试 | `tests/<模块>/*_test.go` |
| PTY、平台集成、UI、端到端测试 | `tests/` 下对应模块或专项目录 |
| 原生窗口测试程序 | `tests/native-probe/` |
| 工作台生命周期与 PTY 验证程序 | `tests/workbench-probe/` |
| 安装、升级与回滚原生文件系统探针 | `tests/installer-probe/` |
| 当前主机平台/运行时能力报告程序 | `tests/platform-probe/` |
| 测试专用辅助代码与 mock | `tests/testutil/` 或对应测试目录 |
| 夹具、样本、截图基准 | `tests/testdata/` 或对应测试目录的 `testdata/` |

测试使用根 `go.mod`，不新增嵌套模块；可合法导入同仓库 `internal/...` 包，通过公开行为和依赖注入验证。不能通过复制实现、改包名或为测试专门暴露私有状态规避目录分离。确需调整可测试性时，应形成合理的生产接口。

Windows/Unix 测试保留平台文件后缀及 build tags。临时运行数据写入临时目录，测试制品写入忽略的输出目录。构建和 CI 编排入口可留在 `scripts/`、`.github/`，具体测试逻辑必须位于 `tests/`。

既有测试及原生验证程序已完成迁移。后续新增模块遵守同一布局；不得通过删除测试或跳过断言满足规范。

专项入口：

```sh
go test ./tests/...
DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/terminalhost ./tests/session
DESKTOP_LOCAL_AGENT_TESTS=1 go test ./tests/terminalhost
go run ./tests/native-probe
go run -race ./tests/workbench-probe
go run ./tests/installer-probe
scripts/native-acceptance.sh
```

`go test ./...` 仍保留为全仓库检查。目录门禁扫描所有自有文件（包括开发时新增、尚未跟踪的文件），拒绝 `tests/` 之外的 `_test.go`；代码评审同时检查其他语言的测试、测试程序、辅助代码和夹具位置。

本地和 CI 均使用 `bash scripts/check.sh`：先拒绝 `tests/` 之外的 Go `_test.go`，再检查格式，运行官方 i18n 校验，重新生成并比较 catalog 代码，随后执行 vet、全部测试及 diff 空白检查。`tests/i18n/` 额外检查漏译、占位符、复数、回退和直接 UI 文案硬编码；语法检查无法替代对动态拼接文案的评审。

## 提交与验收

每项功能同时维护全部已支持语言的文案及必要测试。P0 建立 catalog 与检查入口，P1 完成既有文案和测试迁移；P2–P4 持续遵守，不留到发布前集中补做。满足文档规范不等于代码已完成；门禁尚未接入时必须在实施状态中明确列出。
