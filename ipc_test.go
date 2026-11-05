package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestIPCClientConcurrentResponses(t *testing.T) {
	requests := make(chan IPCMessage, 12)
	client := newIPCClient(func(m IPCMessage) error { requests <- m; return nil }, time.Second)
	defer client.close(io.EOF)
	var wg sync.WaitGroup
	errorsFound := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			command := fmt.Sprintf("command-%d-中文", i)
			value, err := client.Call([]byte(command))
			if err != nil || string(value) != command {
				errorsFound <- fmt.Errorf("got %q, %v for %q", value, err, command)
			}
		}(i)
	}
	var pending []IPCMessage
	for i := 0; i < 12; i++ {
		pending = append(pending, <-requests)
	}
	for i := len(pending) - 1; i >= 0; i-- {
		m := pending[i]
		client.deliver(IPCMessage{Type: "api_result", ID: m.ID, ValueB64: m.CommandB64})
	}
	wg.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

func TestIPCClientTimeoutAndLateResponse(t *testing.T) {
	requests := make(chan IPCMessage, 1)
	client := newIPCClient(func(m IPCMessage) error { requests <- m; return nil }, 20*time.Millisecond)
	defer client.close(io.EOF)
	done := make(chan error, 1)
	go func() { _, err := client.Call([]byte("timeout")); done <- err }()
	request := <-requests
	if err := <-done; !errors.Is(err, ErrAPITimeout) {
		t.Fatalf("error = %v", err)
	}
	// 超时结果与未知 ID 不得阻塞唯一 reader。
	client.deliver(IPCMessage{Type: "api_result", ID: request.ID, ValueB64: ""})
	client.deliver(IPCMessage{Type: "api_result", ID: "unknown"})
}

func TestIPCClientDisconnectReleasesCallers(t *testing.T) {
	request := make(chan IPCMessage, 1)
	client := newIPCClient(func(m IPCMessage) error { request <- m; return nil }, time.Second)
	done := make(chan error, 1)
	go func() { _, err := client.Call([]byte("wait")); done <- err }()
	<-request
	client.close(io.EOF)
	select {
	case err := <-done:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected API remained blocked")
	}
	if _, err := client.Call(nil); !errors.Is(err, io.EOF) {
		t.Fatalf("subsequent call = %v", err)
	}
}

func TestIPCClientResponseErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response IPCMessage
	}{
		{"core error", IPCMessage{Error: "core unavailable"}},
		{"invalid base64", IPCMessage{ValueB64: "!"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var client *IPCClient
			client = newIPCClient(func(m IPCMessage) error {
				r := tc.response
				r.Type = "api_result"
				r.ID = m.ID
				client.deliver(r)
				return nil
			}, time.Second)
			defer client.close(io.EOF)
			if _, err := client.Call(nil); err == nil {
				t.Fatal("expected response error")
			}
		})
	}
}

func TestIPCClientWriteFailure(t *testing.T) {
	writeError := errors.New("pipe write failed")
	client := newIPCClient(func(IPCMessage) error { return writeError }, time.Second)
	defer client.close(io.EOF)
	if _, err := client.Call([]byte("x")); !errors.Is(err, writeError) {
		t.Fatalf("error = %v", err)
	}
}

func TestIPCClientTimeoutIncludesBlockedWrite(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	client := newIPCClient(func(IPCMessage) error { close(blocked); <-release; return nil }, 10*time.Millisecond)
	defer client.close(io.EOF)
	done := make(chan error, 1)
	go func() { _, err := client.Call([]byte("blocked write")); done <- err }()
	<-blocked
	select {
	case err := <-done:
		if !errors.Is(err, ErrAPITimeout) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("API timeout did not include blocked pipe write")
	}
}

func TestIPCClientUTF8(t *testing.T) {
	var client *IPCClient
	client = newIPCClient(func(m IPCMessage) error {
		client.deliver(IPCMessage{Type: "api_result", ID: m.ID, ValueB64: base64.StdEncoding.EncodeToString([]byte("中文😀"))})
		return nil
	}, time.Second)
	defer client.close(io.EOF)
	value, err := client.Call([]byte("payload"))
	if err != nil || string(value) != "中文😀" {
		t.Fatalf("got %q, %v", value, err)
	}
}
