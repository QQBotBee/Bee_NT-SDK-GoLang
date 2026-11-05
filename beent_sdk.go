package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	// BeeNTSeparator separates the opcode, plugin index and API arguments.
	BeeNTSeparator = "#|&BEExNT&|#"
	// BeeNTFieldSeparator separates columns inside list records.
	BeeNTFieldSeparator = "/c.BeExNTs.\\"
	MessageContinue     = 0
	MessageIgnore       = MessageContinue
	MessageIntercept    = 1
)

// BeeNTAPI binds a framework transport to a plugin index. It is safe for
// concurrent use when its transport supports concurrent calls.
type BeeNTAPI struct {
	index     int32
	transport APITransport
}

// NewBeeNTAPI returns the API connection installed by the Worker runtime.
func NewBeeNTAPI() (*BeeNTAPI, error) {
	return NewBeeNTAPIWithTransport(currentPluginIndex(), currentAPITransport())
}

// NewBeeNTAPIWithTransport creates an API connection for an explicit plugin.
func NewBeeNTAPIWithTransport(index int32, transport APITransport) (*BeeNTAPI, error) {
	if index < 0 {
		return nil, errors.New("BeeNT plugin index must be nonnegative")
	}
	if transport == nil {
		return nil, errors.New("BeeNT API transport is not connected")
	}
	v := reflect.ValueOf(transport)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Func || v.Kind() == reflect.Slice || v.Kind() == reflect.Chan) && v.IsNil() {
		return nil, errors.New("BeeNT API transport is nil")
	}
	return &BeeNTAPI{index: index, transport: transport}, nil
}

// Call invokes an opcode from the supplied BeeNT reference. Framework business
// failures remain the returned text; errors describe validation/IPC failures.
// Results are kept verbatim so JSON and literal backslashes are not corrupted.
func (api *BeeNTAPI) Call(opcode int, args ...string) (string, error) {
	if api == nil || api.transport == nil {
		return "", errors.New("BeeNT API transport is not connected")
	}
	if opcode < 1 || opcode >= len(apiArgumentCounts) {
		return "", fmt.Errorf("unsupported BeeNT opcode %d", opcode)
	}
	if len(args) != apiArgumentCounts[opcode] {
		return "", fmt.Errorf("BeeNT opcode %d requires %d arguments, got %d", opcode, apiArgumentCounts[opcode], len(args))
	}
	fields := make([]string, 0, len(args)+2)
	fields = append(fields, strconv.Itoa(opcode), strconv.FormatInt(int64(api.index), 10))
	for i, arg := range args {
		if !utf8.ValidString(arg) {
			return "", fmt.Errorf("BeeNT argument %d contains invalid UTF-8", i+1)
		}
		if strings.ContainsRune(arg, 0) || strings.Contains(arg, BeeNTSeparator) {
			return "", fmt.Errorf("BeeNT argument %d contains a reserved protocol delimiter or NUL", i+1)
		}
		fields = append(fields, arg)
	}
	command := encodeFrameworkText(strings.Join(fields, BeeNTSeparator))
	result, err := api.transport.Call([]byte(command))
	if err != nil {
		return "", fmt.Errorf("BeeNT opcode %d: %w", opcode, err)
	}
	if !utf8.Valid(result) {
		return "", fmt.Errorf("BeeNT opcode %d returned invalid UTF-8", opcode)
	}
	return string(result), nil
}

func encodeFrameworkText(value string) string {
	var out strings.Builder
	encoder := simplifiedchinese.GBK.NewEncoder()
	decoder := simplifiedchinese.GBK.NewDecoder()
	for _, r := range value {
		if r < 128 {
			out.WriteRune(r)
			continue
		}
		part := string(r)
		encoded, err := encoder.Bytes([]byte(part))
		if err == nil {
			decoded, decodeErr := decoder.Bytes(encoded)
			if decodeErr == nil && string(decoded) == part {
				out.WriteString(part)
				continue
			}
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, `\u%04X`, unit)
		}
	}
	return out.String()
}

// DecodeFrameworkText decodes the framework's UTF-16 \uXXXX escapes in plain
// text. Do not apply this to JSON before json.Unmarshal; it can unescape quotes.
// Malformed escapes and unpaired surrogates are preserved unchanged.
func DecodeFrameworkText(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		unit, ok := unicodeUnit(value, i)
		if !ok {
			out.WriteByte(value[i])
			i++
			continue
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			low, valid := unicodeUnit(value, i+6)
			if valid && low >= 0xdc00 && low <= 0xdfff {
				out.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
				i += 12
				continue
			}
			out.WriteString(value[i : i+6])
			i += 6
			continue
		}
		if unit >= 0xdc00 && unit <= 0xdfff {
			out.WriteString(value[i : i+6])
			i += 6
			continue
		}
		out.WriteRune(rune(unit))
		i += 6
	}
	return out.String()
}

func unicodeUnit(value string, index int) (uint16, bool) {
	if index+6 > len(value) || value[index] != '\\' || value[index+1] != 'u' {
		return 0, false
	}
	v, err := strconv.ParseUint(value[index+2:index+6], 16, 16)
	return uint16(v), err == nil
}

func boolArgument(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (api *BeeNTAPI) callBool(opcode int, args ...string) (bool, error) {
	raw, err := api.Call(opcode, args...)
	if err != nil {
		return false, err
	}
	switch strings.TrimSpace(raw) {
	case "1", "真":
		return true, nil
	case "0", "假":
		return false, nil
	default:
		return false, fmt.Errorf("BeeNT opcode %d returned invalid boolean %q", opcode, raw)
	}
}

// GetAppDataDir returns the Worker's directory, which the native bridge creates
// as <host directory>/plugin_data/<PluginName>. It never uses the DLL path.
func GetAppDataDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}
func (api *BeeNTAPI) GetAppDataDir() (string, error) { return GetAppDataDir() }

// Event values are strings because BeeNT delivers eventType as text.
const (
	EventGroupFileUpload       = "1"
	EventGroupAdminAdded       = "2"
	EventGroupAdminRemoved     = "3"
	EventGroupMemberLeft       = "4"
	EventGroupMemberKicked     = "5"
	EventRobotKicked           = "6"
	EventGroupMemberInvited    = "7"
	EventGroupMemberJoined     = "8"
	EventGroupMemberBanned     = "9"
	EventGroupMemberUnbanned   = "10"
	EventGroupMessageRecalled  = "11"
	EventGroupReactionAdded    = "12"
	EventGroupReactionRemoved  = "13"
	EventGroupEssenceAdded     = "14"
	EventGroupJoinRequest      = "15"
	EventGroupInvitation       = "16"
	EventGroupTitleChanged     = "17"
	EventGroupHonorChanged     = "18"
	EventGroupNameChanged      = "19"
	EventFriendAdded           = "20"
	EventFriendMessageRecalled = "21"
	EventFriendRequest         = "22"
	EventPoke                  = "23"
	EventGroupWholeBanEnabled  = "24"
	EventGroupWholeBanDisabled = "25"
	GroupRequestApply          = "26"
	GroupRequestInvite         = "27"
	EventGroupCardChanged      = "28"
	EventRobotOffline          = "29"
	EventWebSocketDisconnected = "30"
	EventWebSocketReconnected  = "31"
)

const (
	CacheAll = iota
	CacheFriendList
	CacheGroupList
	CacheGroupMemberList
	CacheGroupAdminList
)

type RobotInfo struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
}
type FriendInfo struct {
	QQ        string
	Nickname  string
	VIPLevel  int
	Sex       int
	QQLevel   int
	QID       string
	Signature string
}
type FriendListItem struct {
	QQ       string
	Nickname string
	Sex      int
}
type GroupInfo struct {
	Name           string
	MemberCount    int
	MaxMemberCount int
}
type GroupListItem struct {
	GroupID        string
	Name           string
	MemberCount    int
	MaxMemberCount int
}
type GroupMemberInfo struct {
	Nickname     string
	Card         string
	QQLevel      int
	GroupLevel   int
	Sex          int
	Role         int
	JoinTime     int64
	LastSentTime int64
	Title        string
}
type GroupMemberListItem struct {
	QQ           string
	Nickname     string
	Card         string
	GroupLevel   int
	JoinTime     int64
	LastSentTime int64
	Role         int
}
type GroupBanListItem struct {
	QQ       string
	Nickname string
	EndTime  int64
}
type FilteredFriendRequest struct {
	QQ           string
	Flag         string
	Reason       string
	Time         int64
	FilterReason string
	Nickname     string
}

func splitResult(raw, separator string, count int) ([]string, error) {
	parts := strings.Split(raw, separator)
	// Framework records often include a final separator, but an empty last
	// field (such as a missing title) is meaningful and must be retained.
	if len(parts) == count+1 && parts[len(parts)-1] == "" {
		parts = parts[:count]
	}
	if len(parts) != count {
		return nil, fmt.Errorf("malformed BeeNT record: expected %d fields, got %d (%q)", count, len(parts), raw)
	}
	return parts, nil
}

func numberField(parts []string, index int) (int, error) {
	n, err := strconv.ParseInt(parts[index], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid BeeNT integer field %d: %q", index+1, parts[index])
	}
	return int(n), nil
}
func timestampField(parts []string, index int) (int64, error) {
	n, err := strconv.ParseInt(parts[index], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid BeeNT timestamp field %d: %q", index+1, parts[index])
	}
	return n, nil
}
func textField(parts []string, index int) string { return DecodeFrameworkText(parts[index]) }

func parseFriendInfo(raw string) (FriendInfo, error) {
	p, err := splitResult(raw, BeeNTSeparator, 7)
	if err != nil {
		return FriendInfo{}, err
	}
	vip, err := numberField(p, 2)
	if err != nil {
		return FriendInfo{}, err
	}
	sex, err := numberField(p, 3)
	if err != nil {
		return FriendInfo{}, err
	}
	level, err := numberField(p, 4)
	if err != nil {
		return FriendInfo{}, err
	}
	return FriendInfo{QQ: p[0], Nickname: textField(p, 1), VIPLevel: vip, Sex: sex, QQLevel: level, QID: textField(p, 5), Signature: textField(p, 6)}, nil
}

func parseGroupInfo(raw string) (GroupInfo, error) {
	p, err := splitResult(raw, BeeNTSeparator, 3)
	if err != nil {
		return GroupInfo{}, err
	}
	count, err := numberField(p, 1)
	if err != nil {
		return GroupInfo{}, err
	}
	max, err := numberField(p, 2)
	if err != nil {
		return GroupInfo{}, err
	}
	return GroupInfo{Name: textField(p, 0), MemberCount: count, MaxMemberCount: max}, nil
}

func parseGroupMemberInfo(raw string) (GroupMemberInfo, error) {
	p, err := splitResult(raw, BeeNTSeparator, 9)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	qqLevel, err := numberField(p, 2)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	level, err := numberField(p, 3)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	sex, err := numberField(p, 4)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	role, err := numberField(p, 5)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	join, err := timestampField(p, 6)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	last, err := timestampField(p, 7)
	if err != nil {
		return GroupMemberInfo{}, err
	}
	return GroupMemberInfo{Nickname: textField(p, 0), Card: textField(p, 1), QQLevel: qqLevel, GroupLevel: level, Sex: sex, Role: role, JoinTime: join, LastSentTime: last, Title: textField(p, 8)}, nil
}

func parseList[T any](raw string, parse func([]string) (T, error), columns int) ([]T, error) {
	items := make([]T, 0)
	if raw == "" {
		return items, nil
	}
	rows := strings.Split(raw, BeeNTSeparator)
	if rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	for i, row := range rows {
		fields, err := splitResult(row, BeeNTFieldSeparator, columns)
		if err != nil {
			return nil, fmt.Errorf("BeeNT row %d: %w", i+1, err)
		}
		item, err := parse(fields)
		if err != nil {
			return nil, fmt.Errorf("BeeNT row %d: %w", i+1, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func parseFriendList(raw string) ([]FriendListItem, error) {
	return parseList(raw, func(p []string) (FriendListItem, error) {
		sex, err := numberField(p, 2)
		return FriendListItem{QQ: p[0], Nickname: textField(p, 1), Sex: sex}, err
	}, 3)
}
func parseGroupList(raw string) ([]GroupListItem, error) {
	return parseList(raw, func(p []string) (GroupListItem, error) {
		count, err := numberField(p, 2)
		if err != nil {
			return GroupListItem{}, err
		}
		max, err := numberField(p, 3)
		return GroupListItem{GroupID: p[0], Name: textField(p, 1), MemberCount: count, MaxMemberCount: max}, err
	}, 4)
}
func parseGroupMemberList(raw string) ([]GroupMemberListItem, error) {
	return parseList(raw, func(p []string) (GroupMemberListItem, error) {
		level, err := numberField(p, 3)
		if err != nil {
			return GroupMemberListItem{}, err
		}
		join, err := timestampField(p, 4)
		if err != nil {
			return GroupMemberListItem{}, err
		}
		last, err := timestampField(p, 5)
		if err != nil {
			return GroupMemberListItem{}, err
		}
		role, err := numberField(p, 6)
		return GroupMemberListItem{QQ: p[0], Nickname: textField(p, 1), Card: textField(p, 2), GroupLevel: level, JoinTime: join, LastSentTime: last, Role: role}, err
	}, 7)
}
func parseGroupBanList(raw string) ([]GroupBanListItem, error) {
	return parseList(raw, func(p []string) (GroupBanListItem, error) {
		end, err := timestampField(p, 2)
		return GroupBanListItem{QQ: p[0], Nickname: textField(p, 1), EndTime: end}, err
	}, 3)
}
func parseFilteredFriendRequests(raw string) ([]FilteredFriendRequest, error) {
	return parseList(raw, func(p []string) (FilteredFriendRequest, error) {
		when, err := timestampField(p, 3)
		return FilteredFriendRequest{QQ: p[0], Flag: p[1], Reason: textField(p, 2), Time: when, FilterReason: textField(p, 4), Nickname: textField(p, 5)}, err
	}, 6)
}

func parseRobotInfo(raw string) (RobotInfo, error) {
	var data struct {
		UserID   json.RawMessage `json:"user_id"`
		Nickname string          `json:"nickname"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return RobotInfo{}, fmt.Errorf("parse BeeNT robot JSON: %w", err)
	}
	var id string
	if len(data.UserID) > 0 && data.UserID[0] == '"' {
		if err := json.Unmarshal(data.UserID, &id); err != nil {
			return RobotInfo{}, err
		}
	} else {
		id = string(data.UserID)
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return RobotInfo{}, fmt.Errorf("invalid BeeNT robot user_id %q", id)
		}
	}
	if id == "" {
		return RobotInfo{}, fmt.Errorf("BeeNT robot JSON has no user_id")
	}
	return RobotInfo{UserID: id, Nickname: data.Nickname}, nil
}

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

// IPCMessage 是 C 壳和 Go Worker 的 JSON Lines 协议。文本载荷经 Base64
// 包装，API 使用 UTF-8；宿主回调的 args_b64 是原始 GBK 字节。
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

// APITransport 不允许将宿主地址拿到 Worker 中直接调用。
// Call 的命令和返回值均为 UTF-8，C 壳负责 GBK 转换和进程内核心调用。
type APITransport interface {
	Call(command []byte) ([]byte, error)
}

var ErrAPITimeout = errors.New("BeeNT API call timed out")

const defaultAPITimeout = 200 * time.Second

// IPCClient 允许多个后台任务同时调用 API。只有 Worker 的 reader 分发结果，
// Call 从不读取共享管道，避免事件和 API 结果互相抢读。
type IPCClient struct {
	mu      sync.Mutex
	write   func(IPCMessage) error
	writes  chan ipcWrite
	pending map[string]chan IPCMessage
	done    chan struct{}
	err     error
	nextID  atomic.Uint64
	timeout time.Duration
}

type ipcWrite struct {
	message IPCMessage
	result  chan error
	expires time.Time
}

func newIPCClient(write func(IPCMessage) error, timeout time.Duration) *IPCClient {
	if timeout <= 0 {
		timeout = defaultAPITimeout
	}
	client := &IPCClient{write: write, writes: make(chan ipcWrite, 64), pending: make(map[string]chan IPCMessage), done: make(chan struct{}), timeout: timeout}
	go client.writeLoop()
	return client
}

// A single bounded write queue keeps Call cancellation effective even when the
// pipe stops accepting bytes. At most one writer is blocked on the OS pipe.
func (client *IPCClient) writeLoop() {
	for {
		select {
		case <-client.done:
			return
		case request := <-client.writes:
			select {
			case <-client.done:
				return
			default:
			}
			if time.Now().After(request.expires) {
				request.result <- ErrAPITimeout
				continue
			}
			err := client.write(request.message)
			request.result <- err
			if err != nil {
				client.close(err)
				return
			}
		}
	}
}

func (client *IPCClient) Call(command []byte) ([]byte, error) {
	if client == nil || client.write == nil {
		return nil, errors.New("BeeNT API transport is not connected")
	}
	id := "api-" + strconv.FormatUint(client.nextID.Add(1), 10)
	reply := make(chan IPCMessage, 1)
	client.mu.Lock()
	if client.err != nil {
		err := client.err
		client.mu.Unlock()
		return nil, err
	}
	client.pending[id] = reply
	client.mu.Unlock()
	defer func() { client.mu.Lock(); delete(client.pending, id); client.mu.Unlock() }()
	timer := time.NewTimer(client.timeout)
	defer timer.Stop()
	request := ipcWrite{
		message: IPCMessage{Type: "api_call", ID: id, CommandB64: base64.StdEncoding.EncodeToString(command)},
		result:  make(chan error, 1), expires: time.Now().Add(client.timeout),
	}
	select {
	case client.writes <- request:
	case <-timer.C:
		return nil, ErrAPITimeout
	case <-client.done:
		client.mu.Lock()
		err := client.err
		client.mu.Unlock()
		return nil, err
	}
	for {
		select {
		case err := <-request.result:
			if err != nil {
				return nil, err
			}
		case response := <-reply:
			if response.Error != "" {
				return nil, errors.New(response.Error)
			}
			value, err := base64.StdEncoding.DecodeString(response.ValueB64)
			if err != nil {
				return nil, fmt.Errorf("invalid BeeNT API result: %w", err)
			}
			return value, nil
		case <-client.done:
			client.mu.Lock()
			err := client.err
			client.mu.Unlock()
			return nil, err
		case <-timer.C:
			return nil, ErrAPITimeout
		}
	}
}

func (client *IPCClient) deliver(response IPCMessage) {
	client.mu.Lock()
	reply := client.pending[response.ID]
	if reply != nil {
		delete(client.pending, response.ID)
	}
	client.mu.Unlock()
	if reply != nil {
		reply <- response
	}
}

func (client *IPCClient) close(err error) {
	if err == nil {
		err = io.EOF
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err == nil {
		client.err = err
		close(client.done)
	}
}

var activeAPI struct {
	sync.RWMutex
	transport APITransport
	index     int32
}

func setCurrentAPITransport(transport APITransport, index int32) {
	activeAPI.Lock()
	activeAPI.transport, activeAPI.index = transport, index
	activeAPI.Unlock()
}

func currentAPITransport() APITransport {
	activeAPI.RLock()
	defer activeAPI.RUnlock()
	return activeAPI.transport
}

func currentPluginIndex() int32 {
	activeAPI.RLock()
	defer activeAPI.RUnlock()
	return activeAPI.index
}
