# BeeNT Go API 参考

依据 `BeeNT框架参考/易语言_插件API接收处理.txt` 的接收端分派实现全部 1–97 操作。与旧 Bee 的编号、参数和上下文协议不同。

```go
api, err := NewBeeNTAPI()
if err != nil { return }
_, err = api.Group("群号").SendText("你好")
_, err = api.Friend("QQ").Reply("消息ID", "引用回复")
_ = api.Log("日志内容")
```

`APITransport.Call([]byte)` 的输入和返回均为 UTF-8，由 C 壳负责 CP936 转换。命令为 `opcode#|&BEExNT&|#index#|&BEExNT&|#arg...`，无参数操作只带 opcode 和 index。每次调用都会检查操作码、参数数量、UTF-8、NUL 和保留分隔符；空字段与普通换行保留。不可用 GBK 表示的 Unicode 字符编码为 UTF-16 `\\uXXXX`（支持代理对）。

`Call(opcode int, args ...string) (string, error)` 保留原始返回文本；错误表示参数或 IPC 故障，框架业务失败仍可能是结果字符串。发送消息成功通常返回消息 ID；修改类 API 成功通常返回空字符串、失败返回原因。原始 JSON 不在解码前替换 Unicode 转义；使用 `encoding/json`。普通文本如需显示框架转义，可调用 `DecodeFrameworkText`。`Log`、`ClearCache`、`RestartFramework` 仅返回传输错误。

`NewBeeNTAPIWithTransport(index int32, transport APITransport)` 可用于自定义传输或测试；索引不可为负。`NewBeeNTAPI()` 使用 Worker 安装的索引和连接。

## 方法表

所有 ID、QQ、群号均为 `string`；布尔参数为 `bool`（线上 1/0）。字符串方法返回 `(string,error)`；下表标注的类型化方法返回 `(类型,error)`。

| 编号 | Go 方法与参数 | 原 SDK 名称 | 返回 |
| --- | --- | --- | --- |
| 1 | `GetRobotInfo()` | 机器人信息 | `RobotInfo` |
| 2 | `Log(content string)` | 输出日志 | `error` |
| 3 | `SendFriendMessage(friendID string, content string)` | 发送好友消息 | `string` |
| 4 | `SendGroupMessage(groupID string, content string)` | 发送群消息 | `string` |
| 5 | `SendFriendVoice(friendID string, voice string)` | 发送好友语音消息 | `string` |
| 6 | `SendGroupVoice(groupID string, voice string)` | 发送群语音消息 | `string` |
| 7 | `UploadFriendImage(image string)` | 上传好友图片 | `string` |
| 8 | `UploadGroupImage(image string)` | 上传群图片 | `string` |
| 9 | `SendFriendVideo(friendID string, video string)` | 发送好友视频消息 | `string` |
| 10 | `SendGroupVideo(groupID string, video string)` | 发送群视频消息 | `string` |
| 11 | `At(qq string)` | 文本_艾特 | `string` |
| 12 | `RockPaperScissors()` | 文本_猜拳魔法表情 | `string` |
| 13 | `Dice()` | 文本_掷骰子魔法表情 | `string` |
| 14 | `SendGroupFriendCard(groupID string, qq string)` | 发送群推荐好友卡片消息 | `string` |
| 15 | `SendFriendFriendCard(friendID string, qq string)` | 发送好友推荐好友卡片消息 | `string` |
| 16 | `SendGroupGroupCard(groupID string, recommendedGroupID string)` | 发送群推荐群卡片消息 | `string` |
| 17 | `SendFriendGroupCard(friendID string, recommendedGroupID string)` | 发送好友推荐群卡片消息 | `string` |
| 18 | `SendFriendCustomMusic(friendID string, cover string, platform string, jumpURL string, audioURL string, title string, description string)` | 发送好友自定义音乐分享消息 | `string` |
| 19 | `SendGroupCustomMusic(groupID string, cover string, platform string, jumpURL string, audioURL string, title string, description string)` | 发送群自定义音乐分享消息 | `string` |
| 20 | `SendFriendMusic(friendID string, platform string, musicID string)` | 发送好友音乐分享消息 | `string` |
| 21 | `SendGroupMusic(groupID string, platform string, musicID string)` | 发送群音乐分享消息 | `string` |
| 22 | `SendFriendForward(friendID string, content string)` | 发送好友自定义合并消息 | `string` |
| 23 | `SendGroupForward(groupID string, content string)` | 发送群自定义合并消息 | `string` |
| 24 | `SendGroupJSON(groupID string, content string)` | 发送群JSON消息 | `string` |
| 25 | `SendFriendJSON(friendID string, content string)` | 发送好友JSON消息 | `string` |
| 26 | `SendGroupXML(groupID string, content string)` | 发送群XML消息 | `string` |
| 27 | `SendFriendXML(friendID string, content string)` | 发送好友XML消息 | `string` |
| 28 | `Face(faceID string)` | 文本_表情 | `string` |
| 29 | `SetGroupBan(groupID string, qq string, seconds int)` | 禁言 | `string` |
| 30 | `SetGroupWholeBan(groupID string, enabled bool)` | 全员禁言 | `string` |
| 31 | `KickGroupMember(groupID string, qq string, rejectRequest bool)` | 踢人 | `string` |
| 32 | `SetGroupAdmin(groupID string, qq string, enabled bool)` | 设置群管理员 | `string` |
| 33 | `SetGroupCard(groupID string, qq string, card string)` | 设置群名片 | `string` |
| 34 | `SetGroupName(groupID string, name string)` | 设置群名称 | `string` |
| 35 | `LeaveGroup(groupID string, dismiss bool)` | 退群 | `string` |
| 36 | `SetGroupTitle(groupID string, qq string, title string)` | 设置群头衔 | `string` |
| 37 | `HandleFriendRequest(qq string, flag string, approve bool, remark string)` | 处理好友请求 | `string` |
| 38 | `HandleGroupRequest(groupID string, qq string, flag string, approve bool, requestType string, reason string)` | 处理加群请求 | `string` |
| 39 | `GetStrangerInfo(qq string, noCache bool)` | 获取陌生人信息 | `FriendInfo` |
| 40 | `GetFriendList()` | 获取好友列表 | `[]FriendListItem` |
| 41 | `GetFriendInfo(qq string, noCache bool)` | 获取好友信息 | `FriendInfo` |
| 42 | `RecallMessage(messageID string)` | 撤回消息 | `string` |
| 43 | `GetGroupInfo(groupID string)` | 获取群信息 | `GroupInfo` |
| 44 | `GetGroupList()` | 获取群列表 | `[]GroupListItem` |
| 45 | `GetGroupMemberInfo(groupID string, qq string)` | 获取群成员信息 | `GroupMemberInfo` |
| 46 | `GetGroupMemberList(groupID string)` | 获取群成员列表 | `[]GroupMemberListItem` |
| 47 | `GetGroupHonor(groupID string, honorType string)` | 获取群荣誉信息 | `string` |
| 48 | `GetCookie(domain string)` | 获取Cookie | `string` |
| 49 | `GetBKN(domain string)` | 获取bkn | `string` |
| 50 | `GetNickname(qq string)` | 取昵称 | `string` |
| 51 | `GetNicknameNoCache(qq string)` | 取昵称_不从缓存 | `string` |
| 52 | `GetGroupNotices(groupID string)` | 获取群公告列表 | `string` |
| 53 | `DeleteGroupNotice(groupID string, noticeID string)` | 删除群公告 | `string` |
| 54 | `GetGroupBanList(groupID string)` | 获取群禁言列表 | `[]GroupBanListItem` |
| 55 | `SetSignature(content string)` | 设置个性签名 | `string` |
| 56 | `SetAvatar(image string)` | 设置头像 | `string` |
| 57 | `GetClientKey()` | 获取ClientKey | `string` |
| 58 | `GetFilteredFriendRequests(count int)` | 获取被过滤好友申请列表 | `[]FilteredFriendRequest` |
| 59 | `HandleFilteredFriendRequest(qq string, flag string, approve bool)` | 处理被过滤好友申请 | `string` |
| 60 | `ClearCache(cacheType int, groupID string)` | 清理缓存 | `error` |
| 61 | `SetNickname(nickname string)` | 设置昵称 | `string` |
| 62 | `DeleteFriend(qq string)` | 删除好友 | `string` |
| 63 | `PublishGroupNotice(groupID string, content string, image string)` | 发布群公告 | `string` |
| 64 | `UploadGroupFile(groupID string, filePath string, fileName string)` | 上传群文件 | `string` |
| 65 | `UploadFriendFile(friendID string, filePath string, fileName string)` | 上传好友文件 | `string` |
| 66 | `SetGroupAvatar(groupID string, image string)` | 设置群头像 | `string` |
| 67 | `GetGroupFiles(groupID string)` | 获取群文件列表 | `string` |
| 68 | `DeleteGroupFile(groupID string, fileID string)` | 删除群文件 | `string` |
| 69 | `GetGroupFolderFiles(groupID string, folderID string)` | 获取群文件夹内文件列表 | `string` |
| 70 | `CreateFlashTransfer(filePath string, title string)` | 创建闪传任务 | `string` |
| 71 | `Like(qq string)` | 点赞 | `string` |
| 72 | `RestartFramework()` | 重启框架 | `error` |
| 73 | `GetFrameworkName()` | 取框架名称 | `string` |
| 74 | `GetFrameworkVersion()` | 取框架版本 | `string` |
| 75 | `GetFrameworkQQ()` | 取框架QQ | `string` |
| 76 | `GetGroupAdmins(groupID string)` | 获取群管理层列表 | `string` |
| 77 | `SetSex(sex int)` | 设置性别 | `string` |
| 78 | `IsGroupMemberBanned(groupID string, qq string)` | 查询某人是否被禁言 | `bool` |
| 79 | `IsRobot(qq string)` | 是否为机器人 | `bool` |
| 80 | `GetGroupAIRoles(groupID string)` | 获取群AI角色列表 | `string` |
| 81 | `SendGroupAIVoice(groupID string, characterID string, content string)` | 发送群AI语音消息 | `string` |
| 82 | `SendFriendReply(friendID string, messageID string, content string)` | 发送好友引用回复消息 | `string` |
| 83 | `SendGroupReply(groupID string, messageID string, content string)` | 发送群引用回复消息 | `string` |
| 84 | `SetGroupEssence(groupID string, messageID string)` | 设置群精华消息 | `string` |
| 85 | `DeleteGroupEssence(groupID string, messageID string)` | 删除群精华消息 | `string` |
| 86 | `GetGroupEssences(groupID string)` | 获取群精华消息列表 | `string` |
| 87 | `KickGroupMembers(groupID string, qqList string, rejectRequest bool)` | 踢人_批量 | `string` |
| 88 | `Image(image string)` | 文本_图片 | `string` |
| 89 | `Voice(voice string)` | 文本_语音 | `string` |
| 90 | `Video(video string)` | 文本_视频 | `string` |
| 91 | `ReplyCode(messageID string)` | 文本_回复 | `string` |
| 92 | `GetFriendInfoJSON(qq string, noCache bool)` | 获取好友信息_JSON | `string` |
| 93 | `GetStrangerInfoJSON(qq string, noCache bool)` | 获取陌生人信息_JSON | `string` |
| 94 | `SetMessageEmojiLike(messageID string, emojiID string, enabled bool)` | 设置消息表情点赞 | `string` |
| 95 | `GetEmojiLikeDetails(messageID string, emojiID string, emojiType string, count int, cookie string)` | 获取表情点赞详情 | `string` |
| 96 | `GetGroupMessageEmojiLikes(groupID string, messageID string, emojiID string, emojiType string, count int)` | 获取群消息表情点赞列表 | `string` |
| 97 | `GetVoiceToText(messageID string)` | 获取语音转文字结果 | `string` |

## 数据与列表

`RobotInfo` 含 `UserID/Nickname`；支持 JSON user_id 为数值或字符串，避免浮点丢失 ID。`FriendInfo` 含 QQ、Nickname、VIPLevel、Sex、QQLevel、QID、Signature。`GroupInfo` 含 Name、MemberCount、MaxMemberCount。

`GroupMemberInfo` 含 Nickname、Card、QQLevel、GroupLevel、Sex、Role、JoinTime、LastSentTime、Title。时间戳为 int64；性别为 -1未知/0男/1女，身份为0成员/1管理员/2群主。

列表项为 `FriendListItem`、`GroupListItem`、`GroupMemberListItem`、`GroupBanListItem`、`FilteredFriendRequest`。线上列表按 `#|&BEExNT&|#` 分行，以 `/c.BeExNTs.\\` 分字段；SDK 检查字段数、整数和时间戳，允许末尾分隔符，异常内容返回错误。查询结果的错误文本不会被当作空结构吞掉。

## 参考勘误

- 45：正确解析第一个字段为昵称，修复易语言样例误赋 QQLevel。
- 96：接收端需要 groupID/messageID/emojiID/emojiType/count；补回遗漏群号。
- 97：只发送 messageID，修复样例重复参数。
- 94：按内部实现保留文本返回语义（成功空串、失败原因），没有按错误注释转换成 bool。

## 事件、缓存与数据目录

31 个事件值见 `types.go` 的 `Event...`、`GroupRequestApply`、`GroupRequestInvite` 常量，都是字符串。缓存类型使用 `CacheAll`、`CacheFriendList`、`CacheGroupList`、`CacheGroupMemberList`、`CacheGroupAdminList`（0–4）。

`Friend(id)` / `Group(id)` 提供 `SendText`、`SendVoice`、`SendVideo`、`Reply`、`SendJSON`、`SendXML`、`SendFile`。引用必须显式提供本次消息 ID。

`GetAppDataDir()` / `api.GetAppDataDir()` 返回 Worker 所在目录，即 C 壳创建的 `<框架根目录>/plugin_data/<插件名>`。将持久化配置存入这里。框架卸载插件可能删除这个目录。
