# BeeNT Go SDK：AI 开发指南

本指南用于让 AI 在当前模板上开发业务插件。先以本仓库源码确定接口，再实现需求；完整的 97 个框架 API 见 [API参考](API参考.md)，设置界面见 [设置窗口开发规范](设置窗口开发规范.md)。这是一个 `package main` 的插件工程，业务与 SDK 一起编译为独立 Worker，最终分发一个内嵌 Worker 的 Windows x86 DLL。

## 可直接复制的开发提示词

将下面整段交给 AI，替换方括号内容。首次沟通时一并提供当前工程目录及业务需求。

```text
请基于当前 BeeNT Go SDK 工程实现插件，不另建通用 SDK，不修改已经验证的运行时。

插件名称：[名称]
作者：[作者]
版本：[版本]
目标系统：[Windows 10/Server 2016及以后，或Windows Server 2012 R2]
功能：[触发条件、私聊/群聊范围、输入输出、权限、配置项、持久化规则]
消息处理：[普通消息继续；只有明确列出的命令需要拦截]
后台任务：[触发方式、间隔、超时、取消条件；没有则写无]
设置界面：[字段、默认值、校验、保存/取消行为；没有则保留模板]

开始前按顺序完整读取：
1. AGENTS.md、README.md、docs/AI开发指南.md。
2. plugin_main.go，确认四项元数据与全部业务回调的真实签名。
3. docs/API参考.md、beent_sdk.go、api_methods.go、types.go。
4. worker_runtime.go、ipc.go，理解Worker初始化、事件串行处理、并发API和停止流程。
5. 如改设置，读取docs/设置窗口开发规范.md、settings.go、settings_nonwindows.go。
6. docs/验证记录.md，了解已有验证范围；业务验证直接使用真实BeeNT框架。

通常只修改plugin_main.go、独立业务/配置辅助文件及对应测试；需要设置时修改settings.go。
不要擅改SDK操作码、参数顺序、IPC、原生C壳、导出、工具链和构建脚本。
发现需求确实需要这些底层变更时，先指出现有能力缺口和必要范围，再单独处理。

必须遵守：
- 当前是BeeNT的97个API和31个事件值；以源码中的具名方法、类型和常量为准。
- 不引入Bee旧版的频道功能、事件编号或初始化协议。
- QQ、群号、消息ID、事件type和flag保留字符串，消息ID允许负值。
- 用NewBeeNTAPI()取得当前Worker连接，不从宿主函数地址直接调用API。
- Go业务使用UTF-8；保留SDK的GBK、Unicode转义和JSON解码边界。
- stdout及原始标准输出句柄专供IPC；日志使用os.Stderr或数据目录。
- 长任务移到受管理的后台任务；禁用时取消并等待，窗口关闭并等待线程结束。
- BeeNT重载可能只调用onDisable，不能依赖onUnload清理；禁用随后停止Worker。
- 保持Go1.20源码兼容，所有产物统一使用精确Go1.20.14，兼容Server2012R2。
- 不把Go runtime或cgo链接进DLL，不覆盖构建的最终Worker版本/PE核验。

先简述实际采用的回调、API、配置和文件范围，然后完成实现与适当测试。
交付时列出更改文件、构建产物、实际工具链、验证结果及尚需实机确认的业务行为。
不要把本地模拟成功或既有16项验收扩大为全部97个API已真实联网验证。
```

## 读文件与修改范围

| 文件 | 用途与边界 |
| --- | --- |
| [plugin_main.go](../plugin_main.go) | 四项元数据、初始化、启用/禁用、消息、事件和设置入口，主要业务修改位置 |
| 业务辅助 `.go` / 本地 `*_test.go` | 解析、权限、配置、状态、外部服务；继续使用 `package main`，`*_test.go` 由 Git 忽略，不提交 |
| [settings.go](../settings.go) | Windows 原生窗口模型、布局、绘制和消息处理；只在需要界面变化时修改 |
| [beent_sdk.go](../beent_sdk.go)、[api_methods.go](../api_methods.go)、[types.go](../types.go) | SDK构造、97个API、返回类型、31个事件值；业务开发先读取并复用 |
| [worker_runtime.go](../worker_runtime.go)、[ipc.go](../ipc.go) | 事件分发、唯一reader、API请求关联与超时；普通业务无需修改 |
| [other/](../other/) | 原生壳、导出、资源、内部构建实现与PE门禁；修改需单独验证运行时 |
| `build/`、`temp/` | 生成物，不作为业务源码修改，不提交临时Worker与测试输出 |

修改 `PluginName` 会改变默认 DLL 名和运行数据目录。已有配置需要迁移时，应明确旧目录、迁移条件及失败处理。构建工具从业务源文件提取四项常量，不要改成依赖运行期计算的元数据。普通业务开发无需阅读SDK内部测试夹具；按需求补充业务测试即可。

## 实际架构与初始化

```text
BeeNT宿主（x86）
  └─ 原生C DLL：botPluginLoad(core,index)
       ├─ 保存核心API地址与插件索引
       ├─ 返回name/author/ver/desc及七个stdcall回调地址
       ├─ GBK边界、API泵、Worker启动与停止
       └─ 命名管道/JSON Lines
            └─ 独立Go Worker：SDK + plugin_main.go + settings.go
```

`botPluginLoad` 轻量返回元数据，不执行 Go 业务。第一次真正需要 Worker 时，原生壳先启动 Worker 并派发 `initialize`，再派发目标事件；点击设置可以发生在启用之前。因此 `onInitialize` 每个 Worker 运行一次，`onSettings` 不能假定 `onEnable` 已执行。`NewBeeNTAPI() (*BeeNTAPI, error)` 使用 Worker 已安装的传输与插件索引，无需机器人 JSON 或消息级初始化参数。

核心函数地址只在 BeeNT 宿主进程有效，Go Worker 不得通过 `unsafe`、`syscall` 或 cgo 调用该地址。纯 C 壳中不放 Go runtime、`buildmode=c-archive` 或窗口实现。当前 C 壳无 CRT，只导入 `KERNEL32.dll`；业务开发不改这条链路。

## 回调与返回值

| Go回调 | 参数与职责 |
| --- | --- |
| `onInitialize()` | 读取配置、准备基础资源，避免长时间阻塞 |
| `onEnable()` | 建立任务context，启动启用期间的业务 |
| `onDisable()` | 取消任务、等待结束、关闭设置窗口；随后原生壳停止Worker |
| `onUnload()` | 正常卸载的最终清理；当前模板复用`onDisable`，清理应可重复执行 |
| `onSettings()` | 调用`showSettingsWindow()`，可在未启用时打开 |
| `onPrivateMessage(friendID, message, messageID string) int` | 好友消息，三个字符串参数 |
| `onGroupMessage(groupID, userID, message, messageID string) int` | 群消息，四个字符串参数 |
| `onEvent(eventType, sourceID, userNickname, userID, operatorNickname, operatorID, message, flag string) int` | 八个字符串字段，保留空值和原始flag |

`MessageContinue`、`MessageIgnore` 均为 `0`；`MessageIntercept` 为 `1`。只对明确要求拦截的消息返回 `1`，不要因处理成功就默认拦截。事件类型使用 `types.go` 的字符串常量，例如 `EventPoke`、`EventGroupCardChanged`；值范围为 `1`—`31`，包括 `GroupRequestApply` / `GroupRequestInvite`。触发者与操作者可能不同，不能互换；请求处理时使用原始 `flag`。

业务事件按接收顺序执行。回调里长时间休眠、HTTP等待或无限循环会延迟后续事件；长任务应交给可取消的后台任务。Worker唯一reader在回调执行期间继续分发API结果，回调内同步调用SDK和回调外后台调用均受支持。

## 可复制的群聊业务示例

以下函数可替换模板同名回调，沿用已有 `fmt` / `os` 导入；只响应原群中的精确命令，保持其它消息继续传递。

```go
func onGroupMessage(groupID, userID, message, messageID string) int {
    if message != "菜单" {
        return MessageContinue
    }
    api, err := NewBeeNTAPI()
    if err != nil {
        fmt.Fprintln(os.Stderr, "取得BeeNT连接:", err)
        return MessageContinue
    }
    result, err := api.Group(groupID).Reply(messageID, "你好，请选择功能 😀")
    if err != nil {
        fmt.Fprintln(os.Stderr, "发送群引用回复:", err)
    } else {
        fmt.Fprintln(os.Stderr, "发送结果:", result)
    }
    return MessageContinue
}
```

常用方法的真实签名如下；更复杂功能应查 [API参考](API参考.md) 后选用具名方法，不靠猜测拼操作码。

```go
// 这些是签名说明，不是需要加入工程的声明。
// api.Friend(friendID).SendText(content string) (string, error)
// api.Group(groupID).Reply(messageID, content string) (string, error)
// api.GetGroupMemberInfo(groupID, qq string) (GroupMemberInfo, error)
// api.RecallMessage(messageID string) (string, error)
// api.Log(content string) error
// GetAppDataDir() (string, error)
```

发送成功通常返回消息ID，但框架业务失败也可能返回一段原因文本，`err == nil` 不能统一代表操作成功。修改类API常见成功值为空字符串。需要撤回时，只记录本插件在该会话成功发送并验证过的消息ID，不能拿用户指令的ID或错误文本去撤回。不要将QQ、群号、消息ID转成32位`int`或浮点数；保留负消息ID。

## 后台任务：登记、取消、等待

沿用 `plugin_main.go` 的 `pluginTasks`。`onEnable` 创建context；`onDisable` 在锁内取消并清空cancel，解锁后 `wg.Wait()`，最后 `closeSettingsWindow()`。所有新任务先在同一把锁下确认已启用并登记，再启动goroutine，避免禁用等待开始后又新增任务。

以下辅助函数可放在业务文件中；该文件需导入 `context`。它使用模板已有的 `pluginTasks`，不新增第二套任务管理状态。

```go
func startPluginTask(work func(context.Context)) bool {
    pluginTasks.Lock()
    if pluginTasks.cancel == nil || pluginTasks.ctx == nil || pluginTasks.ctx.Err() != nil {
        pluginTasks.Unlock()
        return false
    }
    ctx := pluginTasks.ctx
    pluginTasks.wg.Add(1)
    pluginTasks.Unlock()
    go func() {
        defer pluginTasks.wg.Done()
        work(ctx)
    }()
    return true
}
```

例如增加以下函数，再在现有 `onEnable` 创建context之后调用 `startPeriodicTask()`；文件需导入 `context` 和 `time`。同一启用周期只启动一次，不能在每条消息到达时重复启动定时器。

```go
func startPeriodicTask() {
    startPluginTask(func(ctx context.Context) {
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
    })
}
```

后台HTTP请求应使用任务context与明确超时；长循环检查取消；计时器及时停止。不能在后台任务内部调用会等待自己的 `onDisable()`，也不能持有 `pluginTasks` 锁等待任务退出。配置、缓存、任务和窗口线程共享状态需要同步，`WaitGroup`只负责等待，不保护数据。

SDK当前没有 `CallContext`；API等待上限为200秒。任务context取消不能撤回已进入框架的API，不能保证中断一个已经等待中的SDK调用。原生禁用有停止期限，业务要避免在退出阶段发起慢API、无限等待或无界重试。API超时也不证明服务端没有执行，发送、撤回、踢人等操作不能自动盲重试。

BeeNT重载可以只调用禁用后 `FreeLibrary`，不经过卸载回调。任务清理必须放在 `onDisable`，不能只放 `onUnload`。禁用停止Worker，下次启用创建新进程，内存状态不保留；持久化配置要在退出前完成。

## 编码、IPC、数据与日志

- Go业务字符串和配置文件使用UTF-8。SDK发出的命令与API结果为UTF-8，C壳负责宿主CP936/GBK转换。宿主回调的 `args_b64` 包装原始GBK字节，Worker解码后才交给业务；JSON Lines外层为UTF-8。不要重复GBK转换。
- 无法用GBK表达的字符由SDK转换为框架支持的UTF-16 `\uXXXX` 转义，emoji使用代理对。业务直接使用中文、换行、emoji，不手工生成代理对。
- `Call`保留返回原文。JSON直接交给 `json.Unmarshal`，不能先调用 `DecodeFrameworkText`；普通文本需要还原转义时才用该函数，类型化昵称等字段已自动还原。
- 参数禁止NUL及 `#|&BEExNT&|#`。SDK校验操作码和参数个数；列表解析交给类型化方法，避免拆错空字段和末尾分隔符。
- 原始stdout专供IPC，不写自定义JSON、日志或原始标准输出句柄。Worker已将Go的 `os.Stdout` 指向stderr，但业务仍应明确使用 `os.Stderr`；不要重新绑定stdout或增加reader抢读管道。
- `GetAppDataDir()` / `api.GetAppDataDir()` 返回 `<BeeNT宿主EXE目录>\plugin_data\<PluginName>`，可用 `filepath.Join` 保存配置。不要依赖当前工作目录、DLL路径、SDK的build目录或临时Worker文件名。框架卸载可能删除数据目录，需要保留的数据应另行备份。
- `bridge.log`记录原生启动/回调失败，`worker-stderr.log`捕获Worker诊断。可按需要添加配置和业务日志，避免把密钥写入日志。

## 构建与验证

保持Go1.20源码兼容，不使用仅存在于较新Go的标准库或语法。所有产物统一使用精确Go1.20.14，兼容Server2012R2；`go.mod`的 `go 1.20` 是兼容声明，不锁定实际编译器。版本、PE、哈希及导入表由构建内部检查，`build/`只输出插件DLL。

```bat
REM 默认读取plugin_main.go，统一精确Go1.20.14。
build.bat

REM 可选指定输出文件名。
build.bat MyPlugin.dll
```

日常双击根目录 `build.bat`，或在CMD使用上面的命令编译当前业务；构建结束保留窗口显示结果。自动化可选加 `--no-pause`。构建完成后，将 `build/` 下生成的插件DLL导入真实BeeNT框架，验证本次功能。内部实现位于 `other/build.ps1`，供构建维护时使用，不是额外的日常入口；其中的 `-GoExecutable` 必须指向实际Go1.20.14可执行文件，版本不符时构建应失败。

开发检查运行 `gofmt`、`go test -count=1 ./...`、`go vet ./...`，并使用精确Go1.20.14在amd64/386验证。运行时或并发变化在工具链和宿主支持时运行 `go test -race ./...`；当前环境已有ThreadSanitizer启动失败记录，实际失败必须如实报告，不能写成race通过。

修改业务要测试命令匹配、权限、原会话路由、错误返回、配置持久化及禁用取消，然后在真实BeeNT中验证消息收发、设置与生命周期。设置变化可由维护者补跑窗口测试；运行时变化才扩展内部原生边界回归。已有验证说明见 [验证记录](验证记录.md)，此前实机操作记录见 [历史验收](history/BeeNT实机验收.md)。不要直接双击Worker程序代替真实框架验证，它需要宿主参数和IPC连接。

2026-11-05，用户确认此前实测的1.0.1版本在真实BeeNT / Server2012R2的16项验收全部通过；这不代表97个API均已真实联网验证，后续变更仍需在真实框架回归。
