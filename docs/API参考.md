# BeeNT Go SDK API 参考

## 调用方式

框架 API 只能在插件回调执行期间调用。回调里先取 API 对象，再调用方法：

```go
api, err := requireAPI()
if err != nil {
	return MessageIgnore
}

id, err := api.SendGroupMessage(groupID, "大家好")
```

回调之外（例如回调返回后仍在运行的 goroutine）调用会返回 `ErrOutsideCallback`，不会挂起。

## 返回约定

- 发送类接口成功返回消息 ID，失败返回 `*FrameworkError`（带框架给出的原因）。
- 设置类接口成功返回 `nil`，失败返回 `*FrameworkError`。
- 查询类接口按下表的返回类型解析；列表类返回切片，JSON 类返回原始 JSON 文本。
- 方法编号与易语言 SDK 的框架调用编号一一对应。

## 与易语言 SDK 不同的两处

- `GetGroupEmojiLikes`（API 96）第一个参数是群号。易语言 SDK 把消息 ID 放在第一位，框架接收端读的是群号。
- `GetVoiceToText`（API 97）只传消息 ID。易语言 SDK 传了两遍，框架只用第一个。

`GetFilteredFriendRequests` 的数量传 0 或负数时不传数量，使用框架默认的 10 条。

## 数据目录

`api.GetAppDataDir()` 返回 `框架目录\plugin_data\插件名\`，末尾带反斜杠。配置、数据库、缓存和日志都写这里，不要写到 DLL 旁边或 `temp_plugin`。

## 方法清单


### 基础

| API | 方法 | 说明 |
| --- | --- | --- |
| 1 | `RobotInfo() (RobotInfo, error)` | 获取机器人信息，成功返回 user_id 与 nickname |
| 2 | `Log(content string) error` | 输出插件日志 |

### 发送消息

| API | 方法 | 说明 |
| --- | --- | --- |
| 3 | `SendFriendMessage(friendQQ, content string) (string, error)` | 发送好友消息，返回消息 ID，失败返回原因 |
| 4 | `SendGroupMessage(groupID, content string) (string, error)` | 发送群消息，返回消息 ID，失败返回原因 |
| 5 | `SendFriendVoice(friendQQ, voice string) (string, error)` | 发送好友语音消息，语音为网址或路径，无需转码 |
| 6 | `SendGroupVoice(groupID, voice string) (string, error)` | 发送群语音消息，语音为网址或路径，无需转码 |
| 7 | `UploadFriendImage(image string) (string, error)` | 上传好友图片，返回图片文本代码，配合发送消息使用 |
| 8 | `UploadGroupImage(image string) (string, error)` | 上传群图片，返回图片文本代码，配合发送消息使用 |
| 9 | `SendFriendVideo(friendQQ, video string) (string, error)` | 发送好友视频消息，视频为网址或路径 |
| 10 | `SendGroupVideo(groupID, video string) (string, error)` | 发送群视频消息，视频为网址或路径 |

### 文本代码

| API | 方法 | 说明 |
| --- | --- | --- |
| 11 | `TextAt(qq string) (string, error)` | 生成艾特文本代码，qq 为 all 时代表全体成员 |
| 12 | `TextRPS() (string, error)` | 生成猜拳魔法表情文本代码 |
| 13 | `TextDice() (string, error)` | 生成掷骰子魔法表情文本代码 |

### 卡片与分享

| API | 方法 | 说明 |
| --- | --- | --- |
| 14 | `SendGroupFriendCard(groupID, qq string) (string, error)` | 发送群推荐好友卡片消息 |
| 15 | `SendFriendFriendCard(friendQQ, qq string) (string, error)` | 发送好友推荐好友卡片消息 |
| 16 | `SendGroupGroupCard(groupID, recommendedGroupID string) (string, error)` | 发送群推荐群卡片消息 |
| 17 | `SendFriendGroupCard(friendQQ, recommendedGroupID string) (string, error)` | 发送好友推荐群卡片消息 |
| 18 | `SendFriendCustomMusic(friendQQ string, music CustomMusic) (string, error)` | 发送好友自定义音乐分享消息 |
| 19 | `SendGroupCustomMusic(groupID string, music CustomMusic) (string, error)` | 发送群自定义音乐分享消息 |
| 20 | `SendFriendMusic(friendQQ, platform, musicID string) (string, error)` | 发送好友音乐分享消息，平台类型为 qq/163/kugou/migu/kuwo |
| 21 | `SendGroupMusic(groupID, platform, musicID string) (string, error)` | 发送群音乐分享消息，平台类型为 qq/163/kugou/migu/kuwo |

### 合并、JSON、XML 消息

| API | 方法 | 说明 |
| --- | --- | --- |
| 22 | `SendFriendForward(friendQQ string, nodes []ForwardNode) (string, error)` | 发送好友自定义合并消息。 节点格式为 QQ\|\|昵称\|\|消息内容，消息内容中的换行会自动转成 \n |
| 23 | `SendGroupForward(groupID string, nodes []ForwardNode) (string, error)` | 发送群自定义合并消息 |
| 24 | `SendGroupJSON(groupID, jsonContent string) (string, error)` | 发送群 JSON 消息 |
| 25 | `SendFriendJSON(friendQQ, jsonContent string) (string, error)` | 发送好友 JSON 消息 |
| 26 | `SendGroupXML(groupID, xmlContent string) (string, error)` | 发送群 XML 消息 |
| 27 | `SendFriendXML(friendQQ, xmlContent string) (string, error)` | 发送好友 XML 消息 |
| 28 | `TextFace(faceID string) (string, error)` | 生成表情文本代码，表情 ID 参考表情 CQ 码 ID 表 |

### 群管理

| API | 方法 | 说明 |
| --- | --- | --- |
| 29 | `Ban(groupID, qq string, duration int) error` | 禁言群成员，duration 单位为秒，0 为取消禁言 |
| 30 | `WholeBan(groupID string, enable bool) error` | 设置全员禁言，enable 为真开启、为假关闭 |
| 31 | `Kick(groupID, qq string, reject bool) error` | 踢出群成员，reject 为真时拒绝此人的加群请求 |
| 32 | `SetGroupAdmin(groupID, qq string, enable bool) error` | 设置或取消群管理员，enable 为真设置、为假取消 |
| 33 | `SetGroupCard(groupID, qq, card string) error` | 设置群名片，card 为空时取消群名片 |
| 34 | `SetGroupName(groupID, name string) error` | 设置群名称 |
| 35 | `LeaveGroup(groupID string, dismiss bool) error` | 退群，dismiss 仅在退出人是群主时有效，为真时解散群 |
| 36 | `SetGroupTitle(groupID, qq, title string) error` | 设置群头衔，title 为空时删除群头衔 |

### 请求处理

| API | 方法 | 说明 |
| --- | --- | --- |
| 37 | `HandleFriendRequest(qq, flag string, approve bool, remark string) error` | 处理好友请求，remark 仅在同意时有效，留空不添加备注 |
| 38 | `HandleGroupRequest(groupID, qq, flag string, approve bool, requestType, reason string) error` | 处理加群请求，requestType 取 GroupRequestApply 或 GroupRequestInvite |

### 信息查询

| API | 方法 | 说明 |
| --- | --- | --- |
| 39 | `GetStrangerInfo(qq string, noCache bool) (FriendInfo, error)` | 获取陌生人信息，noCache 为真时不使用缓存 |
| 40 | `GetFriendList() ([]FriendListItem, error)` | 获取好友列表 |
| 41 | `GetFriendInfo(friendQQ string, noCache bool) (FriendInfo, error)` | 获取好友信息，noCache 为真时不使用缓存 |
| 42 | `RecallMessage(messageID string) error` | 撤回消息，成功返回空，失败返回原因 |
| 43 | `GetGroupInfo(groupID string) (GroupInfo, error)` | 获取群信息 |
| 44 | `GetGroupList() ([]GroupListItem, error)` | 获取群列表 |
| 45 | `GetGroupMemberInfo(groupID, qq string) (GroupMemberInfo, error)` | 获取群成员信息 |
| 46 | `GetGroupMemberList(groupID string) ([]GroupMemberListItem, error)` | 获取群成员列表 |
| 47 | `GetGroupHonorInfo(groupID, honorType string) (string, error)` | 获取群荣誉信息，返回 JSON 文本，honorType 取 Honor* 常量 |
| 48 | `GetCookies(domain string) (string, error)` | 获取指定域名的 Cookie，包含 p_skey、skey 等信息 |
| 49 | `GetBKN(domain string) (string, error)` | 获取指定域名的 bkn |
| 50 | `GetNickname(qq string) (string, error)` | 从缓存中获取昵称，速度快 |
| 51 | `GetNicknameFresh(qq string) (string, error)` | 不从缓存获取昵称，速度慢但数据更新 |

### 群公告与禁言

| API | 方法 | 说明 |
| --- | --- | --- |
| 52 | `GetGroupNoticeList(groupID string) (string, error)` | 获取群公告列表，返回 JSON 文本 |
| 53 | `DeleteGroupNotice(groupID, noticeID string) error` | 删除群公告，noticeID 来自获取群公告的 notice_id |
| 54 | `GetGroupBanList(groupID string) ([]GroupBanItem, error)` | 获取群禁言列表 |

### 资料设置

| API | 方法 | 说明 |
| --- | --- | --- |
| 55 | `SetSignature(content string) error` | 设置个性签名 |
| 56 | `SetAvatar(image string) error` | 设置 QQ 头像，支持透明图片，图片为网址或路径 |
| 57 | `GetClientKey() (string, error)` | 获取 ClientKey |

### 被过滤的好友申请

| API | 方法 | 说明 |
| --- | --- | --- |
| 58 | `GetFilteredFriendRequests(count int) ([]FilteredFriendRequest, error)` | 获取被过滤的好友申请列表，count 小于等于 0 时使用框架默认的 10 条 |
| 59 | `HandleFilteredFriendRequest(qq, flag string, approve bool) error` | 处理被过滤的好友申请 |

### 缓存与资料

| API | 方法 | 说明 |
| --- | --- | --- |
| 60 | `ClearCache(cacheType int, groupID string) error` | 手动清理框架缓存，cacheType 取 Cache* 常量，groupID 仅对群成员与群管理层缓存有效 |
| 61 | `SetNickname(nickname string) error` | 设置 QQ 昵称 |
| 62 | `DeleteFriend(friendQQ string) error` | 删除好友 |
| 63 | `PublishGroupNotice(groupID, content, image string) error` | 发布群公告，image 为本地路径，不支持链接，可空 |

### 文件

| API | 方法 | 说明 |
| --- | --- | --- |
| 64 | `UploadGroupFile(groupID, filePath, fileName string) error` | 上传群文件，仅支持本地路径，文件名需包含后缀 |
| 65 | `UploadFriendFile(friendQQ, filePath, fileName string) error` | 给好友发送文件，仅支持本地路径，文件名需包含后缀 |
| 66 | `SetGroupAvatar(groupID, image string) error` | 设置群头像，图片为链接或路径 |
| 67 | `GetGroupFileList(groupID string) (string, error)` | 获取群文件列表，返回 JSON 文本 |
| 68 | `DeleteGroupFile(groupID, fileID string) error` | 删除群文件，fileID 为 file_id |
| 69 | `GetGroupFolderFiles(groupID, folderID string) (string, error)` | 获取群文件夹内文件列表，folderID 为 folder_id，返回 JSON 文本 |
| 70 | `CreateFlashTask(filePath, title string) (FlashTask, error)` | 创建闪传任务，仅支持本地路径 |

### 框架信息

| API | 方法 | 说明 |
| --- | --- | --- |
| 71 | `SendLike(qq string) error` | 点赞一次，失败返回原因 |
| 72 | `RestartFramework() error` | 立即重启框架，不会调用禁用与卸载函数，属于强制重启 |
| 73 | `FrameworkName() (string, error)` | 获取框架名称 |
| 74 | `FrameworkVersion() (string, error)` | 获取框架版本，例如 0.1 |
| 75 | `FrameworkQQ() (string, error)` | 获取当前机器人 QQ |
| 76 | `GetGroupAdminList(groupID string) ([]string, error)` | 获取群主和管理员 QQ 列表 |
| 77 | `SetGender(gender int) error` | 设置 QQ 性别：-1 未知、0 男、1 女 |
| 78 | `IsBanned(groupID, qq string) (bool, error)` | 查询某人是否被禁言 |
| 79 | `IsRobot(qq string) (bool, error)` | 判断对方是否为官方机器人 |

### AI 语音

| API | 方法 | 说明 |
| --- | --- | --- |
| 80 | `GetGroupAICharacters(groupID string) (string, error)` | 获取群 AI 角色列表，返回 JSON 文本 |
| 81 | `SendGroupAIVoice(groupID, characterID, text string) (string, error)` | 发送群 AI 语音消息，characterID 来自 GetGroupAICharacters， 成功返回语音链接，失败返回原因 |

### 引用回复与精华

| API | 方法 | 说明 |
| --- | --- | --- |
| 82 | `SendFriendReply(friendQQ, messageID, content string) (string, error)` | 发送好友引用回复消息 |
| 83 | `SendGroupReply(groupID, messageID, content string) (string, error)` | 发送群引用回复消息 |
| 84 | `SetEssenceMessage(groupID, messageID string) error` | 设置群精华消息 |
| 85 | `DeleteEssenceMessage(groupID, messageID string) error` | 删除群精华消息 |
| 86 | `GetEssenceMessageList(groupID string) (string, error)` | 获取群精华消息列表，返回 JSON 文本 |
| 87 | `KickMembers(groupID string, qqList []string, reject bool) error` | 批量踢人，qqList 为多个 QQ，reject 为真时拒绝这些人的加群请求 |

### CQ 码

| API | 方法 | 说明 |
| --- | --- | --- |
| 88 | `TextImage(image string) (string, error)` | 生成图片 CQ 码，image 为网址或路径 |
| 89 | `TextVoice(voice string) (string, error)` | 生成语音 CQ 码，voice 为网址或路径 |
| 90 | `TextVideo(video string) (string, error)` | 生成视频 CQ 码，video 为网址或路径 |
| 91 | `TextReply(messageID string) (string, error)` | 生成回复 CQ 码，messageID 为引用的消息 ID |

### JSON 信息

| API | 方法 | 说明 |
| --- | --- | --- |
| 92 | `GetFriendInfoJSON(friendQQ string, noCache bool) (string, error)` | 获取好友信息，返回 JSON 文本，noCache 为真时不使用缓存 |
| 93 | `GetStrangerInfoJSON(qq string, noCache bool) (string, error)` | 获取陌生人信息，返回 JSON 文本，noCache 为真时不使用缓存 |

### 表情点赞

| API | 方法 | 说明 |
| --- | --- | --- |
| 94 | `SetMessageEmojiLike(messageID, emojiID string, set bool) error` | 设置消息表情点赞，set 为真设置、为假取消 |
| 95 | `GetEmojiLikeDetail(messageID, emojiID, emojiType string, count int, cookie string) (string, error)` | 获取表情点赞详情，返回 JSON 文本 |
| 96 | `GetGroupEmojiLikes(groupID, messageID, emojiID, emojiType string, count int) (string, error)` | 获取群消息表情点赞列表，返回 JSON 文本 |
| 97 | `GetVoiceToText(messageID string) (string, error)` | 获取语音转文字结果，失败返回原因 |
