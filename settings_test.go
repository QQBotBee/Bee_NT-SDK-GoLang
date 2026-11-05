//go:build windows

package main

import (
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Opt in because this exercises a real top-level window on the interactive
// desktop. Structural tests cannot detect CreateWindowExW/callback failures.
func TestSettingsRealWindowLifecycle(t *testing.T) {
	if os.Getenv("BEENT_SETTINGS_UI_TEST") != "1" {
		t.Skip("set BEENT_SETTINGS_UI_TEST=1 for native window verification")
	}
	defer closeSettingsWindow()
	findWindow := user32.NewProc("FindWindowW")
	isVisible := user32.NewProc("IsWindowVisible")
	className, _ := syscall.UTF16PtrFromString("BeeNTGoSettingsWindow")
	waitWindow := func() uintptr {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			hwnd, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0)
			if hwnd != 0 {
				return hwnd
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("settings window never appeared in FindWindowW")
		return 0
	}
	showSettingsWindow()
	hwnd := waitWindow()
	visible, _, _ := isVisible.Call(hwnd)
	if visible == 0 {
		t.Fatal("settings HWND exists but is not visible")
	}
	showSettingsWindow()
	if again, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0); again != hwnd {
		t.Fatalf("second settings call changed HWND %x -> %x", hwnd, again)
	}
	closeSettingsWindow()
	if remaining, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0); remaining != 0 {
		t.Fatal("disable-style close left HWND alive")
	}
	showSettingsWindow()
	hwnd = waitWindow()
	settingsNative.Lock()
	done := settingsNative.done
	settingsNative.Unlock()
	procPostMessageW.Call(hwnd, wmClose, 0, 0)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WM_CLOSE did not destroy the settings window")
	}
}

func TestSettingsNativeStructLayouts(t *testing.T) {
	var expectedClassSize, expectedPaintSize, expectedMessageSize uintptr = 48, 64, 32
	if unsafe.Sizeof(uintptr(0)) == 8 {
		expectedClassSize, expectedPaintSize, expectedMessageSize = 80, 72, 48
	}
	if size := unsafe.Sizeof(wndClassEx{}); size != expectedClassSize {
		t.Errorf("WNDCLASSEX size=%d want=%d", size, expectedClassSize)
	}
	if size := unsafe.Sizeof(paintStruct{}); size != expectedPaintSize {
		t.Errorf("PAINTSTRUCT size=%d want=%d", size, expectedPaintSize)
	}
	if size := unsafe.Sizeof(msg{}); size != expectedMessageSize {
		t.Errorf("MSG size=%d want=%d", size, expectedMessageSize)
	}
}

// 模拟关闭发生在窗口线程已启动但 HWND 尚未创建的间隙。
func TestSettingsCloseBeforeWindowCreated(t *testing.T) {
	done := make(chan struct{})
	settingsNative.Lock()
	settingsNative.hwnd = 0
	settingsNative.done = done
	settingsNative.closing = true
	settingsNative.Unlock()
	go settingsWindowThread(done)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		// 清理失败用例打开的真实窗口，避免测试自己遗留窗口线程。
		settingsNative.Lock()
		hwnd := settingsNative.hwnd
		settingsNative.Unlock()
		if hwnd != 0 {
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("close requested before HWND creation did not stop window thread")
	}
}
