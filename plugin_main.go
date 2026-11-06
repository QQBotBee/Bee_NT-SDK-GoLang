package main

// 插件信息常量，由构建工具读取并生成框架初始化返回的 JSON。
// 名称同时用作 plugin_data 下的目录名与默认 DLL 文件名，不能含 Windows 文件名禁止的字符。
const (
	PluginName        = "BeeNTGoPlugin"
	PluginAuthor      = "BeeNT Go SDK"
	PluginVersion     = "1.0.0"
	PluginDescription = "Bee×NT Go 插件开发模板"
)

// 所有回调都在框架调用期间同步执行。框架 API 只能在回调内调用，
// 在回调之外（例如回调返回后仍在运行的 goroutine）调用会返回 ErrOutsideCallback。
// 需要调用框架时先取 API 对象：
//
//	api, err := requireAPI()
//	if err != nil {
//		return MessageIgnore
//	}

// onEnable 在插件启用时调用。
func onEnable() {}

// onDisable 在插件禁用或框架重载时调用，负责停止任务并关闭设置窗口。
func onDisable() {
	closeSettingsWindow()
}

// onUnload 在插件卸载时调用，做最终清理。
func onUnload() {}

// onSettings 在用户点击插件设置时调用。
func onSettings() {
	showSettingsWindow()
}

// onFriendMessage 处理好友消息。
// 返回 MessageIntercept 拦截消息，返回 MessageIgnore 让后面的插件继续处理。
func onFriendMessage(friendQQ, content, messageID string) int {
	// 示例：api.SendFriendMessage(friendQQ, "你好")
	_, _, _ = friendQQ, content, messageID
	return MessageIgnore
}

// onGroupMessage 处理群消息。
// 返回 MessageIntercept 拦截消息，返回 MessageIgnore 让后面的插件继续处理。
func onGroupMessage(groupID, senderQQ, content, messageID string) int {
	// 示例：api.SendGroupMessage(groupID, "大家好")
	_, _, _, _ = groupID, senderQQ, content, messageID
	return MessageIgnore
}

// onEventMessage 处理框架事件，事件类型取 Event* 常量。
// triggerQQ 是被作用的人（B 被 A 踢，B 是触发者），operatorQQ 是动手的人（A 是操作者）。
// 加群申请事件可用 api.HandleGroupRequest(source, triggerQQ, flag, true, GroupRequestApply, "") 处理。
// 返回 MessageIntercept 拦截事件，返回 MessageIgnore 继续传递给后续插件。
func onEventMessage(eventType, source, triggerNick, triggerQQ, operatorNick, operatorQQ, content, flag string) int {
	switch EventType(eventType) {
	case EventGroupJoinRequest:
		// 有人申请加群
	case EventFriendAddRequest:
		// 有人申请加好友
	}
	_, _, _, _, _, _, _, _ = eventType, source, triggerNick, triggerQQ, operatorNick, operatorQQ, content, flag
	return MessageIgnore
}
