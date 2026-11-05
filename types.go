package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

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
