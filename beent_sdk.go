package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// 框架协议分隔符，与易语言 SDK 常量表一致。
// 框架换行符分隔一次框架调用的参数；框架分隔符分隔列表返回中的一行字段。
const (
	SeparatorNewline = "#|&BEExNT&|#"
	SeparatorField   = "/c.BeExNTs.\\"
)

// 消息处理结果，对应 BeeNT_消息处理_忽略 / BeeNT_消息处理_拦截。
const (
	MessageIgnore    = 0 // 忽略消息，其他插件仍可处理
	MessageContinue  = MessageIgnore
	MessageIntercept = 1 // 拦截消息，其他插件无法接收到
)

// 加群验证类型，对应 BeeNT_群验证_申请 / BeeNT_群验证_邀请。
const (
	GroupRequestApply  = "26" // 有加群申请
	GroupRequestInvite = "27" // 我被邀请加群
)

// 缓存类型，对应 BeeNT_缓存_*，供 ClearCache 使用。
const (
	CacheAll            = 0 // 全部
	CacheFriendList     = 1 // 好友列表
	CacheGroupList      = 2 // 群列表
	CacheGroupMember    = 3 // 群成员列表
	CacheGroupAdminList = 4 // 群管理员层列表
)

// 性别：0 男，1 女，-1 未知。
const (
	GenderMale    = 0
	GenderFemale  = 1
	GenderUnknown = -1
)

// 群身份：0 普通成员，1 管理，2 群主。
const (
	RoleMember = 0
	RoleAdmin  = 1
	RoleOwner  = 2
)

// 群荣誉信息类型，供 GetGroupHonorInfo 使用。
const (
	HonorAll           = "0" // 全部
	HonorCurrentDragon = "1" // 当前龙王
	HonorHistoryDragon = "2" // 历史龙王
	HonorFire          = "3" // 群聊之火
	HonorFirework      = "4" // 群聊焰火
	HonorStrongNewbie  = "5" // 冒尖小春笋
	HonorHappySource   = "6" // 快乐之源
)

// EventType 是事件消息的事件类型，值为框架传入的字符串编号。
type EventType string

// 事件类型，对应易语言 SDK 常量表中的 BeeNT_* 事件常量。
const (
	EventGroupFileUpload        EventType = "1"  // 有人上传文件
	EventGroupAdminAdd          EventType = "2"  // 群管理员变动：添加
	EventGroupAdminRemove       EventType = "3"  // 群管理员变动：取消
	EventGroupMemberLeave       EventType = "4"  // 有人退群
	EventGroupMemberKick        EventType = "5"  // 有人被踢
	EventGroupSelfKicked        EventType = "6"  // 我被踢
	EventGroupMemberInvited     EventType = "7"  // 某人已被邀请入群（已加入）
	EventGroupMemberJoin        EventType = "8"  // 某人加入了群（已加入）
	EventGroupMemberBan         EventType = "9"  // 某人被禁言
	EventGroupMemberUnban       EventType = "10" // 某人被解除禁言
	EventGroupMessageRecall     EventType = "11" // 群聊撤回消息
	EventGroupReactionAdd       EventType = "12" // 有人对消息表情回应
	EventGroupReactionRemove    EventType = "13" // 有人对消息取消表情回应
	EventGroupEssenceSet        EventType = "14" // 设置精华消息
	EventGroupJoinRequest       EventType = "15" // 有加群申请
	EventGroupSelfInvited       EventType = "16" // 我被邀请加群
	EventGroupTitleChange       EventType = "17" // 头衔变更
	EventGroupHonorChange       EventType = "18" // 荣誉变更（龙王、群聊火花等）
	EventGroupNameChange        EventType = "19" // 群名称变更
	EventFriendAdded            EventType = "20" // 新添加好友（已同意）
	EventFriendMessageRecall    EventType = "21" // 好友撤回消息
	EventFriendAddRequest       EventType = "22" // 有加好友申请
	EventPoke                   EventType = "23" // 戳一戳（来源可能是群或好友）
	EventGroupWholeBan          EventType = "24" // 全员禁言
	EventGroupWholeUnban        EventType = "25" // 取消全员禁言
	EventGroupCardChange        EventType = "28" // 群员名片变更
	EventFrameworkRobotOffline  EventType = "29" // 框架 QQ 掉线
	EventFrameworkWebSocketDown EventType = "30" // WebSocket 服务断开（将无法再收到消息）
	EventFrameworkWebSocketUp   EventType = "31" // WebSocket 服务重连成功
)

// EventDescriptions 给出事件类型的中文说明，便于日志输出。
var EventDescriptions = map[EventType]string{
	EventGroupFileUpload:        "有人上传文件",
	EventGroupAdminAdd:          "群管理员变动：添加",
	EventGroupAdminRemove:       "群管理员变动：取消",
	EventGroupMemberLeave:       "有人退群",
	EventGroupMemberKick:        "有人被踢",
	EventGroupSelfKicked:        "我被踢",
	EventGroupMemberInvited:     "某人已被邀请入群",
	EventGroupMemberJoin:        "某人加入了群",
	EventGroupMemberBan:         "某人被禁言",
	EventGroupMemberUnban:       "某人被解除禁言",
	EventGroupMessageRecall:     "群聊撤回消息",
	EventGroupReactionAdd:       "有人对消息表情回应",
	EventGroupReactionRemove:    "有人对消息取消表情回应",
	EventGroupEssenceSet:        "设置精华消息",
	EventGroupJoinRequest:       "有加群申请",
	EventGroupSelfInvited:       "我被邀请加群",
	EventGroupTitleChange:       "头衔变更",
	EventGroupHonorChange:       "荣誉变更",
	EventGroupNameChange:        "群名称变更",
	EventFriendAdded:            "新添加好友",
	EventFriendMessageRecall:    "好友撤回消息",
	EventFriendAddRequest:       "有加好友申请",
	EventPoke:                   "戳一戳",
	EventGroupWholeBan:          "全员禁言",
	EventGroupWholeUnban:        "取消全员禁言",
	EventGroupCardChange:        "群员名片变更",
	EventFrameworkRobotOffline:  "机器人下线",
	EventFrameworkWebSocketDown: "WebSocket 断开",
	EventFrameworkWebSocketUp:   "WebSocket 重连成功",
}

// 框架返回的数据结构，字段与易语言 SDK 自定义数据类型一一对应。

// RobotInfo 是机器人信息（API 1）。
// OneBot 返回的 user_id 是数字，这里统一保留为字符串，避免大号 QQ 丢失精度。
type RobotInfo struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
}

// UnmarshalJSON 兼容 user_id 为数字或字符串两种形态。
func (info *RobotInfo) UnmarshalJSON(data []byte) error {
	var raw struct {
		UserID   json.RawMessage `json:"user_id"`
		Nickname string          `json:"nickname"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	info.Nickname = raw.Nickname
	info.UserID = strings.Trim(strings.TrimSpace(string(raw.UserID)), `"`)
	return nil
}

// FriendInfo 是好友或陌生人信息（API 39、41）。
type FriendInfo struct {
	QQ        string // QQ
	Nickname  string // 昵称
	VIPLevel  int    // 会员等级
	Gender    int    // 性别：0 男 1 女 -1 未知
	QQLevel   int    // QQ 等级
	QID       string // QID
	Signature string // 个性签名
}

// FriendListItem 是好友列表中的一项（API 40）。
type FriendListItem struct {
	QQ       string // QQ
	Nickname string // 昵称
	Gender   int    // 性别：0 男 1 女 -1 未知
}

// GroupInfo 是群信息（API 43）。
type GroupInfo struct {
	Name      string // 名称
	MemberNum int    // 当前人数
	MemberMax int    // 最大人数
}

// GroupListItem 是群列表中的一项（API 44）。
type GroupListItem struct {
	GroupID   string // 群号
	Name      string // 名称
	MemberNum int    // 当前人数
	MemberMax int    // 最大人数
}

// GroupMemberInfo 是群成员信息（API 45）。
type GroupMemberInfo struct {
	Nickname     string // QQ 昵称
	Card         string // 群名片
	QQLevel      int    // QQ 等级
	Level        int    // 群聊等级（活跃等级）
	Gender       int    // 性别：0 男 1 女 -1 未知
	Role         int    // 身份：0 普通成员 1 管理 2 群主
	JoinTime     int64  // 加群时间戳
	LastSentTime int64  // 最后发言时间戳
	Title        string // 头衔，空表示没有头衔
}

// GroupMemberListItem 是群成员列表中的一项（API 46）。
type GroupMemberListItem struct {
	QQ           string // QQ
	Nickname     string // QQ 昵称
	Card         string // 群名片
	Level        int    // 群聊等级（活跃等级）
	JoinTime     int64  // 加群时间戳
	LastSentTime int64  // 最后发言时间戳
	Role         int    // 身份：0 普通成员 1 管理 2 群主
}

// GroupBanItem 是群禁言列表中的一项（API 54）。
type GroupBanItem struct {
	QQ       string // QQ
	Nickname string // QQ 昵称
	EndTime  int64  // 禁言结束时间戳
}

// FilteredFriendRequest 是被过滤的好友申请（API 58），只能用 HandleFilteredFriendRequest 处理。
type FilteredFriendRequest struct {
	QQ        string // QQ
	Flag      string // 消息 flag
	Reason    string // 申请理由
	Time      int64  // 申请时间戳
	FilterWhy string // 被过滤原因
	Nickname  string // QQ 昵称
}

// ForwardNode 是自定义合并消息的一条节点，格式为 QQ||昵称||消息内容。
type ForwardNode struct {
	QQ       string
	Nickname string
	Content  string
}

// IPCMessage 是 C 壳与 Go 工作进程之间的一条 JSON Lines 消息。
type IPCMessage struct {
	Type       string   `json:"type"`
	ID         string   `json:"id,omitempty"`
	Event      string   `json:"event,omitempty"`
	ArgsB64    []string `json:"args_b64,omitempty"`
	CommandB64 string   `json:"command_b64,omitempty"`
	ValueB64   string   `json:"value_b64,omitempty"`
	Result     int      `json:"result,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// APITransport 是框架调用的传输层。生产环境由 IPC 客户端实现，测试可注入替身。
type APITransport interface {
	Call(command string) (string, error)
}

// BeeNTAPI 是 Bee×NT 框架 API 的 Go 封装。
// 它持有初始化时框架传入的核心模块地址与插件索引，所有框架调用都通过它发出。
type BeeNTAPI struct {
	core   uintptr
	index  string
	appDir string
	call   func(api int, args ...string) (string, error)
}

// processState 保存工作进程内的全局状态。框架在整个生命周期内只初始化一次，
// 因此 API 对象是进程级单例，所有回调共享同一个核心地址与插件索引。
type processState struct {
	mu        sync.Mutex
	api       *BeeNTAPI
	transport APITransport
}

var global processState

// NewBeeNTAPI 用框架传入的核心模块地址与插件索引构造 API 对象。
func NewBeeNTAPI(core uintptr, index string) *BeeNTAPI {
	api := &BeeNTAPI{core: core, index: index}
	api.call = func(code int, args ...string) (string, error) {
		return api.invoke(code, args...)
	}
	return api
}

// Core 返回核心模块地址，供需要直接调用框架入口的场景使用。
func (api *BeeNTAPI) Core() uintptr { return api.core }

// Index 返回插件索引。
func (api *BeeNTAPI) Index() string { return api.index }

// SetAppDataDir 记录插件数据目录，由工作进程在初始化时设置。
func (api *BeeNTAPI) SetAppDataDir(dir string) { api.appDir = dir }

// GetAppDataDir 返回当前插件的数据目录，形如 D:\Bee×NT\plugin_data\插件名\。
// 插件配置、数据库、缓存和日志都应写入该目录。
func (api *BeeNTAPI) GetAppDataDir() (string, error) {
	if api.appDir == "" {
		return "", errors.New("插件数据目录尚未初始化")
	}
	return api.appDir, nil
}

// invoke 组装一次框架调用：API 编号、插件索引、参数依次用框架换行符拼接。
func (api *BeeNTAPI) invoke(code int, args ...string) (string, error) {
	transport := currentTransport()
	if transport == nil {
		return "", errors.New("框架调用传输层尚未就绪")
	}
	parts := make([]string, 0, 2+len(args))
	parts = append(parts, strconv.Itoa(code), api.index)
	parts = append(parts, args...)
	return transport.Call(strings.Join(parts, SeparatorNewline))
}

func currentTransport() APITransport {
	global.mu.Lock()
	defer global.mu.Unlock()
	return global.transport
}

func setCurrentTransport(transport APITransport) {
	global.mu.Lock()
	global.transport = transport
	global.mu.Unlock()
}

func currentAPI() *BeeNTAPI {
	global.mu.Lock()
	defer global.mu.Unlock()
	return global.api
}

func setCurrentAPI(api *BeeNTAPI) {
	global.mu.Lock()
	global.api = api
	global.mu.Unlock()
}

// boolText 将布尔值转换为框架约定的文本："1" 为真，"0" 为假。
func boolText(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// parseBool 解析框架返回的逻辑值，兼容 "1"/"真" 与 "0"/"假"。
func parseBool(value string) bool {
	switch strings.TrimSpace(value) {
	case "1", "真", "true", "True", "TRUE":
		return true
	default:
		return false
	}
}

// splitLines 按框架换行符切分返回文本，去掉末尾空行。
func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, SeparatorNewline)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// splitFields 按框架分隔符切分一行。
func splitFields(value string) []string {
	return strings.Split(value, SeparatorField)
}

// field 安全读取一行的第 index 个字段（从 0 计）。
func field(fields []string, index int) string {
	if index < 0 || index >= len(fields) {
		return ""
	}
	return fields[index]
}

func atoi(value string) int {
	number, _ := strconv.Atoi(strings.TrimSpace(value))
	return number
}

func atoi64(value string) int64 {
	number, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return number
}

// encodeBase64 将字节编码为 Base64，供 IPC 使用。
func encodeBase64(value []byte) string { return base64.StdEncoding.EncodeToString(value) }

// decodeBase64 将 Base64 还原为字节。
func decodeBase64(value string) ([]byte, error) { return base64.StdEncoding.DecodeString(value) }

// decodeJSON 解析框架返回的 JSON 文本，空文本视为无数据。
func decodeJSON(value string, target any) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return json.Unmarshal([]byte(value), target)
}

// formatForward 将合并消息节点序列化为框架要求的文本：
// 每条为 QQ||昵称||消息内容，多条用换行符隔开，消息内容中的换行写作 \n。
func formatForward(nodes []ForwardNode) string {
	lines := make([]string, 0, len(nodes))
	for _, node := range nodes {
		content := strings.ReplaceAll(node.Content, "\r\n", "\\n")
		content = strings.ReplaceAll(content, "\n", "\\n")
		lines = append(lines, node.QQ+"||"+node.Nickname+"||"+content)
	}
	return strings.Join(lines, "\n")
}

// ErrNotInitialized 表示在框架初始化之前调用了 API。
var ErrNotInitialized = errors.New("插件尚未初始化")

// requireAPI 取出当前 API 对象，未初始化时返回错误。
func requireAPI() (*BeeNTAPI, error) {
	api := currentAPI()
	if api == nil {
		return nil, ErrNotInitialized
	}
	return api, nil
}

// describeError 统一格式化框架调用错误，便于日志定位。
func describeError(code int, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("框架调用 %d 失败: %w", code, err)
}

// pluginDataDir 返回当前插件的数据目录。
// C 壳把工作进程释放在 框架目录\plugin_data\插件名\ 下，因此工作进程所在目录就是数据目录。
// 返回值带结尾反斜杠，与易语言 SDK 的"应用目录"约定一致。
func pluginDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe) + string(os.PathSeparator)
}

// ==================== 框架 API 1～97 ====================

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

// ==================== 工作进程：IPC 与事件分发 ====================

// ErrOutsideCallback 表示在插件回调之外调用了框架 API。
// C 壳只在等待事件结果期间处理 api_call，回调之外发出的调用得不到应答，必须提前报错而不是挂起。
var ErrOutsideCallback = errors.New("框架 API 只能在插件回调执行期间调用")

// ipcClient 通过标准输入输出与 C 壳通信：发出框架调用请求并等待结果。
// C 壳在等待事件结果期间会处理嵌套的 api_call，因此调用是同步且可重入的。
type ipcClient struct {
	write  func(IPCMessage) error
	read   func() (IPCMessage, error)
	mu     sync.Mutex
	seq    uint64
	active int32
}

// newIPCClient 构造一个 IPC 客户端。
func newIPCClient(write func(IPCMessage) error, read func() (IPCMessage, error)) *ipcClient {
	return &ipcClient{write: write, read: read}
}

// begin 与 end 标记一次插件回调的执行区间，只有区间内才允许调用框架 API。
func (c *ipcClient) begin() { atomic.AddInt32(&c.active, 1) }
func (c *ipcClient) end()   { atomic.AddInt32(&c.active, -1) }

// Call 发起一次框架调用，返回框架的文本结果。
func (c *ipcClient) Call(command string) (string, error) {
	if atomic.LoadInt32(&c.active) == 0 {
		return "", ErrOutsideCallback
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	id := strconv.FormatUint(c.seq, 10)
	if err := c.write(IPCMessage{Type: "api_call", ID: id, CommandB64: encodeBase64([]byte(command))}); err != nil {
		return "", err
	}
	reply, err := c.read()
	if err != nil {
		return "", err
	}
	if reply.Type != "api_result" || reply.ID != id {
		return "", fmt.Errorf("框架调用应答不匹配: type=%s id=%s", reply.Type, reply.ID)
	}
	if reply.Error != "" {
		return "", errors.New(reply.Error)
	}
	raw, err := decodeBase64(reply.ValueB64)
	if err != nil {
		return "", fmt.Errorf("框架返回解码失败: %w", err)
	}
	return string(raw), nil
}

// decodeArg 将 C 壳传来的 Base64 参数还原为 UTF-8 文本。
// C 壳在发送前已把框架的 GBK 文本转换为 UTF-8，这里只做 Base64 还原。
func decodeArg(value string) (string, error) {
	raw, err := decodeBase64(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// decodedArgs 批量还原事件参数。
func decodedArgs(msg IPCMessage) ([]string, error) {
	args := make([]string, len(msg.ArgsB64))
	for i, encoded := range msg.ArgsB64 {
		value, err := decodeArg(encoded)
		if err != nil {
			return nil, fmt.Errorf("参数 %d 解码失败: %w", i, err)
		}
		args[i] = value
	}
	return args, nil
}

// arg 安全读取参数，越界返回空字符串。
func arg(args []string, index int) string {
	if index < 0 || index >= len(args) {
		return ""
	}
	return args[index]
}

// dispatchEvent 根据事件名称调用插件回调，返回是否退出工作进程与处理结果。
// Bee×NT 没有独立的初始化回调：框架调用 BeeNT_初始化 时只取回插件信息和函数指针，
// 核心地址与插件索引由 C 壳通过命令行交给工作进程。
func dispatchEvent(msg IPCMessage) (shutdown bool, result IPCMessage) {
	result = IPCMessage{Type: "event_result", ID: msg.ID, Result: MessageIgnore}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Error = fmt.Sprintf("插件回调异常: %v", recovered)
		}
	}()
	if msg.Event == "stop" {
		closeSettingsWindow()
		return true, result
	}
	args, err := decodedArgs(msg)
	if err != nil {
		result.Error = err.Error()
		return false, result
	}
	switch msg.Event {
	case "enable":
		onEnable()
	case "disable":
		onDisable()
	case "unload":
		onUnload()
		return true, result
	case "settings":
		onSettings()
	case "friend_message":
		result.Result = onFriendMessage(arg(args, 0), arg(args, 1), arg(args, 2))
	case "group_message":
		result.Result = onGroupMessage(arg(args, 0), arg(args, 1), arg(args, 2), arg(args, 3))
	case "event_message":
		result.Result = onEventMessage(
			arg(args, 0), arg(args, 1), arg(args, 2), arg(args, 3),
			arg(args, 4), arg(args, 5), arg(args, 6), arg(args, 7),
		)
	default:
		result.Error = "未知事件: " + msg.Event
	}
	return false, result
}

// runWorker 运行 IPC 主循环：读取事件、调用回调、写回结果。
func runWorker(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(output)
	var readMu sync.Mutex
	readMessage := func() (IPCMessage, error) {
		readMu.Lock()
		defer readMu.Unlock()
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return IPCMessage{}, err
			}
			return IPCMessage{}, io.EOF
		}
		var message IPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return IPCMessage{}, err
		}
		return message, nil
	}
	var writeMu sync.Mutex
	writeMessage := func(message IPCMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return encoder.Encode(message)
	}
	client := newIPCClient(writeMessage, readMessage)
	setCurrentTransport(client)
	defer setCurrentTransport(nil)

	for {
		msg, err := readMessage()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			_ = writeMessage(IPCMessage{Type: "event_result", Error: err.Error()})
			continue
		}
		if msg.Type != "event" {
			_ = writeMessage(IPCMessage{Type: "event_result", ID: msg.ID, Error: "不支持的消息类型"})
			continue
		}
		client.begin()
		shutdown, result := dispatchEvent(msg)
		client.end()
		if err := writeMessage(result); err != nil {
			return err
		}
		if shutdown {
			return nil
		}
	}
}

// workerOptions 是 C 壳启动工作进程时传入的参数。
type workerOptions struct {
	HostPID     uint32
	Core        uintptr
	PluginIndex string
}

// parseWorkerOptions 读取 --host-pid、--core 与 --plugin-index。
// 核心地址与插件索引在框架调用 BeeNT_初始化 时确定，之后不再变化。
func parseWorkerOptions(args []string) (workerOptions, error) {
	var options workerOptions
	seen := map[string]bool{}
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "--host-pid", "--core", "--plugin-index":
			if seen[args[i]] {
				return workerOptions{}, fmt.Errorf("参数重复: %s", args[i])
			}
			seen[args[i]] = true
			switch args[i] {
			case "--host-pid":
				value, err := strconv.ParseUint(args[i+1], 10, 32)
				if err != nil {
					return workerOptions{}, fmt.Errorf("--host-pid 无效: %w", err)
				}
				options.HostPID = uint32(value)
			case "--core":
				value, err := strconv.ParseUint(args[i+1], 10, 64)
				if err != nil || value == 0 {
					return workerOptions{}, errors.New("--core 无效")
				}
				options.Core = uintptr(value)
			case "--plugin-index":
				options.PluginIndex = args[i+1]
			}
			i++
		}
	}
	if !seen["--host-pid"] || !seen["--core"] || !seen["--plugin-index"] {
		return workerOptions{}, errors.New("缺少 --host-pid、--core 或 --plugin-index 参数")
	}
	return options, nil
}

// prepareWorker 用命令行参数建立进程级 API 对象，并开始监控宿主进程。
func prepareWorker(args []string) error {
	options, err := parseWorkerOptions(args)
	if err != nil {
		return err
	}
	api := NewBeeNTAPI(options.Core, options.PluginIndex)
	api.SetAppDataDir(pluginDataDir())
	setCurrentAPI(api)
	startHostWatcher(options.HostPID)
	return nil
}
