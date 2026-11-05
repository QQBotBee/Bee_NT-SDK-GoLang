package main

import "strconv"

// API opcodes follow the BeeNT receive-side dispatch table, not Bee legacy IDs.
const (
	OpGetRobotInfo                = 1
	OpLog                         = 2
	OpSendFriendMessage           = 3
	OpSendGroupMessage            = 4
	OpSendFriendVoice             = 5
	OpSendGroupVoice              = 6
	OpUploadFriendImage           = 7
	OpUploadGroupImage            = 8
	OpSendFriendVideo             = 9
	OpSendGroupVideo              = 10
	OpAt                          = 11
	OpRockPaperScissors           = 12
	OpDice                        = 13
	OpSendGroupFriendCard         = 14
	OpSendFriendFriendCard        = 15
	OpSendGroupGroupCard          = 16
	OpSendFriendGroupCard         = 17
	OpSendFriendCustomMusic       = 18
	OpSendGroupCustomMusic        = 19
	OpSendFriendMusic             = 20
	OpSendGroupMusic              = 21
	OpSendFriendForward           = 22
	OpSendGroupForward            = 23
	OpSendGroupJSON               = 24
	OpSendFriendJSON              = 25
	OpSendGroupXML                = 26
	OpSendFriendXML               = 27
	OpFace                        = 28
	OpSetGroupBan                 = 29
	OpSetGroupWholeBan            = 30
	OpKickGroupMember             = 31
	OpSetGroupAdmin               = 32
	OpSetGroupCard                = 33
	OpSetGroupName                = 34
	OpLeaveGroup                  = 35
	OpSetGroupTitle               = 36
	OpHandleFriendRequest         = 37
	OpHandleGroupRequest          = 38
	OpGetStrangerInfo             = 39
	OpGetFriendList               = 40
	OpGetFriendInfo               = 41
	OpRecallMessage               = 42
	OpGetGroupInfo                = 43
	OpGetGroupList                = 44
	OpGetGroupMemberInfo          = 45
	OpGetGroupMemberList          = 46
	OpGetGroupHonor               = 47
	OpGetCookie                   = 48
	OpGetBKN                      = 49
	OpGetNickname                 = 50
	OpGetNicknameNoCache          = 51
	OpGetGroupNotices             = 52
	OpDeleteGroupNotice           = 53
	OpGetGroupBanList             = 54
	OpSetSignature                = 55
	OpSetAvatar                   = 56
	OpGetClientKey                = 57
	OpGetFilteredFriendRequests   = 58
	OpHandleFilteredFriendRequest = 59
	OpClearCache                  = 60
	OpSetNickname                 = 61
	OpDeleteFriend                = 62
	OpPublishGroupNotice          = 63
	OpUploadGroupFile             = 64
	OpUploadFriendFile            = 65
	OpSetGroupAvatar              = 66
	OpGetGroupFiles               = 67
	OpDeleteGroupFile             = 68
	OpGetGroupFolderFiles         = 69
	OpCreateFlashTransfer         = 70
	OpLike                        = 71
	OpRestartFramework            = 72
	OpGetFrameworkName            = 73
	OpGetFrameworkVersion         = 74
	OpGetFrameworkQQ              = 75
	OpGetGroupAdmins              = 76
	OpSetSex                      = 77
	OpIsGroupMemberBanned         = 78
	OpIsRobot                     = 79
	OpGetGroupAIRoles             = 80
	OpSendGroupAIVoice            = 81
	OpSendFriendReply             = 82
	OpSendGroupReply              = 83
	OpSetGroupEssence             = 84
	OpDeleteGroupEssence          = 85
	OpGetGroupEssences            = 86
	OpKickGroupMembers            = 87
	OpImage                       = 88
	OpVoice                       = 89
	OpVideo                       = 90
	OpReplyCode                   = 91
	OpGetFriendInfoJSON           = 92
	OpGetStrangerInfoJSON         = 93
	OpSetMessageEmojiLike         = 94
	OpGetEmojiLikeDetails         = 95
	OpGetGroupMessageEmojiLikes   = 96
	OpGetVoiceToText              = 97
)

var apiArgumentCounts = [...]int{
	0,
	0, // 1 GetRobotInfo
	1, // 2 Log
	2, // 3 SendFriendMessage
	2, // 4 SendGroupMessage
	2, // 5 SendFriendVoice
	2, // 6 SendGroupVoice
	1, // 7 UploadFriendImage
	1, // 8 UploadGroupImage
	2, // 9 SendFriendVideo
	2, // 10 SendGroupVideo
	1, // 11 At
	0, // 12 RockPaperScissors
	0, // 13 Dice
	2, // 14 SendGroupFriendCard
	2, // 15 SendFriendFriendCard
	2, // 16 SendGroupGroupCard
	2, // 17 SendFriendGroupCard
	7, // 18 SendFriendCustomMusic
	7, // 19 SendGroupCustomMusic
	3, // 20 SendFriendMusic
	3, // 21 SendGroupMusic
	2, // 22 SendFriendForward
	2, // 23 SendGroupForward
	2, // 24 SendGroupJSON
	2, // 25 SendFriendJSON
	2, // 26 SendGroupXML
	2, // 27 SendFriendXML
	1, // 28 Face
	3, // 29 SetGroupBan
	2, // 30 SetGroupWholeBan
	3, // 31 KickGroupMember
	3, // 32 SetGroupAdmin
	3, // 33 SetGroupCard
	2, // 34 SetGroupName
	2, // 35 LeaveGroup
	3, // 36 SetGroupTitle
	4, // 37 HandleFriendRequest
	6, // 38 HandleGroupRequest
	2, // 39 GetStrangerInfo
	0, // 40 GetFriendList
	2, // 41 GetFriendInfo
	1, // 42 RecallMessage
	1, // 43 GetGroupInfo
	0, // 44 GetGroupList
	2, // 45 GetGroupMemberInfo
	1, // 46 GetGroupMemberList
	2, // 47 GetGroupHonor
	1, // 48 GetCookie
	1, // 49 GetBKN
	1, // 50 GetNickname
	1, // 51 GetNicknameNoCache
	1, // 52 GetGroupNotices
	2, // 53 DeleteGroupNotice
	1, // 54 GetGroupBanList
	1, // 55 SetSignature
	1, // 56 SetAvatar
	0, // 57 GetClientKey
	1, // 58 GetFilteredFriendRequests
	3, // 59 HandleFilteredFriendRequest
	2, // 60 ClearCache
	1, // 61 SetNickname
	1, // 62 DeleteFriend
	3, // 63 PublishGroupNotice
	3, // 64 UploadGroupFile
	3, // 65 UploadFriendFile
	2, // 66 SetGroupAvatar
	1, // 67 GetGroupFiles
	2, // 68 DeleteGroupFile
	2, // 69 GetGroupFolderFiles
	2, // 70 CreateFlashTransfer
	1, // 71 Like
	0, // 72 RestartFramework
	0, // 73 GetFrameworkName
	0, // 74 GetFrameworkVersion
	0, // 75 GetFrameworkQQ
	1, // 76 GetGroupAdmins
	1, // 77 SetSex
	2, // 78 IsGroupMemberBanned
	1, // 79 IsRobot
	1, // 80 GetGroupAIRoles
	3, // 81 SendGroupAIVoice
	3, // 82 SendFriendReply
	3, // 83 SendGroupReply
	2, // 84 SetGroupEssence
	2, // 85 DeleteGroupEssence
	1, // 86 GetGroupEssences
	3, // 87 KickGroupMembers
	1, // 88 Image
	1, // 89 Voice
	1, // 90 Video
	1, // 91 ReplyCode
	2, // 92 GetFriendInfoJSON
	2, // 93 GetStrangerInfoJSON
	3, // 94 SetMessageEmojiLike
	5, // 95 GetEmojiLikeDetails
	5, // 96 GetGroupMessageEmojiLikes
	1, // 97 GetVoiceToText
}

// GetRobotInfo 机器人信息 (BeeNT opcode 1).
func (api *BeeNTAPI) GetRobotInfo() (RobotInfo, error) {
	raw, err := api.Call(OpGetRobotInfo)
	if err != nil {
		var zero RobotInfo
		return zero, err
	}
	return parseRobotInfo(raw)
}

// Log 输出日志 (BeeNT opcode 2).
func (api *BeeNTAPI) Log(content string) error {
	_, err := api.Call(OpLog, content)
	return err
}

// SendFriendMessage 发送好友消息 (BeeNT opcode 3).
func (api *BeeNTAPI) SendFriendMessage(friendID string, content string) (string, error) {
	return api.Call(OpSendFriendMessage, friendID, content)
}

// SendGroupMessage 发送群消息 (BeeNT opcode 4).
func (api *BeeNTAPI) SendGroupMessage(groupID string, content string) (string, error) {
	return api.Call(OpSendGroupMessage, groupID, content)
}

// SendFriendVoice 发送好友语音消息 (BeeNT opcode 5).
func (api *BeeNTAPI) SendFriendVoice(friendID string, voice string) (string, error) {
	return api.Call(OpSendFriendVoice, friendID, voice)
}

// SendGroupVoice 发送群语音消息 (BeeNT opcode 6).
func (api *BeeNTAPI) SendGroupVoice(groupID string, voice string) (string, error) {
	return api.Call(OpSendGroupVoice, groupID, voice)
}

// UploadFriendImage 上传好友图片 (BeeNT opcode 7).
func (api *BeeNTAPI) UploadFriendImage(image string) (string, error) {
	return api.Call(OpUploadFriendImage, image)
}

// UploadGroupImage 上传群图片 (BeeNT opcode 8).
func (api *BeeNTAPI) UploadGroupImage(image string) (string, error) {
	return api.Call(OpUploadGroupImage, image)
}

// SendFriendVideo 发送好友视频消息 (BeeNT opcode 9).
func (api *BeeNTAPI) SendFriendVideo(friendID string, video string) (string, error) {
	return api.Call(OpSendFriendVideo, friendID, video)
}

// SendGroupVideo 发送群视频消息 (BeeNT opcode 10).
func (api *BeeNTAPI) SendGroupVideo(groupID string, video string) (string, error) {
	return api.Call(OpSendGroupVideo, groupID, video)
}

// At 文本_艾特 (BeeNT opcode 11).
func (api *BeeNTAPI) At(qq string) (string, error) {
	return api.Call(OpAt, qq)
}

// RockPaperScissors 文本_猜拳魔法表情 (BeeNT opcode 12).
func (api *BeeNTAPI) RockPaperScissors() (string, error) {
	return api.Call(OpRockPaperScissors)
}

// Dice 文本_掷骰子魔法表情 (BeeNT opcode 13).
func (api *BeeNTAPI) Dice() (string, error) {
	return api.Call(OpDice)
}

// SendGroupFriendCard 发送群推荐好友卡片消息 (BeeNT opcode 14).
func (api *BeeNTAPI) SendGroupFriendCard(groupID string, qq string) (string, error) {
	return api.Call(OpSendGroupFriendCard, groupID, qq)
}

// SendFriendFriendCard 发送好友推荐好友卡片消息 (BeeNT opcode 15).
func (api *BeeNTAPI) SendFriendFriendCard(friendID string, qq string) (string, error) {
	return api.Call(OpSendFriendFriendCard, friendID, qq)
}

// SendGroupGroupCard 发送群推荐群卡片消息 (BeeNT opcode 16).
func (api *BeeNTAPI) SendGroupGroupCard(groupID string, recommendedGroupID string) (string, error) {
	return api.Call(OpSendGroupGroupCard, groupID, recommendedGroupID)
}

// SendFriendGroupCard 发送好友推荐群卡片消息 (BeeNT opcode 17).
func (api *BeeNTAPI) SendFriendGroupCard(friendID string, recommendedGroupID string) (string, error) {
	return api.Call(OpSendFriendGroupCard, friendID, recommendedGroupID)
}

// SendFriendCustomMusic 发送好友自定义音乐分享消息 (BeeNT opcode 18).
func (api *BeeNTAPI) SendFriendCustomMusic(friendID string, cover string, platform string, jumpURL string, audioURL string, title string, description string) (string, error) {
	return api.Call(OpSendFriendCustomMusic, friendID, cover, platform, jumpURL, audioURL, title, description)
}

// SendGroupCustomMusic 发送群自定义音乐分享消息 (BeeNT opcode 19).
func (api *BeeNTAPI) SendGroupCustomMusic(groupID string, cover string, platform string, jumpURL string, audioURL string, title string, description string) (string, error) {
	return api.Call(OpSendGroupCustomMusic, groupID, cover, platform, jumpURL, audioURL, title, description)
}

// SendFriendMusic 发送好友音乐分享消息 (BeeNT opcode 20).
func (api *BeeNTAPI) SendFriendMusic(friendID string, platform string, musicID string) (string, error) {
	return api.Call(OpSendFriendMusic, friendID, platform, musicID)
}

// SendGroupMusic 发送群音乐分享消息 (BeeNT opcode 21).
func (api *BeeNTAPI) SendGroupMusic(groupID string, platform string, musicID string) (string, error) {
	return api.Call(OpSendGroupMusic, groupID, platform, musicID)
}

// SendFriendForward 发送好友自定义合并消息 (BeeNT opcode 22).
func (api *BeeNTAPI) SendFriendForward(friendID string, content string) (string, error) {
	return api.Call(OpSendFriendForward, friendID, content)
}

// SendGroupForward 发送群自定义合并消息 (BeeNT opcode 23).
func (api *BeeNTAPI) SendGroupForward(groupID string, content string) (string, error) {
	return api.Call(OpSendGroupForward, groupID, content)
}

// SendGroupJSON 发送群JSON消息 (BeeNT opcode 24).
func (api *BeeNTAPI) SendGroupJSON(groupID string, content string) (string, error) {
	return api.Call(OpSendGroupJSON, groupID, content)
}

// SendFriendJSON 发送好友JSON消息 (BeeNT opcode 25).
func (api *BeeNTAPI) SendFriendJSON(friendID string, content string) (string, error) {
	return api.Call(OpSendFriendJSON, friendID, content)
}

// SendGroupXML 发送群XML消息 (BeeNT opcode 26).
func (api *BeeNTAPI) SendGroupXML(groupID string, content string) (string, error) {
	return api.Call(OpSendGroupXML, groupID, content)
}

// SendFriendXML 发送好友XML消息 (BeeNT opcode 27).
func (api *BeeNTAPI) SendFriendXML(friendID string, content string) (string, error) {
	return api.Call(OpSendFriendXML, friendID, content)
}

// Face 文本_表情 (BeeNT opcode 28).
func (api *BeeNTAPI) Face(faceID string) (string, error) {
	return api.Call(OpFace, faceID)
}

// SetGroupBan 禁言 (BeeNT opcode 29).
func (api *BeeNTAPI) SetGroupBan(groupID string, qq string, seconds int) (string, error) {
	return api.Call(OpSetGroupBan, groupID, qq, strconv.Itoa(seconds))
}

// SetGroupWholeBan 全员禁言 (BeeNT opcode 30).
func (api *BeeNTAPI) SetGroupWholeBan(groupID string, enabled bool) (string, error) {
	return api.Call(OpSetGroupWholeBan, groupID, boolArgument(enabled))
}

// KickGroupMember 踢人 (BeeNT opcode 31).
func (api *BeeNTAPI) KickGroupMember(groupID string, qq string, rejectRequest bool) (string, error) {
	return api.Call(OpKickGroupMember, groupID, qq, boolArgument(rejectRequest))
}

// SetGroupAdmin 设置群管理员 (BeeNT opcode 32).
func (api *BeeNTAPI) SetGroupAdmin(groupID string, qq string, enabled bool) (string, error) {
	return api.Call(OpSetGroupAdmin, groupID, qq, boolArgument(enabled))
}

// SetGroupCard 设置群名片 (BeeNT opcode 33).
func (api *BeeNTAPI) SetGroupCard(groupID string, qq string, card string) (string, error) {
	return api.Call(OpSetGroupCard, groupID, qq, card)
}

// SetGroupName 设置群名称 (BeeNT opcode 34).
func (api *BeeNTAPI) SetGroupName(groupID string, name string) (string, error) {
	return api.Call(OpSetGroupName, groupID, name)
}

// LeaveGroup 退群 (BeeNT opcode 35).
func (api *BeeNTAPI) LeaveGroup(groupID string, dismiss bool) (string, error) {
	return api.Call(OpLeaveGroup, groupID, boolArgument(dismiss))
}

// SetGroupTitle 设置群头衔 (BeeNT opcode 36).
func (api *BeeNTAPI) SetGroupTitle(groupID string, qq string, title string) (string, error) {
	return api.Call(OpSetGroupTitle, groupID, qq, title)
}

// HandleFriendRequest 处理好友请求 (BeeNT opcode 37).
func (api *BeeNTAPI) HandleFriendRequest(qq string, flag string, approve bool, remark string) (string, error) {
	return api.Call(OpHandleFriendRequest, qq, flag, boolArgument(approve), remark)
}

// HandleGroupRequest 处理加群请求 (BeeNT opcode 38).
func (api *BeeNTAPI) HandleGroupRequest(groupID string, qq string, flag string, approve bool, requestType string, reason string) (string, error) {
	return api.Call(OpHandleGroupRequest, groupID, qq, flag, boolArgument(approve), requestType, reason)
}

// GetStrangerInfo 获取陌生人信息 (BeeNT opcode 39).
func (api *BeeNTAPI) GetStrangerInfo(qq string, noCache bool) (FriendInfo, error) {
	raw, err := api.Call(OpGetStrangerInfo, qq, boolArgument(noCache))
	if err != nil {
		var zero FriendInfo
		return zero, err
	}
	return parseFriendInfo(raw)
}

// GetFriendList 获取好友列表 (BeeNT opcode 40).
func (api *BeeNTAPI) GetFriendList() ([]FriendListItem, error) {
	raw, err := api.Call(OpGetFriendList)
	if err != nil {
		var zero []FriendListItem
		return zero, err
	}
	return parseFriendList(raw)
}

// GetFriendInfo 获取好友信息 (BeeNT opcode 41).
func (api *BeeNTAPI) GetFriendInfo(qq string, noCache bool) (FriendInfo, error) {
	raw, err := api.Call(OpGetFriendInfo, qq, boolArgument(noCache))
	if err != nil {
		var zero FriendInfo
		return zero, err
	}
	return parseFriendInfo(raw)
}

// RecallMessage 撤回消息 (BeeNT opcode 42).
func (api *BeeNTAPI) RecallMessage(messageID string) (string, error) {
	return api.Call(OpRecallMessage, messageID)
}

// GetGroupInfo 获取群信息 (BeeNT opcode 43).
func (api *BeeNTAPI) GetGroupInfo(groupID string) (GroupInfo, error) {
	raw, err := api.Call(OpGetGroupInfo, groupID)
	if err != nil {
		var zero GroupInfo
		return zero, err
	}
	return parseGroupInfo(raw)
}

// GetGroupList 获取群列表 (BeeNT opcode 44).
func (api *BeeNTAPI) GetGroupList() ([]GroupListItem, error) {
	raw, err := api.Call(OpGetGroupList)
	if err != nil {
		var zero []GroupListItem
		return zero, err
	}
	return parseGroupList(raw)
}

// GetGroupMemberInfo 获取群成员信息 (BeeNT opcode 45).
func (api *BeeNTAPI) GetGroupMemberInfo(groupID string, qq string) (GroupMemberInfo, error) {
	raw, err := api.Call(OpGetGroupMemberInfo, groupID, qq)
	if err != nil {
		var zero GroupMemberInfo
		return zero, err
	}
	return parseGroupMemberInfo(raw)
}

// GetGroupMemberList 获取群成员列表 (BeeNT opcode 46).
func (api *BeeNTAPI) GetGroupMemberList(groupID string) ([]GroupMemberListItem, error) {
	raw, err := api.Call(OpGetGroupMemberList, groupID)
	if err != nil {
		var zero []GroupMemberListItem
		return zero, err
	}
	return parseGroupMemberList(raw)
}

// GetGroupHonor 获取群荣誉信息 (BeeNT opcode 47).
func (api *BeeNTAPI) GetGroupHonor(groupID string, honorType string) (string, error) {
	return api.Call(OpGetGroupHonor, groupID, honorType)
}

// GetCookie 获取Cookie (BeeNT opcode 48).
func (api *BeeNTAPI) GetCookie(domain string) (string, error) {
	return api.Call(OpGetCookie, domain)
}

// GetBKN 获取bkn (BeeNT opcode 49).
func (api *BeeNTAPI) GetBKN(domain string) (string, error) {
	return api.Call(OpGetBKN, domain)
}

// GetNickname 取昵称 (BeeNT opcode 50).
func (api *BeeNTAPI) GetNickname(qq string) (string, error) {
	return api.Call(OpGetNickname, qq)
}

// GetNicknameNoCache 取昵称_不从缓存 (BeeNT opcode 51).
func (api *BeeNTAPI) GetNicknameNoCache(qq string) (string, error) {
	return api.Call(OpGetNicknameNoCache, qq)
}

// GetGroupNotices 获取群公告列表 (BeeNT opcode 52).
func (api *BeeNTAPI) GetGroupNotices(groupID string) (string, error) {
	return api.Call(OpGetGroupNotices, groupID)
}

// DeleteGroupNotice 删除群公告 (BeeNT opcode 53).
func (api *BeeNTAPI) DeleteGroupNotice(groupID string, noticeID string) (string, error) {
	return api.Call(OpDeleteGroupNotice, groupID, noticeID)
}

// GetGroupBanList 获取群禁言列表 (BeeNT opcode 54).
func (api *BeeNTAPI) GetGroupBanList(groupID string) ([]GroupBanListItem, error) {
	raw, err := api.Call(OpGetGroupBanList, groupID)
	if err != nil {
		var zero []GroupBanListItem
		return zero, err
	}
	return parseGroupBanList(raw)
}

// SetSignature 设置个性签名 (BeeNT opcode 55).
func (api *BeeNTAPI) SetSignature(content string) (string, error) {
	return api.Call(OpSetSignature, content)
}

// SetAvatar 设置头像 (BeeNT opcode 56).
func (api *BeeNTAPI) SetAvatar(image string) (string, error) {
	return api.Call(OpSetAvatar, image)
}

// GetClientKey 获取ClientKey (BeeNT opcode 57).
func (api *BeeNTAPI) GetClientKey() (string, error) {
	return api.Call(OpGetClientKey)
}

// GetFilteredFriendRequests 获取被过滤好友申请列表 (BeeNT opcode 58).
func (api *BeeNTAPI) GetFilteredFriendRequests(count int) ([]FilteredFriendRequest, error) {
	raw, err := api.Call(OpGetFilteredFriendRequests, strconv.Itoa(count))
	if err != nil {
		var zero []FilteredFriendRequest
		return zero, err
	}
	return parseFilteredFriendRequests(raw)
}

// HandleFilteredFriendRequest 处理被过滤好友申请 (BeeNT opcode 59).
func (api *BeeNTAPI) HandleFilteredFriendRequest(qq string, flag string, approve bool) (string, error) {
	return api.Call(OpHandleFilteredFriendRequest, qq, flag, boolArgument(approve))
}

// ClearCache 清理缓存 (BeeNT opcode 60).
func (api *BeeNTAPI) ClearCache(cacheType int, groupID string) error {
	_, err := api.Call(OpClearCache, strconv.Itoa(cacheType), groupID)
	return err
}

// SetNickname 设置昵称 (BeeNT opcode 61).
func (api *BeeNTAPI) SetNickname(nickname string) (string, error) {
	return api.Call(OpSetNickname, nickname)
}

// DeleteFriend 删除好友 (BeeNT opcode 62).
func (api *BeeNTAPI) DeleteFriend(qq string) (string, error) {
	return api.Call(OpDeleteFriend, qq)
}

// PublishGroupNotice 发布群公告 (BeeNT opcode 63).
func (api *BeeNTAPI) PublishGroupNotice(groupID string, content string, image string) (string, error) {
	return api.Call(OpPublishGroupNotice, groupID, content, image)
}

// UploadGroupFile 上传群文件 (BeeNT opcode 64).
func (api *BeeNTAPI) UploadGroupFile(groupID string, filePath string, fileName string) (string, error) {
	return api.Call(OpUploadGroupFile, groupID, filePath, fileName)
}

// UploadFriendFile 上传好友文件 (BeeNT opcode 65).
func (api *BeeNTAPI) UploadFriendFile(friendID string, filePath string, fileName string) (string, error) {
	return api.Call(OpUploadFriendFile, friendID, filePath, fileName)
}

// SetGroupAvatar 设置群头像 (BeeNT opcode 66).
func (api *BeeNTAPI) SetGroupAvatar(groupID string, image string) (string, error) {
	return api.Call(OpSetGroupAvatar, groupID, image)
}

// GetGroupFiles 获取群文件列表 (BeeNT opcode 67).
func (api *BeeNTAPI) GetGroupFiles(groupID string) (string, error) {
	return api.Call(OpGetGroupFiles, groupID)
}

// DeleteGroupFile 删除群文件 (BeeNT opcode 68).
func (api *BeeNTAPI) DeleteGroupFile(groupID string, fileID string) (string, error) {
	return api.Call(OpDeleteGroupFile, groupID, fileID)
}

// GetGroupFolderFiles 获取群文件夹内文件列表 (BeeNT opcode 69).
func (api *BeeNTAPI) GetGroupFolderFiles(groupID string, folderID string) (string, error) {
	return api.Call(OpGetGroupFolderFiles, groupID, folderID)
}

// CreateFlashTransfer 创建闪传任务 (BeeNT opcode 70).
func (api *BeeNTAPI) CreateFlashTransfer(filePath string, title string) (string, error) {
	return api.Call(OpCreateFlashTransfer, filePath, title)
}

// Like 点赞 (BeeNT opcode 71).
func (api *BeeNTAPI) Like(qq string) (string, error) {
	return api.Call(OpLike, qq)
}

// RestartFramework 重启框架 (BeeNT opcode 72).
func (api *BeeNTAPI) RestartFramework() error {
	_, err := api.Call(OpRestartFramework)
	return err
}

// GetFrameworkName 取框架名称 (BeeNT opcode 73).
func (api *BeeNTAPI) GetFrameworkName() (string, error) {
	return api.Call(OpGetFrameworkName)
}

// GetFrameworkVersion 取框架版本 (BeeNT opcode 74).
func (api *BeeNTAPI) GetFrameworkVersion() (string, error) {
	return api.Call(OpGetFrameworkVersion)
}

// GetFrameworkQQ 取框架QQ (BeeNT opcode 75).
func (api *BeeNTAPI) GetFrameworkQQ() (string, error) {
	return api.Call(OpGetFrameworkQQ)
}

// GetGroupAdmins 获取群管理层列表 (BeeNT opcode 76).
func (api *BeeNTAPI) GetGroupAdmins(groupID string) (string, error) {
	return api.Call(OpGetGroupAdmins, groupID)
}

// SetSex 设置性别 (BeeNT opcode 77).
func (api *BeeNTAPI) SetSex(sex int) (string, error) {
	return api.Call(OpSetSex, strconv.Itoa(sex))
}

// IsGroupMemberBanned 查询某人是否被禁言 (BeeNT opcode 78).
func (api *BeeNTAPI) IsGroupMemberBanned(groupID string, qq string) (bool, error) {
	return api.callBool(OpIsGroupMemberBanned, groupID, qq)
}

// IsRobot 是否为机器人 (BeeNT opcode 79).
func (api *BeeNTAPI) IsRobot(qq string) (bool, error) {
	return api.callBool(OpIsRobot, qq)
}

// GetGroupAIRoles 获取群AI角色列表 (BeeNT opcode 80).
func (api *BeeNTAPI) GetGroupAIRoles(groupID string) (string, error) {
	return api.Call(OpGetGroupAIRoles, groupID)
}

// SendGroupAIVoice 发送群AI语音消息 (BeeNT opcode 81).
func (api *BeeNTAPI) SendGroupAIVoice(groupID string, characterID string, content string) (string, error) {
	return api.Call(OpSendGroupAIVoice, groupID, characterID, content)
}

// SendFriendReply 发送好友引用回复消息 (BeeNT opcode 82).
func (api *BeeNTAPI) SendFriendReply(friendID string, messageID string, content string) (string, error) {
	return api.Call(OpSendFriendReply, friendID, messageID, content)
}

// SendGroupReply 发送群引用回复消息 (BeeNT opcode 83).
func (api *BeeNTAPI) SendGroupReply(groupID string, messageID string, content string) (string, error) {
	return api.Call(OpSendGroupReply, groupID, messageID, content)
}

// SetGroupEssence 设置群精华消息 (BeeNT opcode 84).
func (api *BeeNTAPI) SetGroupEssence(groupID string, messageID string) (string, error) {
	return api.Call(OpSetGroupEssence, groupID, messageID)
}

// DeleteGroupEssence 删除群精华消息 (BeeNT opcode 85).
func (api *BeeNTAPI) DeleteGroupEssence(groupID string, messageID string) (string, error) {
	return api.Call(OpDeleteGroupEssence, groupID, messageID)
}

// GetGroupEssences 获取群精华消息列表 (BeeNT opcode 86).
func (api *BeeNTAPI) GetGroupEssences(groupID string) (string, error) {
	return api.Call(OpGetGroupEssences, groupID)
}

// KickGroupMembers 踢人_批量 (BeeNT opcode 87).
func (api *BeeNTAPI) KickGroupMembers(groupID string, qqList string, rejectRequest bool) (string, error) {
	return api.Call(OpKickGroupMembers, groupID, qqList, boolArgument(rejectRequest))
}

// Image 文本_图片 (BeeNT opcode 88).
func (api *BeeNTAPI) Image(image string) (string, error) {
	return api.Call(OpImage, image)
}

// Voice 文本_语音 (BeeNT opcode 89).
func (api *BeeNTAPI) Voice(voice string) (string, error) {
	return api.Call(OpVoice, voice)
}

// Video 文本_视频 (BeeNT opcode 90).
func (api *BeeNTAPI) Video(video string) (string, error) {
	return api.Call(OpVideo, video)
}

// ReplyCode 文本_回复 (BeeNT opcode 91).
func (api *BeeNTAPI) ReplyCode(messageID string) (string, error) {
	return api.Call(OpReplyCode, messageID)
}

// GetFriendInfoJSON 获取好友信息_JSON (BeeNT opcode 92).
func (api *BeeNTAPI) GetFriendInfoJSON(qq string, noCache bool) (string, error) {
	return api.Call(OpGetFriendInfoJSON, qq, boolArgument(noCache))
}

// GetStrangerInfoJSON 获取陌生人信息_JSON (BeeNT opcode 93).
func (api *BeeNTAPI) GetStrangerInfoJSON(qq string, noCache bool) (string, error) {
	return api.Call(OpGetStrangerInfoJSON, qq, boolArgument(noCache))
}

// SetMessageEmojiLike 设置消息表情点赞 (BeeNT opcode 94).
func (api *BeeNTAPI) SetMessageEmojiLike(messageID string, emojiID string, enabled bool) (string, error) {
	return api.Call(OpSetMessageEmojiLike, messageID, emojiID, boolArgument(enabled))
}

// GetEmojiLikeDetails 获取表情点赞详情 (BeeNT opcode 95).
func (api *BeeNTAPI) GetEmojiLikeDetails(messageID string, emojiID string, emojiType string, count int, cookie string) (string, error) {
	return api.Call(OpGetEmojiLikeDetails, messageID, emojiID, emojiType, strconv.Itoa(count), cookie)
}

// GetGroupMessageEmojiLikes 获取群消息表情点赞列表 (BeeNT opcode 96).
func (api *BeeNTAPI) GetGroupMessageEmojiLikes(groupID string, messageID string, emojiID string, emojiType string, count int) (string, error) {
	return api.Call(OpGetGroupMessageEmojiLikes, groupID, messageID, emojiID, emojiType, strconv.Itoa(count))
}

// GetVoiceToText 获取语音转文字结果 (BeeNT opcode 97).
func (api *BeeNTAPI) GetVoiceToText(messageID string) (string, error) {
	return api.Call(OpGetVoiceToText, messageID)
}

// MessageTarget binds a friend or group ID to an API connection.
type MessageTarget struct {
	api   *BeeNTAPI
	id    string
	group bool
}

func (api *BeeNTAPI) Friend(friendID string) *MessageTarget {
	return &MessageTarget{api: api, id: friendID}
}
func (api *BeeNTAPI) Group(groupID string) *MessageTarget {
	return &MessageTarget{api: api, id: groupID, group: true}
}
func (target *MessageTarget) SendText(content string) (string, error) {
	if target.group {
		return target.api.SendGroupMessage(target.id, content)
	}
	return target.api.SendFriendMessage(target.id, content)
}
func (target *MessageTarget) SendVoice(voice string) (string, error) {
	if target.group {
		return target.api.SendGroupVoice(target.id, voice)
	}
	return target.api.SendFriendVoice(target.id, voice)
}
func (target *MessageTarget) SendVideo(video string) (string, error) {
	if target.group {
		return target.api.SendGroupVideo(target.id, video)
	}
	return target.api.SendFriendVideo(target.id, video)
}
func (target *MessageTarget) Reply(messageID, content string) (string, error) {
	if target.group {
		return target.api.SendGroupReply(target.id, messageID, content)
	}
	return target.api.SendFriendReply(target.id, messageID, content)
}
func (target *MessageTarget) SendJSON(content string) (string, error) {
	if target.group {
		return target.api.SendGroupJSON(target.id, content)
	}
	return target.api.SendFriendJSON(target.id, content)
}
func (target *MessageTarget) SendXML(content string) (string, error) {
	if target.group {
		return target.api.SendGroupXML(target.id, content)
	}
	return target.api.SendFriendXML(target.id, content)
}
func (target *MessageTarget) SendFile(filePath, fileName string) (string, error) {
	if target.group {
		return target.api.UploadGroupFile(target.id, filePath, fileName)
	}
	return target.api.UploadFriendFile(target.id, filePath, fileName)
}
