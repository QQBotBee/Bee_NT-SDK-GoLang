package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func b64(value string) string { return encodeBase64([]byte(value)) }

func TestIPCClientRejectsCallOutsideCallback(t *testing.T) {
	client := newIPCClient(func(IPCMessage) error { return nil }, func() (IPCMessage, error) { return IPCMessage{}, nil })
	if _, err := client.Call("1"); !errors.Is(err, ErrOutsideCallback) {
		t.Fatalf("回调之外应返回 ErrOutsideCallback，实际: %v", err)
	}
}

func TestIPCClientRoundTrip(t *testing.T) {
	var sent IPCMessage
	client := newIPCClient(
		func(m IPCMessage) error { sent = m; return nil },
		func() (IPCMessage, error) {
			return IPCMessage{Type: "api_result", ID: "1", ValueB64: b64("结果")}, nil
		},
	)
	client.begin()
	defer client.end()
	value, err := client.Call("命令")
	if err != nil || value != "结果" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	if sent.Type != "api_call" || sent.CommandB64 != b64("命令") {
		t.Fatalf("发出的消息不对: %+v", sent)
	}
}

func TestIPCClientMismatchedAndFailedReply(t *testing.T) {
	cases := map[string]IPCMessage{
		"id 不匹配": {Type: "api_result", ID: "9"},
		"类型不对":   {Type: "event", ID: "1"},
		"框架报错":   {Type: "api_result", ID: "1", Error: "boom"},
	}
	for name, reply := range cases {
		reply := reply
		client := newIPCClient(func(IPCMessage) error { return nil }, func() (IPCMessage, error) { return reply, nil })
		client.begin()
		if _, err := client.Call("x"); err == nil {
			t.Errorf("%s: 应返回错误", name)
		}
		client.end()
	}
}

func TestParseWorkerOptions(t *testing.T) {
	options, err := parseWorkerOptions([]string{"--host-pid", "42", "--core", "4096", "--plugin-index", "3"})
	if err != nil || options.HostPID != 42 || options.Core != 4096 || options.PluginIndex != "3" {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	bad := [][]string{
		{"--host-pid", "42", "--core", "4096"},
		{"--host-pid", "42", "--core", "0", "--plugin-index", "3"},
		{"--host-pid", "x", "--core", "1", "--plugin-index", "3"},
		{"--host-pid", "1", "--host-pid", "2", "--core", "1", "--plugin-index", "3"},
	}
	for _, args := range bad {
		if _, err := parseWorkerOptions(args); err == nil {
			t.Errorf("应拒绝参数: %v", args)
		}
	}
}

func TestDispatchEventRouting(t *testing.T) {
	setCurrentAPI(nil)
	shutdown, result := dispatchEvent(IPCMessage{Event: "friend_message", ID: "5", ArgsB64: []string{b64("1"), b64("hi"), b64("m")}})
	if shutdown || result.ID != "5" || result.Result != MessageIgnore || result.Error != "" {
		t.Fatalf("shutdown=%v result=%+v", shutdown, result)
	}
	if _, result = dispatchEvent(IPCMessage{Event: "nope"}); !strings.Contains(result.Error, "未知事件") {
		t.Fatalf("未知事件应报错: %+v", result)
	}
	if shutdown, _ = dispatchEvent(IPCMessage{Event: "unload"}); !shutdown {
		t.Fatal("unload 应退出工作进程")
	}
	if _, result = dispatchEvent(IPCMessage{Event: "enable", ArgsB64: []string{"!!"}}); result.Error == "" {
		t.Fatal("非法 Base64 参数应报错")
	}
}

func mustLine(t *testing.T, message IPCMessage) string {
	t.Helper()
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func TestRunWorkerLoop(t *testing.T) {
	input := mustLine(t, IPCMessage{Type: "event", ID: "1", Event: "group_message", ArgsB64: []string{b64("g"), b64("q"), b64("hi"), b64("m")}}) +
		mustLine(t, IPCMessage{Type: "api_result", ID: "x"}) + // 不是事件，应被拒绝
		"这不是 JSON\n" +
		mustLine(t, IPCMessage{Type: "event", ID: "2", Event: "stop"}) +
		mustLine(t, IPCMessage{Type: "event", ID: "3", Event: "enable"}) // stop 之后的消息不应被处理
	var output bytes.Buffer
	if err := runWorker(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	var replies []IPCMessage
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var reply IPCMessage
		if err := json.Unmarshal([]byte(line), &reply); err != nil {
			t.Fatalf("输出不是合法 JSON: %q", line)
		}
		replies = append(replies, reply)
	}
	if len(replies) != 4 {
		t.Fatalf("应有 4 条应答，实际 %d: %q", len(replies), output.String())
	}
	if replies[0].ID != "1" || replies[0].Error != "" || replies[0].Result != MessageIgnore {
		t.Errorf("事件应答不对: %+v", replies[0])
	}
	if replies[1].Error == "" || replies[2].Error == "" {
		t.Errorf("非事件消息与非法 JSON 都应返回错误: %+v %+v", replies[1], replies[2])
	}
	if replies[3].ID != "2" || replies[3].Error != "" {
		t.Errorf("stop 应答不对: %+v", replies[3])
	}
	if currentTransport() != nil {
		t.Error("runWorker 退出后应清理传输层")
	}
}

func TestRunWorkerSetsTransportWhileRunning(t *testing.T) {
	reader, writer := io.Pipe()
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runWorker(reader, &syncWriter{w: &output}) }()
	_, _ = writer.Write([]byte(mustLine(t, IPCMessage{Type: "event", ID: "1", Event: "enable"})))
	deadline := time.Now().Add(2 * time.Second)
	for currentTransport() == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if currentTransport() == nil {
		t.Fatal("运行期间应已建立传输层")
	}
	_ = writer.Close() // 输入结束，runWorker 应正常返回
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// syncWriter 让测试在另一个 goroutine 写入时也能安全读取。
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
