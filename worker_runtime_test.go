package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func eventArgs(event string, args ...string) IPCMessage {
	m := IPCMessage{Type: "event", ID: "e1", Event: event}
	for _, arg := range args {
		bytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(arg))
		if err != nil {
			panic(err)
		}
		m.ArgsB64 = append(m.ArgsB64, base64.StdEncoding.EncodeToString(bytes))
	}
	return m
}

func TestDispatchBeeNTCallbacks(t *testing.T) {
	var calls []string
	h := workerCallbacks{
		initialize: func() { calls = append(calls, "initialize") },
		enable:     func() { calls = append(calls, "enable") },
		disable:    func() { calls = append(calls, "disable") },
		unload:     func() { calls = append(calls, "unload") },
		settings:   func() { calls = append(calls, "settings") },
		private: func(id, content, msg string) int {
			calls = append(calls, strings.Join([]string{id, content, msg}, ","))
			return 1
		},
		group: func(g, id, content, msg string) int {
			calls = append(calls, strings.Join([]string{g, id, content, msg}, ","))
			return 1
		},
		event: func(kind, source, nick, id, opNick, op, content, flag string) int {
			calls = append(calls, strings.Join([]string{kind, source, nick, id, opNick, op, content, flag}, ","))
			return 0
		},
	}
	for _, tc := range []struct {
		name     string
		args     []string
		result   int
		shutdown bool
		call     string
	}{
		{"initialize", nil, 0, false, "initialize"},
		{"enable", nil, 0, false, "enable"},
		{"settings", nil, 0, false, "settings"},
		{"private_message", []string{"123", "中文\\uD83D\\uDE00", "-42"}, 1, false, "123,中文😀,-42"},
		{"group_message", []string{"100", "123", "测试", "-43"}, 1, false, "100,123,测试,-43"},
		{"common_event", []string{"15", "100", "昵称", "123", "管理员", "456", "申请", "flag"}, 0, false, "15,100,昵称,123,管理员,456,申请,flag"},
		{"disable", nil, 0, false, "disable"},
		{"unload", nil, 0, true, "unload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shutdown, got := dispatchEventWithHandlers(eventArgs(tc.name, tc.args...), h)
			if got.Error != "" || got.Type != "event_result" || got.ID != "e1" || got.Result != tc.result || shutdown != tc.shutdown {
				t.Fatalf("got %v, shutdown=%v", got, shutdown)
			}
			if calls[len(calls)-1] != tc.call {
				t.Fatalf("call=%q", calls[len(calls)-1])
			}
		})
	}
}

func TestDispatchInvalidAndPanic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message IPCMessage
		h       workerCallbacks
	}{
		{"missing argument", eventArgs("group_message", "1"), workerCallbacks{}},
		{"unknown event", eventArgs("unknown"), workerCallbacks{}},
		{"invalid base64", IPCMessage{Type: "event", Event: "private_message", ArgsB64: []string{"!"}}, workerCallbacks{}},
		{"panic", eventArgs("enable"), workerCallbacks{enable: func() { panic("plugin panic") }}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := dispatchEventWithHandlers(tc.message, tc.h)
			if got.Error == "" || got.Result != 0 {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestWorkerNestedAndBackgroundAPI(t *testing.T) {
	in, send := io.Pipe()
	receive, out := io.Pipe()
	defer in.Close()
	defer send.Close()
	defer receive.Close()
	defer out.Close()
	done := make(chan error, 1)
	h := workerCallbacks{enable: func() {
		result, err := currentAPITransport().Call([]byte("2#|&BEExNT&|#7#|&BEExNT&|#启用"))
		if err != nil || string(result) != "同步结果" {
			panic("API result missing")
		}
	}}
	go func() { done <- runWorker(in, out, 7, h) }()
	encoder := json.NewEncoder(send)
	scanner := bufio.NewScanner(receive)
	read := func() IPCMessage {
		t.Helper()
		if !scanner.Scan() {
			t.Fatalf("worker output ended: %v", scanner.Err())
		}
		var message IPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			t.Fatal(err)
		}
		return message
	}
	if err := encoder.Encode(eventArgs("enable")); err != nil {
		t.Fatal(err)
	}
	apiCall := read()
	if apiCall.Type != "api_call" {
		t.Fatalf("got %+v", apiCall)
	}
	if err := encoder.Encode(IPCMessage{Type: "api_result", ID: apiCall.ID, ValueB64: base64.StdEncoding.EncodeToString([]byte("同步结果"))}); err != nil {
		t.Fatal(err)
	}
	if reply := read(); reply.Type != "event_result" || reply.Error != "" {
		t.Fatalf("event=%+v", reply)
	}
	background := make(chan error, 1)
	go func() { _, err := currentAPITransport().Call([]byte("background")); background <- err }()
	apiCall = read()
	if err := encoder.Encode(IPCMessage{Type: "api_result", ID: apiCall.ID}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-background:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("background API blocked outside callback")
	}
	if err := encoder.Encode(eventArgs("stop")); err != nil {
		t.Fatal(err)
	}
	if reply := read(); reply.Type != "event_result" {
		t.Fatalf("reply=%v", reply)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not end worker")
	}
}

func TestWorkerEOFAndInvalidInput(t *testing.T) {
	if err := runWorker(strings.NewReader(""), io.Discard, 0, workerCallbacks{}); err != nil {
		t.Fatal(err)
	}
	if err := runWorker(strings.NewReader("not-json\n"), io.Discard, 0, workerCallbacks{}); err == nil {
		t.Fatal("malformed IPC accepted")
	}
}

func TestParseWorkerArgs(t *testing.T) {
	pid, index, err := parseWorkerArgs([]string{"--plugin-index", "0", "--host-pid", "123"})
	if err != nil || pid != 123 || index != 0 {
		t.Fatalf("got %d/%d, %v", pid, index, err)
	}
	for _, args := range [][]string{nil, {"--host-pid", "0", "--plugin-index", "0"}, {"--host-pid", "123"}, {"--host-pid", "123", "--plugin-index", "-1"}, {"--host-pid", "4294967296", "--plugin-index", "0"}} {
		if _, _, err := parseWorkerArgs(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
