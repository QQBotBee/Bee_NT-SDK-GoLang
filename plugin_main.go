// 本文件是 BeeNT 插件的业务入口，通常从这里修改插件信息和实现功能。
// 设置界面位于 settings.go；业务变大后可在根目录增加同属 package main 的辅助文件。
// 开发说明见 README.md、docs/AI开发指南.md；完整接口见 docs/API参考.md。
//
// 构建：运行 build.bat，或 build.bat MyPlugin.dll；build 目录只输出最终 DLL。
// 工具链统一使用 Go 1.20.14，插件业务运行在 DLL 内嵌的独立 Go Worker 中。
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// 插件信息由构建工具读取，并生成 botPluginLoad 返回的元数据。
// 保留字符串字面量，文本须能用 GBK 表示；不要在这里调用函数或读取配置。
// PluginName 同时决定插件列表名称、默认 DLL 名、数据目录名和默认设置窗口标题。
// 名称须符合 Windows 目录名规则；修改已有插件名称时注意迁移原有配置。
const (
	// PluginName 是框架显示的插件名称。
	PluginName = "BeeNT Go 示例插件"
	// PluginAuthor 是插件作者。
	PluginAuthor = "BeeNT Go SDK"
	// PluginVersion 是插件版本。
	PluginVersion = "1.0.0"
	// PluginDescription 是插件功能说明。
	PluginDescription = "使用原生 C 壳与独立 Go Worker 开发的 BeeNT 插件"
)

// pluginTasks 管理仅在插件启用期间运行的后台任务。
// ctx/cancel 的读取、任务登记和禁用状态变更使用同一把锁；每个任务退出时调用 Done。
// 其它共享业务状态仍需单独同步，不能因为消息回调串行执行就忽略后台并发。
var pluginTasks struct {
	sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// 以下回调由 Worker 运行时调用，无需自己注册函数指针或编写初始化 JSON。
// NewBeeNTAPI() 取得当前 Worker 的连接与插件索引，不需要旧 Bee 的 robotJSON。
// API 对象不保存当前消息上下文；回复、请求处理等操作应显式传入本次回调的 ID 和 flag。
//
// 消息与事件按接收顺序执行；HTTP 请求、定时器和长循环应交给可取消的后台任务。
// Go 字符串使用 UTF-8，GBK 边界由 SDK 处理；QQ、群号和消息 ID 保留字符串。
// MessageContinue / MessageIgnore 为 0，MessageIntercept 为 1；只拦截需要独占的指令。

// logPlugin 优先输出框架日志，连接或调用失败时将错误写入标准错误流。
// Worker 的原始 stdout 用于 IPC；诊断使用框架日志、os.Stderr 或插件数据目录。
func logPlugin(message string) {
	api, err := NewBeeNTAPI()
	if err == nil {
		err = api.Log(message)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "BeeNT:", err)
	}
}

// onInitialize 每个 Worker 进程启动时调用一次，适合读取配置和准备基础资源。
// botPluginLoad 只返回元数据，不执行这里的业务；首次启用或打开设置才启动 Worker。
// 设置可以在启用之前打开，因此基础配置读取放在这里，长期任务放在 onEnable。
// 不要长时间阻塞初始化；禁用后 Worker 退出，下次启动会重新执行本函数。
func onInitialize() {
	logPlugin("Go Worker 初始化完成")

	// 示例：取得配置、数据库、缓存和业务日志的统一保存目录。
	// dataDir, err := GetAppDataDir()
	// if err != nil {
	//     logPlugin("取得数据目录失败：" + err.Error())
	//     return
	// }
	// logPlugin("插件数据目录：" + dataDir)
	// 使用 filepath.Join(dataDir, "config.json") 拼接路径时需导入 path/filepath。
	// 在此读取自己的配置，并处理文件不存在、格式错误和旧配置缺字段等情况。
	// 不要根据 DLL 路径或当前工作目录拼接数据目录。
}

// onEnable 在插件启用时调用，创建本次启用周期的取消上下文。
// 可在下面完成运行状态初始化，启动定时任务、外部监听器等服务。
// 每个启用周期只启动一次；不要在每条消息回调里重复启动同一个长期任务。
func onEnable() {
	pluginTasks.Lock()
	if pluginTasks.cancel != nil {
		pluginTasks.cancel()
	}
	pluginTasks.ctx, pluginTasks.cancel = context.WithCancel(context.Background())
	pluginTasks.Unlock()
	logPlugin("插件已启用")

	// 后台任务接入方法见 docs/AI开发指南.md 的“后台任务：登记、取消、等待”。
	// 沿用这里的 pluginTasks，在同一把锁内检查 ctx/cancel 并执行 wg.Add(1)，
	// 再启动 goroutine；退出时 defer pluginTasks.wg.Done()。
	// 循环监听 ctx.Done()，HTTP/数据库等阻塞操作需设置超时并传递 ctx。
	// 禁用会等待任务退出，不能启动无法结束的 goroutine。
}

// onDisable 在禁用或重载时调用，负责取消并等待任务、释放运行资源和关闭设置窗口。
// BeeNT 重载可能只禁用再释放 DLL，必要清理必须放在这里，不能留给 onUnload。
// 清理应允许重复执行，也要处理只打开过设置、尚未启用的情况。
// 随后原生壳停止 Worker，下次启用创建新进程，内存状态不会自动保留。
func onDisable() {
	pluginTasks.Lock()
	if pluginTasks.cancel != nil {
		pluginTasks.cancel()
		pluginTasks.cancel = nil
	}
	pluginTasks.Unlock()
	// 若任务阻塞在监听器等资源上，在等待前发送退出信号或关闭用于唤醒的句柄。
	pluginTasks.wg.Wait()
	// 任务结束后释放仍被任务使用的共享资源，并保存需要保留的状态。
	// 配置、数据库和业务日志统一通过 GetAppDataDir() 确定保存位置。
	closeSettingsWindow()
	logPlugin("插件已禁用")
}

// onUnload 在框架正常卸载插件时执行，重载不保证收到它。
// 复用 onDisable 的清理，避免任务或窗口残留；自己增加的资源释放也须允许重复调用。
// 框架卸载可能删除插件数据目录，长期保留的数据应有备份。
func onUnload() {
	onDisable()
	logPlugin("插件已卸载")
}

// onSettings 在用户点击插件设置时调用，打开或聚焦单实例原生设置窗口。
// 可以在尚未启用插件时触发，不能假定 onEnable 已创建运行任务。
// 展示文案修改 settings.go 的 defaultSettingsModel()；当前模板没有配置输入或保存控件。
// 增加配置功能时还需实现输入校验、持久化和业务生效；耗时操作不阻塞窗口线程。
func onSettings() {
	showSettingsWindow()
}

// onPrivateMessage 处理机器人收到的好友私聊消息。
// 默认不自动回复；下面是可取消注释的精确命令示例，结果返回到当前好友会话。
// 返回 MessageContinue 继续后续插件处理，MessageIntercept 拦截当前消息。
func onPrivateMessage(
	friendID string, // 好友 QQ，即本条私聊消息发送人
	message string, // 收到的消息内容，已转换为 Go UTF-8 字符串
	messageID string, // 本条消息 ID，可用于引用；可能为负数，保留原始字符串
) int {
	// 示例：收到“菜单”时向当前好友发送一条消息。
	// if message != "菜单" {
	//     return MessageContinue
	// }
	// api, err := NewBeeNTAPI()
	// if err != nil {
	//     logPlugin("取得框架连接失败：" + err.Error())
	//     return MessageContinue
	// }
	// result, err := api.Friend(friendID).SendText("你好，这里是 BeeNT Go 插件")
	// if err != nil {
	//     logPlugin("发送好友消息失败：" + err.Error())
	// } else {
	//     logPlugin("发送好友消息结果：" + result)
	// }
	// 引用当前消息时，将上方 SendText 调用替换为：
	// result, err := api.Friend(friendID).Reply(messageID, "收到你的消息")

	// err == nil 表示未发生调用错误，框架业务失败仍可能返回原因文本。
	// 发送成功通常返回消息 ID；不要把任意非空结果直接当作可撤回的消息 ID。
	return MessageContinue
}

// onGroupMessage 处理机器人收到的群聊消息。
// groupID 是回复目标，userID 是发送人；业务权限应根据发送人和群配置判断。
// 返回 MessageContinue 继续后续插件处理，MessageIntercept 拦截当前消息。
func onGroupMessage(
	groupID string, // 消息来源群号
	userID string, // 本条群消息发送人的 QQ
	message string, // 收到的群消息内容
	messageID string, // 本条群消息 ID，引用回复时显式传入
) int {
	// 示例：收到“菜单”时向来源群发送一条消息。
	// if message != "菜单" {
	//     return MessageContinue
	// }
	// api, err := NewBeeNTAPI()
	// if err != nil {
	//     logPlugin("取得框架连接失败：" + err.Error())
	//     return MessageContinue
	// }
	// result, err := api.Group(groupID).SendText("请选择功能")
	// if err != nil {
	//     logPlugin("发送群消息失败：" + err.Error())
	// } else {
	//     logPlugin("发送群消息结果：" + result)
	// }
	// 引用当前消息时，将上方 SendText 调用替换为：
	// result, err := api.Group(groupID).Reply(messageID, "收到你的群消息")
	// 查询发送人资料可使用 api.GetGroupMemberInfo(groupID, userID)。
	// 禁言、踢人、修改群资料等操作须先检查业务权限及机器人权限。
	// 撤回自己的回复使用 api.RecallMessage(已确认有效的发送结果ID)，不要使用指令的 messageID。

	return MessageContinue
}

// onEvent 处理好友、群管理、消息变化、请求和框架连接等事件。
// 各事件只填充相关字段，其余可能为空；以事件类型解释 message 和来源，不统一视为聊天消息。
// userID 与 operatorID 可能不同，例如被禁言成员与执行禁言的管理员，不能互换。
// 返回 MessageContinue 继续后续处理，MessageIntercept 拦截当前事件。
func onEvent(
	eventType string, // 事件类型，使用 beent_sdk.go 中的字符串常量，不使用旧 Bee 的事件编号
	sourceID string, // 事件来源，群事件通常为群号，好友事件按框架提供的来源处理
	userNickname string, // 触发者或被操作对象的昵称，可能为空
	userID string, // 触发者或被操作对象的 QQ
	operatorNickname string, // 操作者昵称，可能为空
	operatorID string, // 操作者 QQ，例如执行禁言、踢人操作的管理员
	message string, // 事件携带的内容，其具体含义取决于 eventType
	flag string, // 验证请求标识，处理请求时传入本次事件的原始值
) int {
	// 按需取消下列 switch 的注释，在对应分支实现业务。
	// 需要调用框架时先 api, err := NewBeeNTAPI() 并检查 err。
	//
	// switch eventType {
	// case EventGroupFileUpload:
	//     // 群文件上传。
	// case EventGroupAdminAdded:
	//     // 新增群管理员。
	// case EventGroupAdminRemoved:
	//     // 取消群管理员。
	// case EventGroupMemberLeft:
	//     // 成员主动退群。
	// case EventGroupMemberKicked:
	//     // 成员被移出群，区分 userID 与 operatorID。
	// case EventRobotKicked:
	//     // 机器人被移出群，可清理该群的运行状态。
	// case EventGroupMemberInvited:
	//     // 成员受邀入群。
	// case EventGroupMemberJoined:
	//     // 成员加入群。
	// case EventGroupMemberBanned:
	//     // 成员被禁言。
	// case EventGroupMemberUnbanned:
	//     // 成员解除禁言。
	// case EventGroupMessageRecalled:
	//     // 群消息被撤回，按该事件约定读取携带的内容。
	// case EventGroupReactionAdded:
	//     // 群消息增加表情回应。
	// case EventGroupReactionRemoved:
	//     // 群消息移除表情回应。
	// case EventGroupEssenceAdded:
	//     // 群消息被设为精华。
	// case EventGroupJoinRequest:
	//     // 加群申请；核对来源、申请人、flag 和业务审批条件后处理。
	// case EventGroupInvitation:
	//     // 群邀请；按邀请业务规则决定是否接受。
	// case EventGroupTitleChanged:
	//     // 群成员专属头衔变化。
	// case EventGroupHonorChanged:
	//     // 群荣誉变化。
	// case EventGroupNameChanged:
	//     // 群名称变化。
	// case EventFriendAdded:
	//     // 新增好友。
	// case EventFriendMessageRecalled:
	//     // 好友消息被撤回。
	// case EventFriendRequest:
	//     // 好友申请；审批后使用 api.HandleFriendRequest(userID, flag, approve, remark)。
	// case EventPoke:
	//     // 戳一戳，按来源判断是否为群内事件。
	// case EventGroupWholeBanEnabled:
	//     // 群开启全员禁言。
	// case EventGroupWholeBanDisabled:
	//     // 群关闭全员禁言。
	// case EventGroupCardChanged:
	//     // 群名片变化。
	// case EventRobotOffline:
	//     // 机器人离线。
	// case EventWebSocketDisconnected:
	//     // 框架 WebSocket 连接断开。
	// case EventWebSocketReconnected:
	//     // 框架 WebSocket 连接恢复。
	// default:
	//     // 未处理或后续新增的事件继续传递。
	// }
	//
	// GroupRequestApply / GroupRequestInvite 是处理加群请求时的 requestType 常量，
	// 不是需要在上方补上的两个普通事件分支。确认业务类型后使用：
	// api.HandleGroupRequest(sourceID, userID, flag, approve, requestType, reason)
	// 其中 approve、remark、requestType、reason 由业务规则提供，不默认自动同意请求。

	return MessageContinue
}
