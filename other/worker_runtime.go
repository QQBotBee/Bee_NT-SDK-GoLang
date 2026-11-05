//go:build beent_worker_runtime

package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type workerCallbacks struct {
	initialize func()
	enable     func()
	disable    func()
	unload     func()
	settings   func()
	private    func(string, string, string) int
	group      func(string, string, string, string) int
	event      func(string, string, string, string, string, string, string, string) int
}

func pluginCallbacks() workerCallbacks {
	return workerCallbacks{
		initialize: onInitialize, enable: onEnable, disable: onDisable,
		unload: onUnload, settings: onSettings,
		private: onPrivateMessage, group: onGroupMessage, event: onEvent,
	}
}

func decodedArgs(message IPCMessage) ([]string, error) {
	args := make([]string, len(message.ArgsB64))
	for i, value := range message.ArgsB64 {
		raw, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("argument %d Base64: %w", i, err)
		}
		decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
		if err != nil {
			return nil, fmt.Errorf("argument %d GBK: %w", i, err)
		}
		args[i] = DecodeFrameworkText(string(decoded))
	}
	return args, nil
}

func dispatchEventWithHandlers(message IPCMessage, handlers workerCallbacks) (shutdown bool, result IPCMessage) {
	result = IPCMessage{Type: "event_result", ID: message.ID}
	defer func() {
		if failure := recover(); failure != nil {
			result.Result = 0
			result.Error = fmt.Sprintf("plugin callback panic (%s): %v", message.Event, failure)
		}
	}()
	if message.Event == "stop" {
		closeSettingsWindow()
		return true, result
	}
	args, err := decodedArgs(message)
	if err != nil {
		result.Error = err.Error()
		return false, result
	}
	counts := map[string]int{"initialize": 0, "enable": 0, "disable": 0, "unload": 0, "settings": 0, "private_message": 3, "group_message": 4, "common_event": 8}
	count, ok := counts[message.Event]
	if !ok || len(args) != count {
		result.Error = fmt.Sprintf("invalid BeeNT event %q: received %d arguments", message.Event, len(args))
		return false, result
	}
	switch message.Event {
	case "initialize":
		if handlers.initialize != nil {
			handlers.initialize()
		}
	case "enable":
		if handlers.enable != nil {
			handlers.enable()
		}
	case "disable":
		if handlers.disable != nil {
			handlers.disable()
		}
	case "unload":
		if handlers.unload != nil {
			handlers.unload()
		}
		shutdown = true
	case "settings":
		if handlers.settings != nil {
			handlers.settings()
		}
	case "private_message":
		if handlers.private != nil {
			result.Result = handlers.private(args[0], args[1], args[2])
		}
	case "group_message":
		if handlers.group != nil {
			result.Result = handlers.group(args[0], args[1], args[2], args[3])
		}
	case "common_event":
		if handlers.event != nil {
			result.Result = handlers.event(args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7])
		}
	}
	if result.Result != 1 {
		result.Result = 0
	}
	return shutdown, result
}

// runWorker 的唯一 reader 在执行回调期间继续分发 API 结果，因此回调内的
// 同步调用和回调外的后台调用都可以完成。业务事件始终按接收顺序执行。
func runWorker(input io.Reader, output io.Writer, index int32, handlers workerCallbacks) error {
	if closer, ok := input.(io.Closer); ok {
		defer closer.Close()
	}
	var writeMu sync.Mutex
	encoder := json.NewEncoder(output)
	write := func(message IPCMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return encoder.Encode(message)
	}
	client := newIPCClient(write, defaultAPITimeout)
	setCurrentAPITransport(client, index)
	defer setCurrentAPITransport(nil, 0)
	defer client.close(io.EOF)
	defer closeSettingsWindow()
	events := make(chan IPCMessage, 32)
	readResult := make(chan error, 1)
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 4096), 8*1024*1024)
		var readErr error
		for scanner.Scan() {
			var message IPCMessage
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
				readErr = fmt.Errorf("invalid IPC JSON: %w", err)
				break
			}
			switch message.Type {
			case "api_result":
				client.deliver(message)
			case "event":
				select {
				case events <- message:
				case <-finished:
					return
				default:
					readErr = errors.New("BeeNT event queue overflow")
				}
			default:
				readErr = fmt.Errorf("unsupported IPC type %q", message.Type)
			}
			if readErr != nil {
				break
			}
		}
		if readErr == nil {
			readErr = scanner.Err()
		}
		client.close(readErr)
		readResult <- readErr
	}()
	for message := range events {
		shutdown, result := dispatchEventWithHandlers(message, handlers)
		if err := write(result); err != nil {
			return err
		}
		if shutdown {
			return nil
		}
	}
	return <-readResult
}

func parseWorkerArgs(args []string) (uint32, int32, error) {
	var pid uint64
	var index int64
	var havePID, haveIndex bool
	for i := 0; i < len(args); i += 2 {
		if i+1 == len(args) {
			return 0, 0, errors.New("missing worker argument value")
		}
		var err error
		switch args[i] {
		case "--host-pid":
			if havePID {
				return 0, 0, errors.New("duplicate host PID")
			}
			pid, err = strconv.ParseUint(args[i+1], 10, 32)
			havePID = true
		case "--plugin-index":
			if haveIndex {
				return 0, 0, errors.New("duplicate plugin index")
			}
			index, err = strconv.ParseInt(args[i+1], 10, 32)
			haveIndex = true
		default:
			return 0, 0, fmt.Errorf("unknown worker argument %q", args[i])
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if !havePID || pid == 0 || !haveIndex || index < 0 {
		return 0, 0, errors.New("valid --host-pid and --plugin-index are required")
	}
	return uint32(pid), int32(index), nil
}

func main() {
	hostPID, index, err := parseWorkerArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	startHostWatcher(hostPID)
	// 插件中的 fmt.Print/log 输出不得破坏 IPC。
	protocolOut := os.Stdout
	os.Stdout = os.Stderr
	if err := runWorker(os.Stdin, protocolOut, index, pluginCallbacks()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
