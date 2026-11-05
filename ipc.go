package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

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
