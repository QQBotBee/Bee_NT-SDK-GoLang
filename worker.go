package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

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
