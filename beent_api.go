package main

import (
	"strconv"
	"strings"
)

// 本文件是 Bee×NT 框架 API 1～97 的 Go 封装，方法编号与易语言 SDK 的
// 框架调用编号一一对应。返回约定与框架一致：发送类接口成功返回消息 ID、
// 失败返回原因；设置类接口成功返回空字符串、失败返回原因。

// ---------- 基础 ----------

// RobotInfo 获取机器人信息，成功返回 user_id 与 nickname（API 1）。
func (api *BeeNTAPI) RobotInfo() (RobotInfo, error) {
	value, err := api.call(1)
	if err != nil {
		return RobotInfo{}, describeError(1, err)
	}
	var info RobotInfo
	if err := decodeJSON(value, &info); err != nil {
		return RobotInfo{}, describeError(1, err)
	}
	return info, nil
}

// Log 输出插件日志（API 2）。
func (api *BeeNTAPI) Log(content string) error {
	_, err := api.call(2, content)
	return describeError(2, err)
}

// ---------- 发送消息 ----------

// SendFriendMessage 发送好友消息，返回消息 ID，失败返回原因（API 3）。
func (api *BeeNTAPI) SendFriendMessage(friendQQ, content string) (string, error) {
	return api.text(3, friendQQ, content)
}

// SendGroupMessage 发送群消息，返回消息 ID，失败返回原因（API 4）。
func (api *BeeNTAPI) SendGroupMessage(groupID, content string) (string, error) {
	return api.text(4, groupID, content)
}

// SendFriendVoice 发送好友语音消息，语音为网址或路径，无需转码（API 5）。
func (api *BeeNTAPI) SendFriendVoice(friendQQ, voice string) (string, error) {
	return api.text(5, friendQQ, voice)
}

// SendGroupVoice 发送群语音消息，语音为网址或路径，无需转码（API 6）。
func (api *BeeNTAPI) SendGroupVoice(groupID, voice string) (string, error) {
	return api.text(6, groupID, voice)
}

// UploadFriendImage 上传好友图片，返回图片文本代码，配合发送消息使用（API 7）。
func (api *BeeNTAPI) UploadFriendImage(image string) (string, error) {
	return api.text(7, image)
}

// UploadGroupImage 上传群图片，返回图片文本代码，配合发送消息使用（API 8）。
func (api *BeeNTAPI) UploadGroupImage(image string) (string, error) {
	return api.text(8, image)
}

// SendFriendVideo 发送好友视频消息，视频为网址或路径（API 9）。
func (api *BeeNTAPI) SendFriendVideo(friendQQ, video string) (string, error) {
	return api.text(9, friendQQ, video)
}

// SendGroupVideo 发送群视频消息，视频为网址或路径（API 10）。
func (api *BeeNTAPI) SendGroupVideo(groupID, video string) (string, error) {
	return api.text(10, groupID, video)
}

// ---------- 文本代码 ----------

// TextAt 生成艾特文本代码，qq 为 all 时代表全体成员（API 11）。
func (api *BeeNTAPI) TextAt(qq string) (string, error) { return api.text(11, qq) }

// TextRPS 生成猜拳魔法表情文本代码（API 12）。
func (api *BeeNTAPI) TextRPS() (string, error) { return api.text(12) }

// TextDice 生成掷骰子魔法表情文本代码（API 13）。
func (api *BeeNTAPI) TextDice() (string, error) { return api.text(13) }

// ---------- 卡片与分享 ----------

// SendGroupFriendCard 发送群推荐好友卡片消息（API 14）。
func (api *BeeNTAPI) SendGroupFriendCard(groupID, qq string) (string, error) {
	return api.text(14, groupID, qq)
}

// SendFriendFriendCard 发送好友推荐好友卡片消息（API 15）。
func (api *BeeNTAPI) SendFriendFriendCard(friendQQ, qq string) (string, error) {
	return api.text(15, friendQQ, qq)
}

// SendGroupGroupCard 发送群推荐群卡片消息（API 16）。
func (api *BeeNTAPI) SendGroupGroupCard(groupID, recommendedGroupID string) (string, error) {
	return api.text(16, groupID, recommendedGroupID)
}

// SendFriendGroupCard 发送好友推荐群卡片消息（API 17）。
func (api *BeeNTAPI) SendFriendGroupCard(friendQQ, recommendedGroupID string) (string, error) {
	return api.text(17, friendQQ, recommendedGroupID)
}

// CustomMusic 是自定义音乐分享的参数。
type CustomMusic struct {
	Cover   string // 封面 URL
	Type    string // 音乐平台类型：qq/163/kugou/migu/kuwo/custom，custom 代表自定义
	URL     string // 点击后跳转 URL
	Audio   string // 音频 URL，可空
	Title   string // 音乐标题，可空
	Summary string // 音乐简介，不支持换行，可空
}

// SendFriendCustomMusic 发送好友自定义音乐分享消息（API 18）。
func (api *BeeNTAPI) SendFriendCustomMusic(friendQQ string, music CustomMusic) (string, error) {
	return api.text(18, friendQQ, music.Cover, music.Type, music.URL, music.Audio, music.Title, music.Summary)
}

// SendGroupCustomMusic 发送群自定义音乐分享消息（API 19）。
func (api *BeeNTAPI) SendGroupCustomMusic(groupID string, music CustomMusic) (string, error) {
	return api.text(19, groupID, music.Cover, music.Type, music.URL, music.Audio, music.Title, music.Summary)
}

// SendFriendMusic 发送好友音乐分享消息，平台类型为 qq/163/kugou/migu/kuwo（API 20）。
func (api *BeeNTAPI) SendFriendMusic(friendQQ, platform, musicID string) (string, error) {
	return api.text(20, friendQQ, platform, musicID)
}

// SendGroupMusic 发送群音乐分享消息，平台类型为 qq/163/kugou/migu/kuwo（API 21）。
func (api *BeeNTAPI) SendGroupMusic(groupID, platform, musicID string) (string, error) {
	return api.text(21, groupID, platform, musicID)
}

// ---------- 合并、JSON、XML 消息 ----------

// SendFriendForward 发送好友自定义合并消息（API 22）。
// 节点格式为 QQ||昵称||消息内容，消息内容中的换行会自动转成 \n。
func (api *BeeNTAPI) SendFriendForward(friendQQ string, nodes []ForwardNode) (string, error) {
	return api.text(22, friendQQ, formatForward(nodes))
}

// SendGroupForward 发送群自定义合并消息（API 23）。
func (api *BeeNTAPI) SendGroupForward(groupID string, nodes []ForwardNode) (string, error) {
	return api.text(23, groupID, formatForward(nodes))
}

// SendGroupJSON 发送群 JSON 消息（API 24）。
func (api *BeeNTAPI) SendGroupJSON(groupID, jsonContent string) (string, error) {
	return api.text(24, groupID, jsonContent)
}

// SendFriendJSON 发送好友 JSON 消息（API 25）。
func (api *BeeNTAPI) SendFriendJSON(friendQQ, jsonContent string) (string, error) {
	return api.text(25, friendQQ, jsonContent)
}

// SendGroupXML 发送群 XML 消息（API 26）。
func (api *BeeNTAPI) SendGroupXML(groupID, xmlContent string) (string, error) {
	return api.text(26, groupID, xmlContent)
}

// SendFriendXML 发送好友 XML 消息（API 27）。
func (api *BeeNTAPI) SendFriendXML(friendQQ, xmlContent string) (string, error) {
	return api.text(27, friendQQ, xmlContent)
}

// TextFace 生成表情文本代码，表情 ID 参考表情 CQ 码 ID 表（API 28）。
func (api *BeeNTAPI) TextFace(faceID string) (string, error) { return api.text(28, faceID) }

// ---------- 群管理 ----------

// Ban 禁言群成员，duration 单位为秒，0 为取消禁言（API 29）。
func (api *BeeNTAPI) Ban(groupID, qq string, duration int) error {
	return api.ok(29, groupID, qq, itoa(duration))
}

// WholeBan 设置全员禁言，enable 为真开启、为假关闭（API 30）。
func (api *BeeNTAPI) WholeBan(groupID string, enable bool) error {
	return api.ok(30, groupID, boolText(enable))
}

// Kick 踢出群成员，reject 为真时拒绝此人的加群请求（API 31）。
func (api *BeeNTAPI) Kick(groupID, qq string, reject bool) error {
	return api.ok(31, groupID, qq, boolText(reject))
}

// SetGroupAdmin 设置或取消群管理员，enable 为真设置、为假取消（API 32）。
func (api *BeeNTAPI) SetGroupAdmin(groupID, qq string, enable bool) error {
	return api.ok(32, groupID, qq, boolText(enable))
}

// SetGroupCard 设置群名片，card 为空时取消群名片（API 33）。
func (api *BeeNTAPI) SetGroupCard(groupID, qq, card string) error {
	return api.ok(33, groupID, qq, card)
}

// SetGroupName 设置群名称（API 34）。
func (api *BeeNTAPI) SetGroupName(groupID, name string) error {
	return api.ok(34, groupID, name)
}

// LeaveGroup 退群，dismiss 仅在退出人是群主时有效，为真时解散群（API 35）。
func (api *BeeNTAPI) LeaveGroup(groupID string, dismiss bool) error {
	return api.ok(35, groupID, boolText(dismiss))
}

// SetGroupTitle 设置群头衔，title 为空时删除群头衔（API 36）。
func (api *BeeNTAPI) SetGroupTitle(groupID, qq, title string) error {
	return api.ok(36, groupID, qq, title)
}

// ---------- 请求处理 ----------

// HandleFriendRequest 处理好友请求，remark 仅在同意时有效，留空不添加备注（API 37）。
func (api *BeeNTAPI) HandleFriendRequest(qq, flag string, approve bool, remark string) error {
	return api.ok(37, qq, flag, boolText(approve), remark)
}

// HandleGroupRequest 处理加群请求，requestType 取 GroupRequestApply 或 GroupRequestInvite（API 38）。
func (api *BeeNTAPI) HandleGroupRequest(groupID, qq, flag string, approve bool, requestType, reason string) error {
	return api.ok(38, groupID, qq, flag, boolText(approve), requestType, reason)
}

// ---------- 信息查询 ----------

// GetStrangerInfo 获取陌生人信息，noCache 为真时不使用缓存（API 39）。
func (api *BeeNTAPI) GetStrangerInfo(qq string, noCache bool) (FriendInfo, error) {
	return api.friendInfo(39, qq, boolText(noCache))
}

// GetFriendList 获取好友列表（API 40）。
func (api *BeeNTAPI) GetFriendList() ([]FriendListItem, error) {
	value, err := api.call(40)
	if err != nil {
		return nil, describeError(40, err)
	}
	var items []FriendListItem
	for _, line := range splitLines(value) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		items = append(items, FriendListItem{
			QQ:       field(fields, 0),
			Nickname: field(fields, 1),
			Gender:   atoi(field(fields, 2)),
		})
	}
	return items, nil
}

// GetFriendInfo 获取好友信息，noCache 为真时不使用缓存（API 41）。
func (api *BeeNTAPI) GetFriendInfo(friendQQ string, noCache bool) (FriendInfo, error) {
	return api.friendInfo(41, friendQQ, boolText(noCache))
}

// RecallMessage 撤回消息，成功返回空，失败返回原因（API 42）。
func (api *BeeNTAPI) RecallMessage(messageID string) error { return api.ok(42, messageID) }

// GetGroupInfo 获取群信息（API 43）。
func (api *BeeNTAPI) GetGroupInfo(groupID string) (GroupInfo, error) {
	value, err := api.call(43, groupID)
	if err != nil {
		return GroupInfo{}, describeError(43, err)
	}
	fields := splitLines(value)
	return GroupInfo{
		Name:      field(fields, 0),
		MemberNum: atoi(field(fields, 1)),
		MemberMax: atoi(field(fields, 2)),
	}, nil
}

// GetGroupList 获取群列表（API 44）。
func (api *BeeNTAPI) GetGroupList() ([]GroupListItem, error) {
	value, err := api.call(44)
	if err != nil {
		return nil, describeError(44, err)
	}
	var items []GroupListItem
	for _, line := range splitLines(value) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		items = append(items, GroupListItem{
			GroupID:   field(fields, 0),
			Name:      field(fields, 1),
			MemberNum: atoi(field(fields, 2)),
			MemberMax: atoi(field(fields, 3)),
		})
	}
	return items, nil
}

// GetGroupMemberInfo 获取群成员信息（API 45）。
func (api *BeeNTAPI) GetGroupMemberInfo(groupID, qq string) (GroupMemberInfo, error) {
	value, err := api.call(45, groupID, qq)
	if err != nil {
		return GroupMemberInfo{}, describeError(45, err)
	}
	fields := splitLines(value)
	return GroupMemberInfo{
		Nickname:     field(fields, 0),
		Card:         field(fields, 1),
		QQLevel:      atoi(field(fields, 2)),
		Level:        atoi(field(fields, 3)),
		Gender:       atoi(field(fields, 4)),
		Role:         atoi(field(fields, 5)),
		JoinTime:     atoi64(field(fields, 6)),
		LastSentTime: atoi64(field(fields, 7)),
		Title:        field(fields, 8),
	}, nil
}

// GetGroupMemberList 获取群成员列表（API 46）。
func (api *BeeNTAPI) GetGroupMemberList(groupID string) ([]GroupMemberListItem, error) {
	value, err := api.call(46, groupID)
	if err != nil {
		return nil, describeError(46, err)
	}
	var items []GroupMemberListItem
	for _, line := range splitLines(value) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		items = append(items, GroupMemberListItem{
			QQ:           field(fields, 0),
			Nickname:     field(fields, 1),
			Card:         field(fields, 2),
			Level:        atoi(field(fields, 3)),
			JoinTime:     atoi64(field(fields, 4)),
			LastSentTime: atoi64(field(fields, 5)),
			Role:         atoi(field(fields, 6)),
		})
	}
	return items, nil
}

// GetGroupHonorInfo 获取群荣誉信息，返回 JSON 文本，honorType 取 Honor* 常量（API 47）。
func (api *BeeNTAPI) GetGroupHonorInfo(groupID, honorType string) (string, error) {
	return api.text(47, groupID, honorType)
}

// GetCookies 获取指定域名的 Cookie，包含 p_skey、skey 等信息（API 48）。
func (api *BeeNTAPI) GetCookies(domain string) (string, error) { return api.text(48, domain) }

// GetBKN 获取指定域名的 bkn（API 49）。
func (api *BeeNTAPI) GetBKN(domain string) (string, error) { return api.text(49, domain) }

// GetNickname 从缓存中获取昵称，速度快（API 50）。
func (api *BeeNTAPI) GetNickname(qq string) (string, error) { return api.text(50, qq) }

// GetNicknameFresh 不从缓存获取昵称，速度慢但数据更新（API 51）。
func (api *BeeNTAPI) GetNicknameFresh(qq string) (string, error) { return api.text(51, qq) }

// ---------- 群公告与禁言 ----------

// GetGroupNoticeList 获取群公告列表，返回 JSON 文本（API 52）。
func (api *BeeNTAPI) GetGroupNoticeList(groupID string) (string, error) {
	return api.text(52, groupID)
}

// DeleteGroupNotice 删除群公告，noticeID 来自获取群公告的 notice_id（API 53）。
func (api *BeeNTAPI) DeleteGroupNotice(groupID, noticeID string) error {
	return api.ok(53, groupID, noticeID)
}

// GetGroupBanList 获取群禁言列表（API 54）。
func (api *BeeNTAPI) GetGroupBanList(groupID string) ([]GroupBanItem, error) {
	value, err := api.call(54, groupID)
	if err != nil {
		return nil, describeError(54, err)
	}
	var items []GroupBanItem
	for _, line := range splitLines(value) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		items = append(items, GroupBanItem{
			QQ:       field(fields, 0),
			Nickname: field(fields, 1),
			EndTime:  atoi64(field(fields, 2)),
		})
	}
	return items, nil
}

// ---------- 资料设置 ----------

// SetSignature 设置个性签名（API 55）。
func (api *BeeNTAPI) SetSignature(content string) error { return api.ok(55, content) }

// SetAvatar 设置 QQ 头像，支持透明图片，图片为网址或路径（API 56）。
func (api *BeeNTAPI) SetAvatar(image string) error { return api.ok(56, image) }

// GetClientKey 获取 ClientKey（API 57）。
func (api *BeeNTAPI) GetClientKey() (string, error) { return api.text(57) }

// ---------- 被过滤的好友申请 ----------

// GetFilteredFriendRequests 获取被过滤的好友申请列表，count 小于等于 0 时使用框架默认的 10 条（API 58）。
func (api *BeeNTAPI) GetFilteredFriendRequests(count int) ([]FilteredFriendRequest, error) {
	countText := ""
	if count > 0 {
		countText = itoa(count)
	}
	value, err := api.call(58, countText)
	if err != nil {
		return nil, describeError(58, err)
	}
	var items []FilteredFriendRequest
	for _, line := range splitLines(value) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		items = append(items, FilteredFriendRequest{
			QQ:        field(fields, 0),
			Flag:      field(fields, 1),
			Reason:    field(fields, 2),
			Time:      atoi64(field(fields, 3)),
			FilterWhy: field(fields, 4),
			Nickname:  field(fields, 5),
		})
	}
	return items, nil
}

// HandleFilteredFriendRequest 处理被过滤的好友申请（API 59）。
func (api *BeeNTAPI) HandleFilteredFriendRequest(qq, flag string, approve bool) error {
	return api.ok(59, qq, flag, boolText(approve))
}

// ---------- 缓存与资料 ----------

// ClearCache 手动清理框架缓存，cacheType 取 Cache* 常量，groupID 仅对群成员与群管理层缓存有效（API 60）。
func (api *BeeNTAPI) ClearCache(cacheType int, groupID string) error {
	_, err := api.call(60, itoa(cacheType), groupID)
	return describeError(60, err)
}

// SetNickname 设置 QQ 昵称（API 61）。
func (api *BeeNTAPI) SetNickname(nickname string) error { return api.ok(61, nickname) }

// DeleteFriend 删除好友（API 62）。
func (api *BeeNTAPI) DeleteFriend(friendQQ string) error { return api.ok(62, friendQQ) }

// PublishGroupNotice 发布群公告，image 为本地路径，不支持链接，可空（API 63）。
func (api *BeeNTAPI) PublishGroupNotice(groupID, content, image string) error {
	return api.ok(63, groupID, content, image)
}

// ---------- 文件 ----------

// UploadGroupFile 上传群文件，仅支持本地路径，文件名需包含后缀（API 64）。
func (api *BeeNTAPI) UploadGroupFile(groupID, filePath, fileName string) error {
	return api.ok(64, groupID, filePath, fileName)
}

// UploadFriendFile 给好友发送文件，仅支持本地路径，文件名需包含后缀（API 65）。
func (api *BeeNTAPI) UploadFriendFile(friendQQ, filePath, fileName string) error {
	return api.ok(65, friendQQ, filePath, fileName)
}

// SetGroupAvatar 设置群头像，图片为链接或路径（API 66）。
func (api *BeeNTAPI) SetGroupAvatar(groupID, image string) error {
	return api.ok(66, groupID, image)
}

// GetGroupFileList 获取群文件列表，返回 JSON 文本（API 67）。
func (api *BeeNTAPI) GetGroupFileList(groupID string) (string, error) {
	return api.text(67, groupID)
}

// DeleteGroupFile 删除群文件，fileID 为 file_id（API 68）。
func (api *BeeNTAPI) DeleteGroupFile(groupID, fileID string) error {
	return api.ok(68, groupID, fileID)
}

// GetGroupFolderFiles 获取群文件夹内文件列表，folderID 为 folder_id，返回 JSON 文本（API 69）。
func (api *BeeNTAPI) GetGroupFolderFiles(groupID, folderID string) (string, error) {
	return api.text(69, groupID, folderID)
}

// FlashTask 是闪传任务的结果。
type FlashTask struct {
	URL        string `json:"url"`  // 分享链接
	ExpireTime int64  `json:"time"` // 过期时间
}

// CreateFlashTask 创建闪传任务，仅支持本地路径（API 70）。
func (api *BeeNTAPI) CreateFlashTask(filePath, title string) (FlashTask, error) {
	value, err := api.call(70, filePath, title)
	if err != nil {
		return FlashTask{}, describeError(70, err)
	}
	var task FlashTask
	if err := decodeJSON(value, &task); err != nil {
		return FlashTask{}, describeError(70, err)
	}
	return task, nil
}

// ---------- 框架信息 ----------

// SendLike 点赞一次，失败返回原因（API 71）。
func (api *BeeNTAPI) SendLike(qq string) error { return api.ok(71, qq) }

// RestartFramework 立即重启框架，不会调用禁用与卸载函数，属于强制重启（API 72）。
func (api *BeeNTAPI) RestartFramework() error {
	_, err := api.call(72)
	return describeError(72, err)
}

// FrameworkName 获取框架名称（API 73）。
func (api *BeeNTAPI) FrameworkName() (string, error) { return api.text(73) }

// FrameworkVersion 获取框架版本，例如 0.1（API 74）。
func (api *BeeNTAPI) FrameworkVersion() (string, error) { return api.text(74) }

// FrameworkQQ 获取当前机器人 QQ（API 75）。
func (api *BeeNTAPI) FrameworkQQ() (string, error) { return api.text(75) }

// GetGroupAdminList 获取群主和管理员 QQ 列表（API 76）。
func (api *BeeNTAPI) GetGroupAdminList(groupID string) ([]string, error) {
	value, err := api.call(76, groupID)
	if err != nil {
		return nil, describeError(76, err)
	}
	var admins []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			admins = append(admins, line)
		}
	}
	return admins, nil
}

// SetGender 设置 QQ 性别：-1 未知、0 男、1 女（API 77）。
func (api *BeeNTAPI) SetGender(gender int) error { return api.ok(77, itoa(gender)) }

// IsBanned 查询某人是否被禁言（API 78）。
func (api *BeeNTAPI) IsBanned(groupID, qq string) (bool, error) {
	return api.flag(78, groupID, qq)
}

// IsRobot 判断对方是否为官方机器人（API 79）。
func (api *BeeNTAPI) IsRobot(qq string) (bool, error) { return api.flag(79, qq) }

// ---------- AI 语音 ----------

// GetGroupAICharacters 获取群 AI 角色列表，返回 JSON 文本（API 80）。
func (api *BeeNTAPI) GetGroupAICharacters(groupID string) (string, error) {
	return api.text(80, groupID)
}

// SendGroupAIVoice 发送群 AI 语音消息，characterID 来自 GetGroupAICharacters，
// 成功返回语音链接，失败返回原因（API 81）。
func (api *BeeNTAPI) SendGroupAIVoice(groupID, characterID, text string) (string, error) {
	return api.text(81, groupID, characterID, text)
}

// ---------- 引用回复与精华 ----------

// SendFriendReply 发送好友引用回复消息（API 82）。
func (api *BeeNTAPI) SendFriendReply(friendQQ, messageID, content string) (string, error) {
	return api.text(82, friendQQ, messageID, content)
}

// SendGroupReply 发送群引用回复消息（API 83）。
func (api *BeeNTAPI) SendGroupReply(groupID, messageID, content string) (string, error) {
	return api.text(83, groupID, messageID, content)
}

// SetEssenceMessage 设置群精华消息（API 84）。
func (api *BeeNTAPI) SetEssenceMessage(groupID, messageID string) error {
	return api.ok(84, groupID, messageID)
}

// DeleteEssenceMessage 删除群精华消息（API 85）。
func (api *BeeNTAPI) DeleteEssenceMessage(groupID, messageID string) error {
	return api.ok(85, groupID, messageID)
}

// GetEssenceMessageList 获取群精华消息列表，返回 JSON 文本（API 86）。
func (api *BeeNTAPI) GetEssenceMessageList(groupID string) (string, error) {
	return api.text(86, groupID)
}

// KickMembers 批量踢人，qqList 为多个 QQ，reject 为真时拒绝这些人的加群请求（API 87）。
func (api *BeeNTAPI) KickMembers(groupID string, qqList []string, reject bool) error {
	return api.ok(87, groupID, strings.Join(qqList, "\n"), boolText(reject))
}

// ---------- CQ 码 ----------

// TextImage 生成图片 CQ 码，image 为网址或路径（API 88）。
func (api *BeeNTAPI) TextImage(image string) (string, error) { return api.text(88, image) }

// TextVoice 生成语音 CQ 码，voice 为网址或路径（API 89）。
func (api *BeeNTAPI) TextVoice(voice string) (string, error) { return api.text(89, voice) }

// TextVideo 生成视频 CQ 码，video 为网址或路径（API 90）。
func (api *BeeNTAPI) TextVideo(video string) (string, error) { return api.text(90, video) }

// TextReply 生成回复 CQ 码，messageID 为引用的消息 ID（API 91）。
func (api *BeeNTAPI) TextReply(messageID string) (string, error) { return api.text(91, messageID) }

// ---------- JSON 信息 ----------

// GetFriendInfoJSON 获取好友信息，返回 JSON 文本，noCache 为真时不使用缓存（API 92）。
func (api *BeeNTAPI) GetFriendInfoJSON(friendQQ string, noCache bool) (string, error) {
	return api.text(92, friendQQ, boolText(noCache))
}

// GetStrangerInfoJSON 获取陌生人信息，返回 JSON 文本，noCache 为真时不使用缓存（API 93）。
func (api *BeeNTAPI) GetStrangerInfoJSON(qq string, noCache bool) (string, error) {
	return api.text(93, qq, boolText(noCache))
}

// ---------- 表情点赞 ----------

// SetMessageEmojiLike 设置消息表情点赞，set 为真设置、为假取消（API 94）。
func (api *BeeNTAPI) SetMessageEmojiLike(messageID, emojiID string, set bool) error {
	return api.ok(94, messageID, emojiID, boolText(set))
}

// GetEmojiLikeDetail 获取表情点赞详情，返回 JSON 文本（API 95）。
func (api *BeeNTAPI) GetEmojiLikeDetail(messageID, emojiID, emojiType string, count int, cookie string) (string, error) {
	return api.text(95, messageID, emojiID, emojiType, itoa(count), cookie)
}

// GetGroupEmojiLikes 获取群消息表情点赞列表，返回 JSON 文本（API 96）。
func (api *BeeNTAPI) GetGroupEmojiLikes(groupID, messageID, emojiID, emojiType string, count int) (string, error) {
	return api.text(96, groupID, messageID, emojiID, emojiType, itoa(count))
}

// GetVoiceToText 获取语音转文字结果，失败返回原因（API 97）。
func (api *BeeNTAPI) GetVoiceToText(messageID string) (string, error) {
	return api.text(97, messageID)
}

// ---------- 内部辅助 ----------

// text 发起一次框架调用并返回文本结果。
func (api *BeeNTAPI) text(code int, args ...string) (string, error) {
	value, err := api.call(code, args...)
	if err != nil {
		return "", describeError(code, err)
	}
	return value, nil
}

// ok 发起一次设置类框架调用，成功返回空、失败返回原因，这里统一转为 error。
func (api *BeeNTAPI) ok(code int, args ...string) error {
	value, err := api.call(code, args...)
	if err != nil {
		return describeError(code, err)
	}
	if value != "" {
		return &FrameworkError{Code: code, Message: value}
	}
	return nil
}

// flag 发起一次逻辑型框架调用。
func (api *BeeNTAPI) flag(code int, args ...string) (bool, error) {
	value, err := api.call(code, args...)
	if err != nil {
		return false, describeError(code, err)
	}
	return parseBool(value), nil
}

// friendInfo 解析好友或陌生人信息的固定字段布局。
func (api *BeeNTAPI) friendInfo(code int, args ...string) (FriendInfo, error) {
	value, err := api.call(code, args...)
	if err != nil {
		return FriendInfo{}, describeError(code, err)
	}
	fields := splitLines(value)
	return FriendInfo{
		QQ:        field(fields, 0),
		Nickname:  field(fields, 1),
		VIPLevel:  atoi(field(fields, 2)),
		Gender:    atoi(field(fields, 3)),
		QQLevel:   atoi(field(fields, 4)),
		QID:       field(fields, 5),
		Signature: field(fields, 6),
	}, nil
}

// FrameworkError 是框架返回的失败原因。
type FrameworkError struct {
	Code    int
	Message string
}

func (e *FrameworkError) Error() string {
	return "框架返回失败: " + e.Message
}

func itoa(value int) string { return strconv.Itoa(value) }
