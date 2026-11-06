//go:build !windows

package main

// 非 Windows 平台的空实现，仅用于让 go vet 与单元测试能在其他系统上编译。

func showSettingsWindow() {}

func closeSettingsWindow() {}

func startHostWatcher(hostPID uint32) {}
