# BeeNT Go SDK：AI 开发指南

## 架构边界

```text
Bee×NT 框架（32 位进程）
  └─ 纯 C 插件壳 build\插件名.dll（other\beent_bridge.c）
       ├─ 导出 BeeNT_初始化 与 botPluginLoad（均指向 BeeNT_Initialize，stdcall）
       ├─ 工作进程生命周期：释放、启动、停止
       ├─ GBK 与 UTF-8 转换
       └─ 通过核心模块地址调用框架 API

beent_go_worker.exe（内嵌在 DLL 里，运行时释放到 plugin_data\插件名\）
  ├─ plugin_main.go    插件业务回调
  ├─ beent_sdk.go      常量、数据类型、框架 API 1～97、IPC 与事件分发
  ├─ settings.go       原生设置窗口与宿主监控
  └─ JSON Lines IPC（标准输入输出）
```

禁止把 Go 运行时、cgo 或 `buildmode=c-archive` 放进插件 DLL：Go 运行时不能随 `FreeLibrary` 卸载，宿主会崩溃。只支持 Windows 32 位（`GOOS=windows GOARCH=386`）。

## 业务代码位置

只改 `plugin_main.go`。四个常量决定插件在框架里的信息，构建工具会读取它们生成 C 头文件：`PluginName`、`PluginAuthor`、`PluginVersion`、`PluginDescription`。`PluginName` 同时是 DLL 默认文件名和 `plugin_data` 下的目录名，不能含 Windows 文件名禁止的字符。

回调已按框架参数顺序拆好：

| 回调 | 参数 |
| --- | --- |
| `onEnable` / `onDisable` / `onUnload` / `onSettings` | 无 |
| `onFriendMessage` | `friendQQ, content, messageID` |
| `onGroupMessage` | `groupID, senderQQ, content, messageID` |
| `onEventMessage` | `eventType, source, triggerNick, triggerQQ, operatorNick, operatorQQ, content, flag` |

消息类回调返回 `MessageIgnore`（0，让后面的插件继续处理）或 `MessageIntercept`（1，拦截）。事件类型用 `EventType` 常量判断，例如 `EventGroupJoinRequest`。

## 调用框架

```go
api, err := requireAPI()
if err != nil {
	return MessageIgnore
}
_, _ = api.SendGroupMessage(groupID, "你好")
```

方法清单见 [API参考.md](API参考.md)。

## 纪律

1. 框架 API 只能在回调执行期间调用。C 壳只在等待 `event_result` 时处理 `api_call`，回调之外发出的调用得不到应答，所以 SDK 直接返回 `ErrOutsideCallback`。不要在回调里启动 goroutine 去调用框架后再让回调先返回。
2. 工作进程的标准输出只能是 IPC。`main` 已把 `os.Stdout` 改道到标准错误，插件里的 `fmt.Print`、`log` 不会破坏协议；需要落盘的日志写到 `GetAppDataDir()` 下。
3. 回调里的 panic 会被 `dispatchEvent` 捕获，作为 `event_result` 的错误返回，工作进程不会因此退出。
4. 事件参数传到 Go 时已是 UTF-8，C 壳在发送前完成 GBK 到 UTF-8 的转换，Go 侧只做 Base64 还原。
5. 数据只写 `GetAppDataDir()` 返回的 `框架目录\plugin_data\插件名\`，不要写在 DLL 旁边：框架会把 DLL 复制到 `temp_plugin` 再加载。
6. Bee×NT 没有独立的初始化回调。框架调用 `BeeNT_初始化` 时只取回插件信息和函数指针，核心地址与插件索引由 C 壳通过命令行 `--core`、`--plugin-index` 交给工作进程，`requireAPI()` 拿到的就是已初始化好的对象。

## 事件与 IPC

C 壳发给工作进程的 `event` 消息，`Event` 字段取值：

| Event | 作用 |
| --- | --- |
| `enable` / `disable` / `unload` / `settings` | 对应生命周期回调；`unload` 后工作进程退出 |
| `friend_message` / `group_message` / `event_message` | 对应消息回调 |
| `stop` | 关闭设置窗口并退出工作进程，不调用插件回调 |

工作进程发出 `api_call`（`command_b64`）后同步等待 ID 匹配的 `api_result`（`value_b64` 或 `error`）。未知事件和参数解码失败都通过 `event_result.Error` 返回。

## 生命周期

- 启用时启动任务；禁用时停止任务并关闭设置窗口；卸载时做最终清理。
- 工作进程会监控宿主进程，宿主退出后自行结束；标准输入 EOF 时 `runWorker` 也会正常返回。

## 构建流程

`build.bat` 分四步：

1. `go run ./other/buildmeta`：读取 `plugin_main.go` 常量，生成 `temp\plugin_config.h`、`temp\worker.rc`，并把 `other\worker_runtime.go`（带 `beent_worker_runtime` 构建标签的 `main` 入口）去掉构建标签后复制成根目录的 `worker_runtime.go`。
2. 用 Go 1.20.14 编译 32 位工作进程，编译后立即删除根目录的 `worker_runtime.go`，并核对产物的 Go 版本。
3. `zig rc` 把工作进程编成资源。
4. `zig cc` 链接纯 C 的 PE32 DLL。

根目录的 `worker_runtime.go` 是临时生成物，已在 `.gitignore` 中忽略，不要手改或提交；入口代码的唯一来源是 `other\worker_runtime.go`。

## 验证边界

本地测试文件（`*_test.go`，不入库）加 `go vet ./...` 覆盖 IPC 往返、事件分发、命令行参数、API 参数顺序与返回解析。生命周期、真实消息收发、设置窗口、工作进程退出和 DLL 卸载稳定性，必须在真实的 Bee×NT 环境里验收。
