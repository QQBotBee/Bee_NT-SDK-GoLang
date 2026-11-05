<div align="center">

# BeeNT Go Plugin SDK

**使用 Go 开发 BeeNT 机器人框架插件的开发模板**

纯 C 插件壳 · 独立 Go Worker · 单 DLL 交付 · 支持禁用与重载

![Go](https://img.shields.io/badge/Go-1.20.14-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows-0078D4)
![Architecture](https://img.shields.io/badge/Architecture-x86%20%2F%2032--bit-555555)
![Build](https://img.shields.io/badge/Build-Go%20%2B%20Zig-F7A41D)
![Encoding](https://img.shields.io/badge/BeeNT-GBK%20%2F%20CP936-blueviolet)

[快速开始](#快速开始) · [开发插件](#开发插件) · [SDK 示例](#sdk-示例) · [设置窗口](#设置窗口) · [架构说明](#架构说明) · [常见问题](#常见问题) · [开发文档](#开发文档)

</div>

---

## 项目简介

BeeNT Go Plugin SDK 是一套面向 **BeeNT 机器人框架**的 Go 插件开发模板，提供插件信息、生命周期、好友消息、群消息、事件回调和框架 API 封装。

模板采用“**纯 C 插件 DLL + 独立 Go Worker**”架构：BeeNT 进程加载原生 DLL，Go 业务运行在独立进程中。插件被禁用或重载时，先结束业务、关闭窗口和停止 Worker，再释放 DLL，避免将 Go Runtime 放进需要反复加载、卸载的宿主模块。

最终只需分发一个 DLL。Go Worker 作为资源内嵌在 DLL 中，由插件运行时自动释放、启动和清理，运行机器无需安装 Go 或 Zig。

```text
单个插件 DLL
├── 纯 C BeeNT 插件壳
│   ├── botPluginLoad 初始化入口
│   ├── 生命周期、消息与事件回调
│   ├── Worker 释放与进程管理
│   ├── GBK / UTF-8 编码转换
│   └── BeeNT 框架 API 调用
└── 内嵌 Go Worker
    ├── Go 插件业务
    ├── BeeNT Go SDK
    ├── 原生设置窗口
    └── JSON Lines IPC
```

## 核心特性

- **Go 编写业务**：生命周期、消息、事件和后台任务均可使用 Go 实现。
- **单 DLL 交付**：Worker 内嵌，无需额外分发 EXE、SDK 源码或 Go 运行环境。
- **固定工具链**：统一使用 **Go 1.20.14**，以 Windows Server 2012 R2 为兼容基线。
- **完整生命周期**：提供初始化、启用、禁用、卸载和设置入口。
- **具名回调参数**：直接使用 `friendID`、`groupID`、`userID`、`messageID` 等业务参数。
- **97 个框架 API**：覆盖提供的 BeeNT 参考中 1～97 操作码，提供具名方法和 Friend/Group 链式调用。
- **事件与请求常量**：提供参考协议中 1～31 的常量值，包含好友、群管理、消息、连接状态事件及加群请求类型。
- **类型化查询**：机器人、好友、群和群成员信息，以及常用列表均有对应 Go 结构。
- **自动编码转换**：处理框架 GBK/CP936 与 Go UTF-8 边界，支持中文和 Unicode 转义。
- **后台 API 调用**：消息回调之外的受管理任务和设置业务也可以调用 SDK。
- **插件数据目录**：通过 `GetAppDataDir()` 统一保存配置、数据库、缓存和日志。
- **原生设置窗口**：支持单实例、固定尺寸、首次居中、置顶及禁用时关闭。
- **一键构建**：根目录仅使用 `build.bat`，完成编译、资源嵌入、链接及内部校验。

## 环境要求

| 环境/工具 | 要求 | 说明 |
| --- | --- | --- |
| 构建机器 | Windows x64，能够运行 Windows PowerShell | 使用 CMD 双击或执行 `build.bat` |
| Go | 固定 Go 1.20.14 | 构建脚本自动下载、校验并缓存官方 Windows/amd64 工具链 |
| Zig | 已加入系统 PATH | 当前本地验证版本为 0.17.0，可用 `zig version` 检查 |
| BeeNT | 支持 `botPluginLoad` 协议的 32 位插件宿主 | 编译产物固定为 Windows/386 |
| 运行系统 | 以 Windows Server 2012 R2 为兼容基线 | 同一工具链用于后续 Windows 系统，实际功能在目标框架中验证 |

构建脚本使用 Windows CMD、Windows PowerShell、Go 和 Zig，无需安装 MinGW、Python 或 Node.js。首次构建需要联网取得工具链和 Go 模块依赖；工具链缓存位于 `temp/toolchains/`。

所有插件统一使用 Go 1.20.14，无需按运行系统选择不同编译版本。`go.mod` 的 `go 1.20` 是语言兼容声明，实际编译器及最终内嵌 Worker 的版本由构建脚本检查，不使用 PATH 中的新版 Go 代替。

## 快速开始

### 1. 获取项目

从仓库下载源码并解压，或使用仓库提供的 Git 地址克隆。将整个工程作为插件模板保留，进入包含 `plugin_main.go` 和 `build.bat` 的项目根目录。

确认 Zig 已加入 PATH：

```bat
zig version
```

Go 1.20.14 由构建脚本取得，无需为本项目切换系统 Go 版本。

### 2. 修改插件信息

打开根目录的 `plugin_main.go`：

```go
const (
    PluginName        = "我的 BeeNT 插件"
    PluginAuthor      = "作者名称"
    PluginVersion     = "1.0.0"
    PluginDescription = "插件功能说明"
)
```

`PluginName` 是插件名称来源，同时决定：

- BeeNT 插件列表显示的名称；
- 默认生成的 DLL 文件名；
- `plugin_data` 下的插件数据目录名；
- 默认设置窗口的标题。

四项信息保持字符串字面量，构建时会从源码提取并生成元数据。文本需要能够用 GBK 表示，插件名称还须符合 Windows 目录名规则。修改已有插件名称时，应考虑旧数据目录中的配置迁移。

### 3. 编写插件业务

业务代码主要写在 `plugin_main.go`。模板已经提供所有生命周期和消息入口，默认仅输出生命周期日志，不自动回复消息。

例如，将群消息回调替换为下面的代码：

```go
func onGroupMessage(groupID, userID, message, messageID string) int {
    if message != "菜单" {
        return MessageContinue
    }

    api, err := NewBeeNTAPI()
    if err != nil {
        logPlugin("取得框架连接失败：" + err.Error())
        return MessageContinue
    }

    result, err := api.Group(groupID).SendText("你好，这里是 BeeNT Go 插件")
    if err != nil {
        logPlugin("发送菜单失败：" + err.Error())
    } else {
        logPlugin("发送菜单结果：" + result)
    }
    return MessageContinue
}
```

这个示例只响应原群中的精确命令“菜单”，其它消息继续交给后续插件。`logPlugin()` 已由模板提供，使用框架日志，连接不可用时回退到标准错误输出。

### 4. 构建插件

在 Windows 中双击：

```bat
build.bat
```

也可以通过命令行指定 DLL 名称：

```bat
build.bat MyPlugin.dll
```

构建成功后，最终插件位于：

```text
build\插件名称.dll
```

`build` 目录只保留最终 DLL，临时 EXE、RES、LIB 和 PDB 等构建文件会自动清理。构建窗口默认停留显示结果；终端或自动化可使用：

```bat
build.bat --no-pause
build.bat --no-pause MyPlugin.dll
```

指定 DLL 文件名只改变输出名称，不改变插件信息和数据目录。插件列表显示的名称仍以 `PluginName` 为准。

### 5. 加载插件

在真实 BeeNT 的插件管理器中添加生成的 DLL，确认名称和版本后启用，然后按自己的业务功能发送消息、触发事件或打开设置。

Worker 会自动释放到：

```text
BeeNT宿主EXE目录\plugin_data\插件名称\
```

运行时 Worker 文件名包含宿主 PID、模块地址和序号，便于区分不同加载实例；正常停止后自动清理。配置和业务数据继续保存在该插件的数据目录中。

## 开发插件

### 业务代码放在哪里

普通插件开发通常修改：

```text
plugin_main.go     插件信息、生命周期、消息和事件业务
settings.go        需要自定义设置界面时修改
```

业务变大后，可在根目录新增职责明确的 `.go` 辅助文件，继续使用 `package main`。SDK、IPC 和原生壳已提供基础能力，先复用现有接口。

### 生命周期入口

| 回调 | 触发时机 | 推荐用途 |
| --- | --- | --- |
| `onInitialize()` | 每个 Worker 第一次启动时 | 读取配置、创建业务资源、准备基础状态 |
| `onEnable()` | 插件被启用 | 创建取消上下文，启动定时任务、监听器和运行期服务 |
| `onDisable()` | 插件被禁用或重载 | 取消并等待任务、释放运行资源、关闭设置窗口 |
| `onUnload()` | 框架正常卸载插件时 | 最终清理，不能作为唯一的任务释放入口 |
| `onSettings()` | 用户点击插件设置 | 打开或聚焦原生设置窗口 |

BeeNT 的 `botPluginLoad(core,index)` 先返回插件信息和七个原生回调地址，此时不启动 Go 业务。第一次需要 Worker 时，运行时才先调用 `onInitialize()`，再执行对应业务回调。

常见顺序如下：

```text
加载 DLL
└── botPluginLoad 返回插件信息与回调地址

首次启用
└── 启动 Worker
    ├── onInitialize
    └── onEnable

点击设置
└── onSettings
    └── 打开或聚焦窗口

禁用或重载
└── onDisable
    ├── 取消并等待后台任务
    ├── 关闭设置窗口
    └── 停止 Worker

再次启用
└── 新 Worker：onInitialize → onEnable
```

设置菜单允许在插件尚未启用时打开，这时会先启动 Worker 并调用 `onInitialize()`，再调用 `onSettings()`。因此，读取基础配置不应只放在 `onEnable()`。

BeeNT 重载会禁用插件并释放 DLL，不保证触发 `onUnload()`。必要清理必须放在 `onDisable()`；当前模板的 `onUnload()` 也会复用禁用清理，业务资源释放应允许重复调用。

### 消息与事件入口

| 回调 | 参数 | 用途 |
| --- | --- | --- |
| `onPrivateMessage` | `friendID, message, messageID` | 好友私聊消息 |
| `onGroupMessage` | `groupID, userID, message, messageID` | 群聊消息 |
| `onEvent` | `eventType, sourceID, userNickname, userID, operatorNickname, operatorID, message, flag` | 框架事件，共八个文本字段 |

所有回调参数均为字符串。群消息回调的参数含义为：

```go
func onGroupMessage(
    groupID string,   // 来源群号
    userID string,    // 发送者 QQ
    message string,   // 消息内容
    messageID string, // 本条消息 ID，用于引用等操作
) int {
    return MessageContinue
}
```

事件类型使用 `types.go` 中的常量，例如 `EventPoke`、`EventGroupCardChanged`、`EventFriendRequest`。`sourceID` 是事件来源，`userID` 与 `operatorID` 分别表示触发者/被操作对象和操作者，不能互换；各事件的部分字段可以为空。处理验证请求时保留原始 `flag`。

### 消息处理结果

```go
return MessageContinue  // 0：继续投递给后续插件
return MessageIntercept // 1：拦截，不再投递给后续插件
return MessageIgnore    // 0：与 MessageContinue 同值
```

处理了业务不代表必须拦截。一般回复可以返回 `MessageContinue`，只有明确需要独占的命令才返回 `MessageIntercept`。

### 后台任务与禁用清理

耗时网络请求、定时器和长循环应放进可取消的后台任务，避免阻塞后续消息。模板的 `pluginTasks` 已提供 context、取消函数和 WaitGroup。

例如在业务辅助文件中增加下面的函数，并导入 `time`，然后在 `onEnable()` 创建 context 后调用 `startPeriodicTask()`：

```go
func startPeriodicTask() {
    pluginTasks.Lock()
    if pluginTasks.cancel == nil || pluginTasks.ctx == nil || pluginTasks.ctx.Err() != nil {
        pluginTasks.Unlock()
        return
    }
    ctx := pluginTasks.ctx
    pluginTasks.wg.Add(1)
    pluginTasks.Unlock()

    go func() {
        defer pluginTasks.wg.Done()
        ticker := time.NewTicker(time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                if ctx.Err() != nil {
                    return
                }
                logPlugin("定时任务运行中")
            }
        }
    }()
}
```

每个启用周期只启动一次。任务在启动前登记，退出时执行 `Done()`，禁用时取消并等待。共享状态仍需锁或其它同步；外部 HTTP、数据库操作设置超时并传递取消信号。更完整的任务组织规则见 [AI 开发指南](docs/AI开发指南.md#后台任务登记取消等待)。

## SDK 示例

### 创建 API 对象

在业务函数中取得当前 Worker 的 API 连接：

```go
api, err := NewBeeNTAPI()
if err != nil {
    logPlugin("取得框架连接失败：" + err.Error())
    return
}
```

这里的片段用于不带返回值的业务辅助函数；放入消息或事件回调时，错误分支使用 `return MessageContinue`。后续示例均假定 `api` 已创建，相关 ID 来自本次回调，各示例独立使用。

### 输出日志

```go
if err := api.Log("插件正在运行"); err != nil {
    fmt.Fprintln(os.Stderr, "输出框架日志失败：", err)
}
```

模板已有 `fmt` 和 `os` 导入。原始 stdout 是 IPC 通道，业务日志写框架日志、`os.Stderr` 或插件数据目录。

### 发送好友消息

```go
result, err := api.Friend(friendID).SendText("你好")
if err != nil {
    logPlugin("发送好友消息失败：" + err.Error())
    return
}
logPlugin("好友消息发送结果：" + result)
```

### 发送群聊消息

```go
result, err := api.Group(groupID).SendText("大家好，中文与 emoji 😀")
if err != nil {
    logPlugin("发送群消息失败：" + err.Error())
    return
}
logPlugin("群消息发送结果：" + result)
```

也可以直接调用具名方法 `api.SendFriendMessage(friendID, content)` 和 `api.SendGroupMessage(groupID, content)`，与链式接口对应。

### 引用当前消息

```go
result, err := api.Group(groupID).Reply(messageID, "收到，这是一条引用回复")
if err != nil {
    logPlugin("引用回复失败：" + err.Error())
    return
}
logPlugin("引用回复结果：" + result)
```

好友引用使用 `api.Friend(friendID).Reply(messageID, content)`。参数顺序为**消息 ID、正文**；使用当前会话、当前指令的消息 ID。

### 撤回自己发送的消息

```go
result, err := api.RecallMessage(sentMessageID)
if err != nil {
    logPlugin("提交撤回失败：" + err.Error())
    return
}
if result != "" {
    logPlugin("框架撤回返回：" + result)
}
```

`sentMessageID` 应是本插件此前在对应会话成功发送并确认有效的消息 ID，不是用户指令的 ID，也不能是接口返回的错误文本。实际撤回结果以真实 BeeNT 和客户端效果为准。

### 查询机器人信息

```go
robot, err := api.GetRobotInfo()
if err != nil {
    logPlugin("查询机器人信息失败：" + err.Error())
    return
}
logPlugin("当前机器人：" + robot.Nickname + "（" + robot.UserID + "）")
```

`GetFrameworkName()`、`GetFrameworkVersion()`、`GetFrameworkQQ()` 可用于读取框架基本信息。

### 查询群与群成员

```go
group, err := api.GetGroupInfo(groupID)
if err != nil {
    logPlugin("查询群信息失败：" + err.Error())
    return
}
member, err := api.GetGroupMemberInfo(groupID, userID)
if err != nil {
    logPlugin("查询群成员失败：" + err.Error())
    return
}
logPlugin("群：" + group.Name + "，成员：" + member.Nickname)
```

`GroupMemberInfo` 包含昵称、群名片、等级、身份、入群时间、最后发言时间和头衔。身份 `Role` 为 `0` 成员、`1` 管理员、`2` 群主；时间戳使用 `int64`。

### 遍历好友列表

```go
friends, err := api.GetFriendList()
if err != nil {
    logPlugin("查询好友列表失败：" + err.Error())
    return
}
for _, friend := range friends {
    logPlugin(friend.QQ + "：" + friend.Nickname)
}
```

群列表使用 `GetGroupList()`，群成员列表使用 `GetGroupMemberList(groupID)`。SDK 负责解析框架的分隔符格式，返回结构切片，无需业务再次拆分文本。

### 获取插件数据目录

```go
dataDir, err := api.GetAppDataDir()
if err != nil {
    logPlugin("取得数据目录失败：" + err.Error())
    return
}
configPath := filepath.Join(dataDir, "config.json")
logPlugin("配置文件位置：" + configPath)
```

此片段需要导入 `path/filepath`。也可以直接调用包级函数 `GetAppDataDir()`。目录及保存规则见 [数据目录与持久化](#数据目录与持久化)。

### 其它常用能力

| 需求 | 接口示例 |
| --- | --- |
| 好友/群语音 | `api.Friend(friendID).SendVoice(voice)` / `api.Group(groupID).SendVoice(voice)` |
| 好友/群视频 | `api.Friend(friendID).SendVideo(video)` / `api.Group(groupID).SendVideo(video)` |
| JSON 消息 | `api.Group(groupID).SendJSON(content)` |
| 上传文件 | `api.Group(groupID).SendFile(filePath, fileName)` |
| 文本艾特、表情 | `api.At(userID)`、`api.Face(faceID)` |
| 处理好友请求 | `api.HandleFriendRequest(userID, flag, approve, remark)` |
| 处理加群请求 | `api.HandleGroupRequest(groupID, userID, flag, approve, requestType, reason)` |
| 群管理 | `api.SetGroupBan(...)`、`api.SetGroupCard(...)`、`api.SetGroupName(...)` |
| 消息表情点赞 | `api.SetMessageEmojiLike(messageID, emojiID, enabled)` |
| 语音转文字 | `api.GetVoiceToText(messageID)` |

媒体参数、管理权限、请求类型和返回语义以 [API 参考](docs/API参考.md) 及真实框架行为为准。对管理类操作设置业务权限检查，避免任意消息触发管理行为。

## API 返回值与消息 ID

### 区分调用错误和框架结果

| 返回形式 | 含义 | 业务处理 |
| --- | --- | --- |
| `(string, error)` 的 `error != nil` | 参数校验或通信等调用故障 | 记录错误并停止当前处理 |
| 发送消息返回文本 | 成功通常为消息 ID，失败可能为原因文本 | 按对应接口确认有效结果，再保存消息 ID |
| 修改类操作返回文本 | 成功通常为空文本，失败通常为原因文本 | 结合该 API 的约定和真实效果处理 |
| 结构体、切片或布尔值与 `error` | SDK 对查询结果进行类型解析 | 检查 `error`，再使用解析后的数据 |

`err == nil` 不能统一等同于框架业务成功。框架返回原因文本时，SDK 会保留它；不要把任意非空文本都当作消息 ID，也不要把空文本一律当成失败。

### 保留 ID 和事件上下文

- QQ、群号、消息 ID 全部保留为字符串，不转换成 32 位整数或浮点数。
- 消息 ID 允许负数，保留原值。
- 引用显式传入本次消息 ID；撤回记录自己发送的有效消息 ID，并与会话关联。
- 请求处理使用对应事件中的原始 `flag`，不要复用上一条请求的数据。
- `NewBeeNTAPI()` 绑定当前 Worker 的插件索引与传输，不接收旧 Bee 的机器人 JSON。
- BeeNT 使用自己的 97 个操作码和回调协议，迁移旧 Bee 业务时需核对接口，不直接套用频道回调或旧操作码。

## 数据目录与持久化

插件数据统一保存到：

```text
BeeNT宿主EXE目录\plugin_data\插件名称\
```

使用 `GetAppDataDir()` 取得目录，再通过 `filepath.Join` 构造文件路径。配置、数据库、缓存和业务日志均可放在这里；不要把持久化数据写到 DLL 所在位置、临时插件目录或当前工作目录。

禁用和重载会停止 Worker，下次启用是一个新进程，内存变量不会自动保留。需要跨重载保存的配置、计数或任务状态应写入文件或数据库，读取时处理文件不存在、格式错误和旧配置缺字段等情况。

保存配置时先校验，写入成功后再更新运行状态或提示成功；共享配置采用锁或不可变快照。框架卸载插件可能删除数据目录，需要长期保留的数据应有备份。

## 设置窗口

`onSettings()` 默认调用 `showSettingsWindow()`。窗口运行在独立 Worker 中，当前模板提供：

- 默认 640×420 客户区，按系统 DPI 缩放；
- 首次居中、始终置顶，支持拖动和最小化；
- 固定尺寸，不提供拉伸和最大化；
- 重复点击只恢复并聚焦已有窗口；
- 关闭后可以重新打开；
- 禁用时关闭并等待窗口线程退出，包括窗口仍在创建的情况。

修改展示文案可调整 `settings.go` 的 `defaultSettingsModel()`：

```go
func defaultSettingsModel() settingsModel {
    return settingsModel{
        Title:       PluginName + " 设置",
        Description: "这里填写插件功能说明和配置提示。",
        Status:      "欢迎使用本插件",
    }
}
```

当前窗口是说明和状态展示模板。需要输入框、保存按钮或动态配置时，还要实现控件消息、输入校验、持久化及业务生效流程；仅修改文案不会自动增加配置功能。

窗口可以在插件未启用时打开，配置读取不能依赖启用任务已存在。网络请求和耗时操作不应直接放进窗口消息处理或绘制函数。完整的线程、绘制、资源释放和关闭规则见 [设置窗口开发规范](docs/设置窗口开发规范.md)。

## 项目结构

```text
.
├── README.md                    # 项目说明与使用示例
├── AGENTS.md                    # AI 与贡献者开发约定
├── .gitignore / .gitattributes  # 生成物忽略、文本与字节保存规则
├── plugin_main.go               # 插件信息、生命周期、消息和事件业务
├── settings.go                  # Windows 原生设置窗口
├── settings_nonwindows.go       # 非 Windows 编译桩
├── beent_sdk.go                 # SDK 构造、命令协议、编码和数据目录
├── api_methods.go               # 97 个具名 API 与 Friend/Group 链式方法
├── types.go                     # 信息结构、列表解析、事件与缓存常量
├── ipc.go                       # API 请求关联、并发与超时
├── worker_runtime.go            # Worker 入口、消息读取与事件分发
├── *_test.go                    # 对应源码的回归测试
├── build.bat                    # 唯一构建入口
├── go.mod / go.sum              # Go 1.20 兼容声明与模块依赖
├── docs/
│   ├── README.md               # 文档导航
│   ├── API参考.md              # 97 个 API、返回类型与事件说明
│   ├── AI开发指南.md           # 提示词、开发规则与业务示例
│   └── 设置窗口开发规范.md     # 窗口线程、布局、资源与关闭约束
├── other/
│   ├── build.ps1               # build.bat 调用的内部构建实现
│   ├── build-tools.ps1         # 固定工具链与产物内部检查
│   ├── beent_bridge.c          # 原生 stdcall 壳、框架 API 与 Worker 管理
│   ├── beent_crt.h             # 无 CRT 辅助实现
│   ├── beent_stack_probe.S     # x86 栈探测
│   ├── BeeNTPlugin.def         # botPluginLoad 导出定义
│   ├── buildmeta/              # 插件元数据与资源输入生成器
│   └── tests/                  # SDK 内部回归工具与测试数据
├── build/                      # 最终 DLL，不提交
└── temp/                       # 工具链缓存与临时文件，不提交
```

根目录 Go 文件同属一个插件包，按 SDK、业务、IPC 和窗口职责拆分。`worker_runtime.go` 是维护源码，直接参与编译；`*_test.go` 保留在对应源码旁。底层实现和测试工具集中在 `other/`，日常业务开发通过根目录 `build.bat` 构建。

## 架构说明

### 为什么使用独立 Worker

BeeNT 重载插件时会禁用并释放 DLL。Go Runtime 在宿主进程中直接随 DLL 反复加载、卸载，存在运行时线程、内存和卸载时序问题。因此，模板让原生壳负责宿主边界，Go Runtime 留在独立进程内。

```text
BeeNT.exe
└── 纯 C 插件 DLL
    ├── 32 位 stdcall ABI
    ├── botPluginLoad 与七个回调地址
    ├── GBK / UTF-8 边界
    ├── 框架 API 进程内调用
    └── Worker 生命周期管理
              ↕ 两条独立单向命名管道
独立 Go Worker
├── plugin_main.go 与业务辅助文件
├── BeeNT Go SDK
├── 原生设置窗口
└── IPC reader 与请求结果关联
```

`botPluginLoad` 返回 `name`、`author`、`ver`、`desc`，以及 `enable`、`disable`、`uninstall`、`setting`、`private`、`group`、`event` 七个原生回调地址。业务开发无需手工生成这些地址或初始化 JSON。

### SDK 调用如何到达框架

```text
BeeNT 回调
→ C 壳传入消息和事件字段
→ Worker 解码为 Go 字符串
→ 执行业务回调
→ SDK 发出 api_call
→ C 壳在 BeeNT 进程内调用框架 API
→ Worker 收到对应 api_result
→ 业务回调返回处理结果
```

两条单向管道分别负责一个方向的通信，唯一 reader 按请求 ID 分发 API 结果。消息和事件业务按顺序处理，等待 SDK 结果时仍能接收响应；后台任务也可通过同一连接调用 API。

框架 API 函数地址只在 BeeNT 进程内有效，Go Worker 不直接调用该地址。C 壳立即复制框架返回文本，不释放框架拥有的指针。

### 编码与调用边界

- BeeNT 宿主文本使用 GBK/CP936，Go 业务与 IPC 使用 UTF-8。
- 不能直接由 GBK 表示的字符由 SDK 转为框架支持的 UTF-16 `\uXXXX` 转义，emoji 使用代理对。
- 框架命令分隔符为 `#|&BEExNT&|#`，参数不能包含该保留分隔符或 NUL，SDK 会校验。
- `Call` 保留返回原文。JSON 直接交给 `json.Unmarshal`，不能先替换其中的 Unicode 转义；普通文本需要时使用 `DecodeFrameworkText`。
- SDK 等待有超时，但超时不能撤销已经进入框架的操作；发送、撤回等有副作用的请求不能盲目自动重试。
- 原始 stdout 专供 IPC。业务日志明确写框架日志、`os.Stderr` 或文件，不直接操作标准输出句柄。

## 编码与构建约束

- 所有产物统一使用 **Go 1.20.14**，源码和依赖保持 Go 1.20 兼容。
- 构建目标固定为 `Windows/386`，Worker 使用 `CGO_ENABLED=0`。
- Go Runtime 与 cgo 不进入 DLL；原生壳仅导入 KERNEL32.dll，不依赖 UCRT。
- 根目录 `build.bat` 和 `other/BeeNTPlugin.def` 使用 ASCII/CRLF，Git 按原始字节保存。
- Go、Markdown 及其它主要文本按 `.gitattributes` 使用 UTF-8/LF；Windows PowerShell 脚本新增中文时需注意编码识别。
- `build/`、`temp/`、运行数据、DLL、EXE、RES、LIB 和 PDB 等不提交；SDK 源码和回归测试需要保留。

## 开发文档

| 文档 | 内容 |
| --- | --- |
| [AI 开发指南](docs/AI开发指南.md) | 可直接复制的开发提示词、读取顺序、可靠性规则和任务示例 |
| [API 参考](docs/API参考.md) | 全部 97 个 API 的参数、类型、返回值、常量和参考勘误 |
| [设置窗口开发规范](docs/设置窗口开发规范.md) | 单实例、DPI、线程、绘制、配置与关闭流程 |
| [文档导航](docs/README.md) | 按业务开发或 SDK 维护选择阅读资料 |
| [开发约定](AGENTS.md) | 仓库边界、协议与工具链要求 |

让 AI 开发插件时，可从 [AI 开发指南的提示词](docs/AI开发指南.md#可直接复制的开发提示词) 开始，提供插件信息、功能触发条件、权限、配置字段和后台任务需求。

## 常见问题

### 构建提示找不到 Zig

确认 `zig.exe` 所在目录已加入 PATH，重新打开 CMD 后执行 `zig version`。应能输出版本号，再重新运行 `build.bat`。

### 本机安装的是新版 Go，会不会用错版本

不会直接使用系统新版 Go 生成插件。构建脚本统一取得缓存中的 Go 1.20.14，核验编译器和最终 DLL 中的 Worker；无需切换系统 Go，也不要仅修改 `go.mod` 来替代版本检查。

### 第一次构建较慢或工具链下载失败

首次构建要下载并解压固定工具链，同时取得 Go 模块依赖。检查构建窗口的错误、网络连接及工程目录写权限，再重新运行 `build.bat`。下载文件会经过 SHA256 校验，缓存位于 `temp/toolchains/`；删除整个 `temp/` 后，下次需要重新取得工具链。

### 指定了 MyPlugin.dll，为什么插件列表名称没变

命令行参数只指定输出文件名。插件名称、版本、说明和数据目录由 `plugin_main.go` 中的四项常量决定，修改后重新构建。

### 插件能加载，为什么发送普通消息没有回复

默认模板仅提供回调和生命周期日志，没有启用自动回复。请在 `onPrivateMessage` 或 `onGroupMessage` 中加入自己的命令处理，并确认插件已启用、触发条件和会话正确。

### 返回 err 为 nil，为什么操作没有成功

`error` 通常表示参数、通信或结果解析故障；框架业务失败可能作为返回文本。发送消息、修改群资料、撤回等 API 的成功约定不同，应检查对应返回值与真实客户端效果。

### 设置菜单点击后没有窗口

先检查数据目录中的 `bridge.log` 和 `worker-stderr.log`。Worker 启动失败也会导致设置无反应；窗口本身的问题再对照设置入口和关闭流程排查。设置在未启用时也应能打开，基础配置不要只在 `onEnable()` 中初始化。

### 中文、emoji 或 JSON 转义显示异常

Go 源码保存为 UTF-8，业务字符串交给 SDK 处理，不要重复做 GBK 转换。普通文本还原转义时使用 `DecodeFrameworkText`；JSON 先使用 `json.Unmarshal`，避免提前替换转义字符破坏 JSON。

插件名称、作者等构建元数据需要能够用 GBK 表示，不能将消息内容支持 emoji 误认为所有元数据都可以直接使用 emoji。

### 重载后内存状态为什么消失

禁用会停止 Worker，再次启用创建新进程。需要保留的配置、计数和业务状态应写入 `GetAppDataDir()` 对应目录，并在初始化时读取。

### 为什么最终只分发一个 DLL，运行时却能看到 EXE

Worker 已内嵌在 DLL 中。运行时自动释放到插件数据目录并启动，正常停止后清理，无需额外分发；不要直接双击 Worker，它需要宿主参数和 IPC 连接。

## 在真实 BeeNT 中验证

完成业务后运行 `build.bat`，将 DLL 导入真实 BeeNT，检查本次实现的实际功能：

- 插件名称、版本正确，加载和启用正常；
- 私聊、群聊和事件按预期触发，回复到正确会话；
- 所用 API 的服务端操作与客户端显示符合预期；
- 中文、换行和 emoji 显示正常；
- 设置能打开、重复点击为单实例、关闭后可重开；
- 配置保存后可读取，禁用和重载后需要保留的数据仍在；
- 禁用后后台任务、设置窗口和 Worker 正常结束；
- 多次重载和退出框架无持续累积的窗口或 Worker。

SDK 1.0.1 已在真实 BeeNT / Windows Server 2012 R2 环境完成 16 项基础功能实机验证，详见 [实机验证记录](docs/history/BeeNT实机验收.md)。新增业务和新使用的 API 仍需在目标框架中验证。

## 贡献与维护

提交改动时保持 BeeNT 的 stdcall ABI、API 编号、参数顺序和编码边界，源码使用 `gofmt`。普通业务优先修改业务与设置文件；涉及 SDK、IPC 或原生壳时，按 [内部维护说明](other/README.md) 执行相关回归。

真实框架的账号权限、服务端结果和界面交互通过实际 BeeNT 操作确认。说明改动内容和实际验证范围，不提交构建产物或临时文件。
