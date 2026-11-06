//go:build beent_worker_runtime

package main

import (
	"fmt"
	"os"
)

// main 是 Go 工作进程入口，由 build.bat 经 buildmeta 复制到项目根目录后参与编译。
// C 壳启动工作进程时传入 --host-pid、--core 与 --plugin-index；
// 标准输入输出专用于和 C 壳通信的 IPC，不能被插件的打印输出污染。
func main() {
	if err := prepareWorker(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// 插件中的 fmt.Print/log 输出改走标准错误，避免破坏 IPC 协议流。
	protocolOut := os.Stdout
	os.Stdout = os.Stderr
	if err := runWorker(os.Stdin, protocolOut); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
