# 原生实现、构建工具与测试

日常插件开发从根目录 `plugin_main.go` 开始。这里集中保存 Worker、平台编译桩、原生 DLL、构建支持和模拟宿主；对业务公开的 API、类型和 IPC 统一位于根目录 `beent_sdk.go`。

## 文件职责

| 文件/目录 | 职责 |
| --- | --- |
| `beent_bridge.c` | stdcall 初始化、七个回调、编码转换、Worker 管理、管道与 API 调用 |
| `beent_crt.h` | 无 CRT 链接所需的受限格式、内存和编码辅助实现 |
| `beent_stack_probe.S` | x86 分配栈探测，逐页触发 Windows guard page 扩栈 |
| `BeeNTPlugin.def` | ASCII `botPluginLoad` 导出定义，按原始字节保存 |
| `build.ps1` | 由根目录 build.bat 调用的统一构建实现 |
| `build-tools.ps1` | 临时 Worker 源码组装、固定工具链下载、PE/资源/导入表核验 |
| `worker_runtime.go` | Worker 入口、事件分发与唯一 reader，构建时复制到 temp 编译 |
| `settings_nonwindows.go` | 非 Windows 编译桩，构建时按平台选择 |
| `tests/local/` | 本地 Go 回归测试，由 Git 忽略，不随仓库分发 |
| `buildmeta/main.go` | 从插件常量生成元数据、资源定义与默认输出名 |
| `buildmeta/main_test.go` | 本地元数据生成器回归测试，Git 忽略，不随仓库分发 |
| `tests/` | C 模拟宿主、fixture Worker、内部回归入口与工具链门禁 |
| `tests/fixtures/acceptance/` | 之前实机测试用的业务与测试源码，仅供内部回归 |

最终 DLL 不链接 Go Runtime 或 CRT，只允许导入 KERNEL32.dll。`worker_runtime.go` 和 `settings_nonwindows.go` 是维护源码，通过模板构建标记排除独立编译；`build-tools.ps1` 将它们和根目录业务源码一起复制到 `temp/`，只在临时副本中移除模板标记。

## 选择测试

以下命令在项目根目录执行，使用 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File` 调用。脚本内的模拟宿主需要 Zig 与 Windows；凡使用真实 Go Worker 的构建，仍要遵守根目录的精确编译器要求。

| 修改范围 | 脚本 | 检查内容 |
| --- | --- | --- |
| Go SDK、结构、IPC、运行时 | `tests/run-acceptance-go.ps1 -GoExecutable <go.exe>` | 在 temp 组装源码并带上本地测试，分别执行 amd64/386 测试与 vet |
| 独立验收业务 | `tests/run-acceptance-go.ps1 -GoExecutable <go.exe>` | 隔离源码包，两个架构的测试与 vet |
| C 壳与生命周期 | `tests/run.ps1` | 实际 x86 LoadLibrary、GBK、阻塞入口、停读、并发回调期间禁用/卸载 |
| 最终正式插件的 Worker | `tests/run-go-host.ps1 -DllPath <DLL>` | 真实 Worker 的初始化、业务回调、API 与重载；用于默认业务模板 |
| 最终验收插件的 Worker | `tests/run-acceptance-host.ps1 -DllPath <DLL>` | 私群路由、编码、引用、撤回、后台任务与禁用；要求验收业务 |
| 设置窗口 | `tests/run-settings-host.ps1 -DllPath <DLL> -ExpectedPluginName <名称>` | 未启用时设置、真实窗口、单实例、关闭重开和禁用销毁 |
| Worker 失败诊断 | `tests/run-diagnostic-host.ps1` | GUI 无 stderr、启动失败、异常退出与回调错误日志 |
| 无 CRT 辅助实现与栈探测 | `tests/run-crt-probe.ps1` | 格式/编码/截断、内存重叠、新线程扩栈和页边界 ABI |
| 构建工具链与产物门禁 | `tests/test-build-toolchain.ps1` | 拒绝错误编译器、架构、CGO、外部导入及过高 PE 版本 |
| 替代插件源码与隔离构建 | `tests/run-build-overlay.ps1 -GoExecutable <go.exe>` | 路径含空格、源码隔离、实际 DLL 与固定 Go 1.20.14 |

这些入口仅供 SDK 维护，不属于普通插件构建流程。实际插件可使用窗口宿主辅助检查，例如：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\other\tests\run-settings-host.ps1 -DllPath '.\build\BeeNT Go 示例插件.dll' -ExpectedPluginName 'BeeNT Go 示例插件'
```

`run-build-overlay.ps1` 会启动真实构建并使用共享 `temp/build`，不能与其它构建并行。内部测试输出放在忽略的 `temp/` 下。验收宿主只适用于内部 fixture，不应用于普通插件业务。

## 验证边界

模拟核心能核对 ABI、数据和生命周期，不能代替真实 BeeNT 的实际功能结果。历史证据见 [验证记录](../docs/验证记录.md)。只执行与维护改动相关的检查；普通业务使用根目录 build.bat 构建，导入真实框架验证。
