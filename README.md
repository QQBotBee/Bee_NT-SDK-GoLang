# BeeNT Go Plugin SDK

使用 Go 开发 Bee×NT 机器人框架插件的模板。

纯 C 插件壳 + 独立 Go 工作进程 + 单 DLL 交付，支持安全卸载。

## 为什么这样设计

Bee×NT 加载 32 位 DLL，按 stdcall 调用导出函数，文本使用 GBK。Go 运行时不能随 DLL 被 `FreeLibrary` 卸载，否则宿主会崩溃。所以 Go 业务跑在独立工作进程里：C 壳被框架加载，负责导出、编码转换和进程管理；Go 工作进程作为资源内嵌在 DLL 中，运行时释放到框架目录的 `plugin_data\插件名\` 下。

```text
build\插件名.dll
├── 纯 C 壳（other\beent_bridge.c）
│   ├── 导出 BeeNT_初始化 与 botPluginLoad
│   ├── 把生命周期和消息转发给工作进程
│   ├── GBK 与 UTF-8 互转
│   └── 通过核心模块地址调用框架 API
└── 内嵌的 32 位 Go 工作进程
    ├── plugin_main.go 里的插件业务
    ├── beent_api.go 里的框架 API 1～97
    └── settings.go 里的原生设置窗口
```

## 环境要求

| 工具 | 要求 |
| --- | --- |
| Go | 必须是 1.20.14，且是 PATH 里第一个 `go.exe` |
| Zig | 已加入 PATH，用来编译 C 壳 |
| 系统 | Windows，框架本身是 32 位进程 |

本机的 Go 1.20.14 在 `D:\Softs\go1.20.14\go\bin`。构建脚本会检查版本，版本不对直接停止，不会自动下载或切换。

## 构建

在本目录打开 CMD，执行：

```bat
build.bat
```

也可以指定输出名：

```bat
build.bat MyPlugin.dll
```

产物是 `build\插件名.dll`。把这个 DLL 放进框架的 `plugin` 目录，在框架里添加即可。框架会复制一份到 `temp_plugin` 再加载，所以不要把数据写在 DLL 旁边，用 `GetAppDataDir()` 拿到的目录。

## 写插件

业务只写在 `plugin_main.go`。四个常量决定插件在框架里显示的信息：

```go
const (
    PluginName        = "BeeNTGoPlugin"
    PluginAuthor      = "BeeNT Go SDK"
    PluginVersion     = "1.0.0"
    PluginDescription = "Bee×NT Go 插件开发样例"
)
```

回调函数已经按框架的参数顺序拆好，直接用具名参数，不用管 IPC：

```go
func onGroupMessage(groupID, senderQQ, content, messageID string) int {
    api, err := requireAPI()
    if err != nil {
        return MessageIgnore
    }
    if content == "ping" {
        _, _ = api.SendGroupMessage(groupID, "pong")
    }
    return MessageIgnore
}
```

返回 `MessageIgnore`（0）让后面的插件继续处理，返回 `MessageIntercept`（1）拦截。

| 回调 | 框架何时调用 |
| --- | --- |
| `onEnable` | 插件被启用 |
| `onDisable` | 插件被禁用或框架重载 |
| `onUnload` | 插件被卸载 |
| `onSettings` | 用户点设置 |
| `onFriendMessage` | 好友消息：好友 QQ、内容、消息 ID |
| `onGroupMessage` | 群消息：群号、发送人 QQ、内容、消息 ID |
| `onEventMessage` | 事件：事件类型、来源、触发者昵称与 QQ、操作者昵称与 QQ、内容、flag |

事件类型用 `EventType` 常量判断，例如 `EventGroupJoinRequest`。加群申请的验证类型用 `GroupRequestApply`（"26"）和 `GroupRequestInvite`（"27"）。

Bee×NT 没有独立的初始化回调。框架调用 `BeeNT_初始化` 时只取回插件信息和函数指针，核心地址与插件索引由 C 壳通过命令行传给工作进程，`requireAPI()` 拿到的就是已经初始化好的对象。

## 调用框架 API

`beent_api.go` 封装了框架 API 1～97，方法编号与易语言 SDK 的框架调用编号一一对应。

```go
api, err := requireAPI()
info, err := api.RobotInfo()          // API 1
id, err := api.SendGroupMessage(groupID, "你好") // API 4，成功返回消息 ID
err = api.Ban(groupID, qq, 60)        // API 29，成功返回 nil，失败带原因
admins, err := api.GetGroupAdminList(groupID) // API 76
```

约定与框架一致：发送类接口成功返回消息 ID、失败返回原因；设置类接口成功返回空、失败返回原因，这里统一转成 `*FrameworkError`。

两个和易语言 SDK 不同、按框架实际实现修正的地方：

- `GetGroupEmojiLikes`（API 96）第一个参数是群号。易语言 SDK 把消息 ID 放在了第一位，框架接收端读的是群号，那个顺序是 SDK 写错了。
- `GetVoiceToText`（API 97）只传消息 ID。易语言 SDK 把消息 ID 传了两遍，框架只用第一个。

`GetFilteredFriendRequests` 的数量传 0 或负数时不传数量，让框架使用默认的 10 条。

## 数据目录

```go
dir, err := api.GetAppDataDir()
```

返回 `框架目录\plugin_data\插件名\`，末尾带反斜杠。配置、数据库、缓存和日志都写这里。工作进程本身也释放在这个目录，宿主退出后工作进程会自动结束，不会残留。

## 设置窗口

`settings.go` 是一个原生 Win32 窗口：单实例、居中、置顶、不可调整大小。改 `defaultSettingsModel` 里的标题和说明即可。`onSettings` 里调用 `showSettingsWindow()`，重复点击只会把已有窗口置前。

## 测试

```bat
D:\Softs\go1.20.14\go\bin\go.exe test ./...
```

测试覆盖 IPC 往返、事件分发、命令行参数、API 参数顺序和返回解析。完整的生命周期、消息回调和设置窗口仍需要在真实的 Bee×NT 里验证。

## 目录

| 文件 | 作用 |
| --- | --- |
| `plugin_main.go` | 插件业务，改这里 |
| `beent_sdk.go` | 常量、数据类型、协议分隔符 |
| `beent_api.go` | 框架 API 1～97 |
| `worker.go` | 工作进程的 IPC 与事件分发 |
| `settings.go` | 设置窗口与宿主监控 |
| `other\beent_bridge.c` | 纯 C 壳 |
| `other\BeeNTPlugin.def` | 导出定义，GBK 编码，不要改编码 |
| `other\buildmeta` | 构建期读取插件常量、生成 C 头 |
| `build.bat` | 一键构建 |
