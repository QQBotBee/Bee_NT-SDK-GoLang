package main

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type contractTransport struct {
	command []byte
	reply   []byte
	err     error
	calls   int
}

func (f *contractTransport) Call(command []byte) ([]byte, error) {
	f.command = append([]byte(nil), command...)
	f.calls++
	return f.reply, f.err
}

func TestCallUsesSingleUTF8CommandWithIndex(t *testing.T) {
	f := &contractTransport{reply: []byte("成功")}
	a, err := NewBeeNTAPIWithTransport(12, f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.SendGroupMessage("123", "中文\r\nhello")
	want := "4#|&BEExNT&|#12#|&BEExNT&|#123#|&BEExNT&|#中文\r\nhello"
	if err != nil || got != "成功" || string(f.command) != want {
		t.Fatalf("got %q %v command=%q want=%q", got, err, f.command, want)
	}
}

func TestCallRejectsMalformedArgumentsBeforeTransport(t *testing.T) {
	cases := []struct {
		name string
		op   int
		args []string
	}{
		{"unknown", 98, nil}, {"zero", 0, nil}, {"too few", 4, []string{"123"}}, {"too many", 1, []string{"extra"}},
		{"delimiter", 4, []string{"123", "a#|&BEExNT&|#b"}}, {"nul", 2, []string{"a\x00b"}}, {"invalid utf8", 2, []string{string([]byte{0xff})}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &contractTransport{}
			a, _ := NewBeeNTAPIWithTransport(0, f)
			if _, err := a.Call(tc.op, tc.args...); err == nil {
				t.Fatal("expected validation error")
			}
			if f.calls != 0 {
				t.Fatal("invalid input reached host")
			}
		})
	}
	if _, err := NewBeeNTAPIWithTransport(-1, &contractTransport{}); err == nil {
		t.Fatal("negative index accepted")
	}
	if _, err := NewBeeNTAPIWithTransport(0, nil); err == nil {
		t.Fatal("nil transport accepted")
	}
	var nilTransport *contractTransport
	if _, err := NewBeeNTAPIWithTransport(0, nilTransport); err == nil {
		t.Fatal("typed nil transport accepted")
	}
	var missing *BeeNTAPI
	if _, err := missing.Call(1); err == nil {
		t.Fatal("nil API accepted")
	}
}

func TestCallEscapesUnrepresentableUnicodeWithoutCorruptingJSON(t *testing.T) {
	f := &contractTransport{reply: []byte(`{"text":"\uD83D\uDE00","quote":"\u0022"}`)}
	a, _ := NewBeeNTAPIWithTransport(0, f)
	got, err := a.SendGroupMessage("1", "中文😀")
	if err != nil || string(f.command) != `4#|&BEExNT&|#0#|&BEExNT&|#1#|&BEExNT&|#中文\uD83D\uDE00` || got != `{"text":"\uD83D\uDE00","quote":"\u0022"}` {
		t.Fatalf("command=%q reply=%q err=%v", f.command, got, err)
	}
	if decoded := DecodeFrameworkText(`中文\uD83D\uDE00`); decoded != "中文😀" {
		t.Fatalf("decoded=%q", decoded)
	}
	for _, raw := range []string{`\uD83D`, `\uDE00`, `\uZZZZ`, `\u12`, `C:\users\data`} {
		if decoded := DecodeFrameworkText(raw); decoded != raw {
			t.Fatalf("malformed or literal escape %q became %q", raw, decoded)
		}
	}
}

func TestCallPreservesEmptyFieldsAndBooleans(t *testing.T) {
	f := &contractTransport{}
	a, _ := NewBeeNTAPIWithTransport(0, f)
	_, err := a.HandleFriendRequest("123", "flag", false, "")
	if err != nil || string(f.command) != "37#|&BEExNT&|#0#|&BEExNT&|#123#|&BEExNT&|#flag#|&BEExNT&|#0#|&BEExNT&|#" {
		t.Fatalf("command=%q err=%v", f.command, err)
	}
	_, err = a.SetGroupWholeBan("123", true)
	if err != nil || string(f.command) != "30#|&BEExNT&|#0#|&BEExNT&|#123#|&BEExNT&|#1" {
		t.Fatalf("command=%q err=%v", f.command, err)
	}
}

func TestCorrectedReactionAndSpeechPayloads(t *testing.T) {
	f := &contractTransport{reply: []byte("{}")}
	a, _ := NewBeeNTAPIWithTransport(3, f)
	_, err := a.GetGroupMessageEmojiLikes("group", "message", "emoji", "type", 20)
	if err != nil || string(f.command) != "96#|&BEExNT&|#3#|&BEExNT&|#group#|&BEExNT&|#message#|&BEExNT&|#emoji#|&BEExNT&|#type#|&BEExNT&|#20" {
		t.Fatalf("op96=%q %v", f.command, err)
	}
	_, err = a.GetVoiceToText("message")
	if err != nil || string(f.command) != "97#|&BEExNT&|#3#|&BEExNT&|#message" {
		t.Fatalf("op97=%q %v", f.command, err)
	}
}

func TestMemberInfoKeepsNicknameAndInt64Times(t *testing.T) {
	f := &contractTransport{reply: []byte("nick#|&BEExNT&|#card#|&BEExNT&|#42#|&BEExNT&|#9#|&BEExNT&|#-1#|&BEExNT&|#2#|&BEExNT&|#4102444800#|&BEExNT&|#4102444900#|&BEExNT&|##|&BEExNT&|#")}
	a, _ := NewBeeNTAPIWithTransport(0, f)
	got, err := a.GetGroupMemberInfo("group", "qq")
	if err != nil || got.Nickname != "nick" || got.QQLevel != 42 || got.JoinTime != 4102444800 || got.LastSentTime != 4102444900 || got.Title != "" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestListsAndMalformedResponses(t *testing.T) {
	f := &contractTransport{reply: []byte("1/c.BeExNTs.\\one/c.BeExNTs.\\0#|&BEExNT&|#2/c.BeExNTs.\\two/c.BeExNTs.\\-1#|&BEExNT&|#")}
	a, _ := NewBeeNTAPIWithTransport(0, f)
	got, err := a.GetFriendList()
	if err != nil || len(got) != 2 || got[1].QQ != "2" || got[1].Sex != -1 {
		t.Fatalf("got %+v %v", got, err)
	}
	f.reply = []byte("failure reason")
	if _, err := a.GetFriendList(); err == nil {
		t.Fatal("short record accepted")
	}
	f.reply = []byte("group#|&BEExNT&|#not-a-number#|&BEExNT&|#10#|&BEExNT&|#")
	if _, err := a.GetGroupInfo("123"); err == nil {
		t.Fatal("bad count accepted")
	}
	f.reply = nil
	got, err = a.GetFriendList()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty list %+v %v", got, err)
	}
}

func TestBooleanAndTransportFailures(t *testing.T) {
	f := &contractTransport{reply: []byte("1")}
	a, _ := NewBeeNTAPIWithTransport(0, f)
	got, err := a.IsRobot("123")
	if err != nil || !got {
		t.Fatalf("got %v %v", got, err)
	}
	f.reply = []byte("failure")
	if _, err := a.IsRobot("123"); err == nil {
		t.Fatal("host failure parsed as false")
	}
	f.err = errors.New("closed")
	if _, err := a.GetFrameworkName(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("lost transport error: %v", err)
	}
	f.err = nil
	f.reply = []byte("not-json")
	if _, err := a.GetRobotInfo(); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestFluentTargetsUseSameAPIAndExplicitMessageID(t *testing.T) {
	f := &contractTransport{}
	a, _ := NewBeeNTAPIWithTransport(8, f)
	_, err := a.Friend("friend").Reply("message", "hi")
	if err != nil || string(f.command) != "82#|&BEExNT&|#8#|&BEExNT&|#friend#|&BEExNT&|#message#|&BEExNT&|#hi" {
		t.Fatalf("command=%q err=%v", f.command, err)
	}
	_, err = a.Group("group").SendText("hello")
	if err != nil || string(f.command) != "4#|&BEExNT&|#8#|&BEExNT&|#group#|&BEExNT&|#hello" {
		t.Fatalf("command=%q err=%v", f.command, err)
	}
}

func TestEveryNamedMethodUsesReceiveSideOpcodeAndArgumentOrder(t *testing.T) {
	// Order is checked against 易语言_插件API接收处理.txt, not apiArgumentCounts.
	names := []string{
		"GetRobotInfo", "Log", "SendFriendMessage", "SendGroupMessage", "SendFriendVoice", "SendGroupVoice", "UploadFriendImage", "UploadGroupImage", "SendFriendVideo", "SendGroupVideo",
		"At", "RockPaperScissors", "Dice", "SendGroupFriendCard", "SendFriendFriendCard", "SendGroupGroupCard", "SendFriendGroupCard", "SendFriendCustomMusic", "SendGroupCustomMusic", "SendFriendMusic",
		"SendGroupMusic", "SendFriendForward", "SendGroupForward", "SendGroupJSON", "SendFriendJSON", "SendGroupXML", "SendFriendXML", "Face", "SetGroupBan", "SetGroupWholeBan",
		"KickGroupMember", "SetGroupAdmin", "SetGroupCard", "SetGroupName", "LeaveGroup", "SetGroupTitle", "HandleFriendRequest", "HandleGroupRequest", "GetStrangerInfo", "GetFriendList",
		"GetFriendInfo", "RecallMessage", "GetGroupInfo", "GetGroupList", "GetGroupMemberInfo", "GetGroupMemberList", "GetGroupHonor", "GetCookie", "GetBKN", "GetNickname",
		"GetNicknameNoCache", "GetGroupNotices", "DeleteGroupNotice", "GetGroupBanList", "SetSignature", "SetAvatar", "GetClientKey", "GetFilteredFriendRequests", "HandleFilteredFriendRequest", "ClearCache",
		"SetNickname", "DeleteFriend", "PublishGroupNotice", "UploadGroupFile", "UploadFriendFile", "SetGroupAvatar", "GetGroupFiles", "DeleteGroupFile", "GetGroupFolderFiles", "CreateFlashTransfer",
		"Like", "RestartFramework", "GetFrameworkName", "GetFrameworkVersion", "GetFrameworkQQ", "GetGroupAdmins", "SetSex", "IsGroupMemberBanned", "IsRobot", "GetGroupAIRoles",
		"SendGroupAIVoice", "SendFriendReply", "SendGroupReply", "SetGroupEssence", "DeleteGroupEssence", "GetGroupEssences", "KickGroupMembers", "Image", "Voice", "Video",
		"ReplyCode", "GetFriendInfoJSON", "GetStrangerInfoJSON", "SetMessageEmojiLike", "GetEmojiLikeDetails", "GetGroupMessageEmojiLikes", "GetVoiceToText",
	}
	counts := []int{0, 1, 2, 2, 2, 2, 1, 1, 2, 2, 1, 0, 0, 2, 2, 2, 2, 7, 7, 3, 3, 2, 2, 2, 2, 2, 2, 1, 3, 2, 3, 3, 3, 2, 2, 3, 4, 6, 2, 0, 2, 1, 1, 0, 2, 1, 2, 1, 1, 1, 1, 1, 2, 1, 1, 1, 0, 1, 3, 2, 1, 1, 3, 3, 3, 2, 1, 2, 2, 2, 1, 0, 0, 0, 0, 1, 1, 2, 1, 1, 3, 3, 3, 2, 2, 1, 3, 1, 1, 1, 1, 2, 2, 3, 5, 5, 1}
	for index, name := range names {
		t.Run(name, func(t *testing.T) {
			op := index + 1
			f := &contractTransport{reply: []byte("fixture")}
			api, _ := NewBeeNTAPIWithTransport(9, f)
			method := reflect.ValueOf(api).MethodByName(name)
			if !method.IsValid() {
				t.Fatalf("missing method %s", name)
			}
			if method.Type().NumIn() != counts[index] {
				t.Fatalf("opcode %d requires %d business arguments, method has %d", op, counts[index], method.Type().NumIn())
			}
			values := make([]reflect.Value, counts[index])
			fields := []string{strconv.Itoa(op), "9"}
			for arg := range values {
				switch method.Type().In(arg).Kind() {
				case reflect.String:
					value := "arg" + strconv.Itoa(arg+1)
					values[arg] = reflect.ValueOf(value)
					fields = append(fields, value)
				case reflect.Int:
					values[arg] = reflect.ValueOf(7)
					fields = append(fields, "7")
				case reflect.Bool:
					values[arg] = reflect.ValueOf(true)
					fields = append(fields, "1")
				default:
					t.Fatal("unsupported method parameter")
				}
			}
			method.Call(values)
			want := strings.Join(fields, "#|&BEExNT&|#")
			if string(f.command) != want || f.calls != 1 {
				t.Fatalf("opcode %d command=%q want=%q calls=%d", op, f.command, want, f.calls)
			}
		})
	}
}

func TestTypedResponsesValidateFieldsAndPreserveEmptyColumns(t *testing.T) {
	cases := []struct {
		name  string
		op    int
		raw   string
		check func(*BeeNTAPI) error
	}{
		{"friend", 41, "123#|&BEExNT&|#nick#|&BEExNT&|#5#|&BEExNT&|#1#|&BEExNT&|#60#|&BEExNT&|#qid#|&BEExNT&|##|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetFriendInfo("123", false)
			if e == nil && (v.Nickname != "nick" || v.VIPLevel != 5 || v.QQLevel != 60 || v.Signature != "") {
				t.Fatalf("friend=%+v", v)
			}
			return e
		}},
		{"group", 43, "name#|&BEExNT&|#12#|&BEExNT&|#200#|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetGroupInfo("1")
			if e == nil && (v.MemberCount != 12 || v.MaxMemberCount != 200) {
				t.Fatalf("group=%+v", v)
			}
			return e
		}},
		{"group list", 44, "123/c.BeExNTs.\\name/c.BeExNTs.\\12/c.BeExNTs.\\200#|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetGroupList()
			if e == nil && (len(v) != 1 || v[0].GroupID != "123" || v[0].MemberCount != 12) {
				t.Fatalf("groups=%+v", v)
			}
			return e
		}},
		{"member list", 46, "123/c.BeExNTs.\\nick/c.BeExNTs.\\/c.BeExNTs.\\4/c.BeExNTs.\\4102444800/c.BeExNTs.\\4102444900/c.BeExNTs.\\2#|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetGroupMemberList("1")
			if e == nil && (len(v) != 1 || v[0].JoinTime != 4102444800 || v[0].Card != "") {
				t.Fatalf("members=%+v", v)
			}
			return e
		}},
		{"ban list", 54, "123/c.BeExNTs.\\nick/c.BeExNTs.\\4102444800#|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetGroupBanList("1")
			if e == nil && (len(v) != 1 || v[0].EndTime != 4102444800) {
				t.Fatalf("bans=%+v", v)
			}
			return e
		}},
		{"request list", 58, "123/c.BeExNTs.\\flag/c.BeExNTs.\\reason/c.BeExNTs.\\4102444800/c.BeExNTs.\\filter/c.BeExNTs.\\nick#|&BEExNT&|#", func(a *BeeNTAPI) error {
			v, e := a.GetFilteredFriendRequests(10)
			if e == nil && (len(v) != 1 || v[0].Flag != "flag" || v[0].Time != 4102444800) {
				t.Fatalf("requests=%+v", v)
			}
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &contractTransport{reply: []byte(tc.raw)}
			a, _ := NewBeeNTAPIWithTransport(0, f)
			if err := tc.check(a); err != nil {
				t.Fatal(err)
			}
			f.reply = []byte("bad record")
			if err := tc.check(a); err == nil {
				t.Fatal("malformed response accepted")
			}
		})
	}
	for _, raw := range []string{`{"user_id":1234567890123456789,"nickname":"\uD83D\uDE00"}`, `{"user_id":"1234567890123456789","nickname":"😀"}`} {
		f := &contractTransport{reply: []byte(raw)}
		a, _ := NewBeeNTAPIWithTransport(0, f)
		v, e := a.GetRobotInfo()
		if e != nil || v.UserID != "1234567890123456789" || v.Nickname != "😀" {
			t.Fatalf("robot=%+v err=%v", v, e)
		}
	}
}
