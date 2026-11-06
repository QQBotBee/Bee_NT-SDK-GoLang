package main

import (
	"strings"
	"testing"
)

type fakeTransport struct {
	commands []string
	reply    string
}

func (f *fakeTransport) Call(command string) (string, error) {
	f.commands = append(f.commands, command)
	return f.reply, nil
}

func newFakeAPI(reply string) (*BeeNTAPI, *fakeTransport) {
	fake := &fakeTransport{reply: reply}
	setCurrentTransport(fake)
	return NewBeeNTAPI(1, "7"), fake
}

func cmd(parts ...string) string { return strings.Join(parts, SeparatorNewline) }

func TestRobotInfoAcceptsNumericUserID(t *testing.T) {
	api, _ := newFakeAPI(`{"user_id":10000000001,"nickname":"机器人"}`)
	defer setCurrentTransport(nil)
	info, err := api.RobotInfo()
	if err != nil || info.UserID != "10000000001" || info.Nickname != "机器人" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	api, _ = newFakeAPI(`{"user_id":"123","nickname":"x"}`)
	if info, _ = api.RobotInfo(); info.UserID != "123" {
		t.Fatalf("字符串形态的 user_id 解析失败: %+v", info)
	}
}

func TestFilteredFriendRequestsDefaultCount(t *testing.T) {
	api, fake := newFakeAPI("")
	defer setCurrentTransport(nil)
	_, _ = api.GetFilteredFriendRequests(0)
	_, _ = api.GetFilteredFriendRequests(5)
	if fake.commands[0] != cmd("58", "7", "") || fake.commands[1] != cmd("58", "7", "5") {
		t.Fatalf("commands=%q", fake.commands)
	}
}

func TestArgumentOrder(t *testing.T) {
	api, fake := newFakeAPI("")
	defer setCurrentTransport(nil)
	_, _ = api.GetGroupEmojiLikes("g", "m", "e", "1", 20)
	_ = api.HandleGroupRequest("g", "q", "f", true, GroupRequestApply, "欢迎")
	_ = api.ClearCache(CacheGroupMember, "g")
	_ = api.KickMembers("g", []string{"1", "2"}, true)
	want := []string{
		cmd("96", "7", "g", "m", "e", "1", "20"),
		cmd("38", "7", "g", "q", "f", "1", "26", "欢迎"),
		cmd("60", "7", "3", "g"),
		cmd("87", "7", "g", "1\n2", "1"),
	}
	for i, expected := range want {
		if fake.commands[i] != expected {
			t.Errorf("第 %d 条命令不一致:\n got %q\nwant %q", i, fake.commands[i], expected)
		}
	}
}

func TestListParsing(t *testing.T) {
	line := strings.Join([]string{"1", "昵称", "名片", "5", "100", "200", "2"}, SeparatorField)
	api, _ := newFakeAPI(line + SeparatorNewline)
	defer setCurrentTransport(nil)
	items, err := api.GetGroupMemberList("g")
	if err != nil || len(items) != 1 || items[0].QQ != "1" || items[0].Role != RoleOwner || items[0].JoinTime != 100 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestSetFailureBecomesError(t *testing.T) {
	api, _ := newFakeAPI("权限不足")
	defer setCurrentTransport(nil)
	err := api.Ban("g", "q", 60)
	fe, ok := err.(*FrameworkError)
	if !ok || fe.Message != "权限不足" {
		t.Fatalf("err=%v", err)
	}
}

func TestFormatForwardEscapesNewlines(t *testing.T) {
	got := formatForward([]ForwardNode{{"1", "甲", "a\r\nb"}, {"2", "乙", "c"}})
	if got != "1||甲||a\\nb\n2||乙||c" {
		t.Fatalf("got %q", got)
	}
}
