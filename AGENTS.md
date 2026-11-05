# BeeNT Go SDK 开发约定

## 从这里开始

开发插件前依次阅读 [README](README.md)、[AI 开发指南](docs/AI开发指南.md) 和相关 [API 参考](docs/API参考.md)。修改设置窗口时还需阅读 [设置窗口开发规范](docs/设置窗口开发规范.md)。文档导航见 [docs/README.md](docs/README.md)。以当前源码为准，历史计划和旧 Bee 文档只作背景。

## 文件职责与修改范围

- `plugin_main.go`：插件信息、生命周期和业务回调；新业务辅助文件放在根目录、同属 `package main`。
- `settings.go` / `settings_nonwindows.go`：Windows 设置窗口与非 Windows 编译桩。
- `beent_sdk.go` / `api_methods.go` / `types.go`：SDK 协议、97 个 API、结构与 31 个事件。
- `ipc.go` / `worker_runtime.go`：维护中的 Worker 源码，直接参与编译，不能忽略或当成生成物覆盖。
- `other/`：原生 C 壳、无 CRT 支持、导出表、构建工具和内部回归测试；详见 [other/README.md](other/README.md)。
- `other/tests/fixtures/`：内部测试业务与数据，不作为公开构建模式。
- 本地测试可放在对应代码旁；`*_test.go` 由 Git 忽略，不提交。`build/`、`temp/` 和运行数据同样不提交。

普通业务集中修改业务和设置文件。SDK、IPC、C 壳或构建工具需要改动时，应说明接口依据，并执行相应回归；不要为了排版或目录对齐改写已经验证的 ABI 与生命周期。

## 构建与系统兼容

目标固定为 Windows/386，纯 C stdcall 插件 DLL + 独立 `CGO_ENABLED=0` Go Worker。禁止把 Go Runtime、cgo、`buildmode=c-shared` 或 `buildmode=c-archive` 链接进 DLL。

所有构建统一精确使用 **Go 1.20.14**，兼容基线是 Windows Server 2012 R2，源码和依赖保持 **Go 1.20 兼容**。不要增加工具链选择分支、系统专用构建入口或使用 PATH 上的新版 Go 生成交付产物，不能仅凭 `go.mod` 声称已锁定编译器。

```bat
build.bat
build.bat MyPlugin.dll
```

根目录只保留这一构建入口，由它调用 `other/build.ps1`。脚本默认下载并缓存官方 Go 1.20.14，内部可显式指定同版本编译器。构建内部检查最终 DLL 内嵌 Worker 的实际版本、PE 与哈希。`build/` 只保留最终 DLL；不要增加工具链选择、公开验收模式或额外交付文件。业务完成后导入真实 BeeNT 验证。

C 壳用 `other/beent_crt.h`、`other/beent_stack_probe.S` 无 CRT 构建，最终导入表仅允许 KERNEL32.dll。不要只添加 `-static`，也不要移除逐页栈探测。不要修改全局 Go 环境或系统 PATH 来临时解决构建问题。

## BeeNT 协议边界

- 协议来源是提供的 `BeeNT框架参考`，尤其 `易语言_插件API接收处理.txt`；不复用旧 Bee 操作码、频道回调或机器人 JSON 初始化。
- 唯一导出 `botPluginLoad(core,index)` 返回 GBK JSON：`name/author/ver/desc` 及 `enable/disable/uninstall/setting/private/group/event` 七个数值回调指针。
- 生命周期原生回调无参数；好友消息 3 个、群消息 4 个、事件 8 个文本参数。Go 回调签名以 `plugin_main.go` 为准。
- `NewBeeNTAPI()` 不接收机器人 JSON。QQ、群号、消息 ID 保留为字符串；时间戳用 `int64`。
- 框架命令分隔符为 `#|&BEExNT&|#`，列表字段分隔符为 `/c.BeExNTs.\`；参数不得包含 NUL 或保留命令分隔符。
- 宿主边界 GBK/CP936，Go/IPC UTF-8；无法由 GBK 表示的字符使用 UTF-16 `\uXXXX` 及代理对。
- `Call` 保留原文，JSON 先 `json.Unmarshal`，普通文本需要时再 `DecodeFrameworkText`。框架业务错误可能是返回文本，不等于 IPC 错误。
- 框架 API 地址和返回指针只在宿主进程有效。C 壳立即复制返回文本，不释放框架拥有的指针。
- `MessageContinue` / `MessageIgnore` 为 0，`MessageIntercept` 为 1。

## 生命周期、并发与窗口

`botPluginLoad` 只返回元数据；`onInitialize` 每个 Worker 启动时执行一次，设置菜单可在未启用时启动 Worker。后台任务在 `onEnable` 启动，在 `onDisable` 取消并等待。BeeNT 重载只禁用再 `FreeLibrary`，不保证卸载回调；不能把必要清理留到 `onUnload`。

保留两条独立单向命名管道、唯一 IPC reader、请求 ID 关联和写入/等待超时。不要让多个 goroutine 读取同一输入流，或改为共用输入输出句柄。超时不撤销已进入框架的操作，不能盲目重试有副作用的 API。

任务启动前登记 WaitGroup，退出时 Done；动态启动与禁用采用同一锁下的启用状态检查。共享配置和状态需同步，阻塞外部调用设置取消或超时。禁用必须停止 Worker，重新启用会重建内存状态。

设置窗口保留专用 OS 线程、消息循环、单实例以及关闭等待。窗口正在创建时立即禁用也必须收尾；配置 I/O 与网络请求不阻塞窗口线程。Worker 数据统一通过 `GetAppDataDir()` 获取，不写入临时 DLL 目录。

原始 stdout 专供 IPC，诊断用 stderr 或文件。GUI 宿主可能没有控制台，保留 `bridge.log` 和 `worker-stderr.log`。正常消息不应逐条同步写原生桥接日志。

## 格式、Git 与验证

所有修改过的 Go 源码运行 `gofmt`。不采用高于 Go 1.20 的语言或标准库特性。文本编码和换行以 `.gitattributes` 为准；批处理与导出表保留原始字节，批处理使用 ASCII/CRLF，其它源码和 Markdown 使用 UTF-8/LF。Windows PowerShell 脚本增加中文时需考虑 5.1 的 UTF-8 BOM 识别，不能统一清除已有 BOM。

维护 SDK 时使用固定编译器运行 `go test -count=1 ./...`、`go vet ./...`，分别覆盖 Windows/amd64、Windows/386。内部测试按 [底层测试索引](other/README.md) 选择，CRT/栈探测改动需额外运行 `run-crt-probe.ps1`。race 仅在工具链和宿主支持时运行；当前记录的检测器启动失败不能写成通过。

SDK 1.0.1 已获用户 Server 2012 R2 环境 16/16 通过反馈，历史步骤见 [实机记录](docs/history/BeeNT实机验收.md)。新业务在真实 BeeNT 中按实际功能验证，不把内部测试或旧反馈等同于新业务已实测。
