package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
