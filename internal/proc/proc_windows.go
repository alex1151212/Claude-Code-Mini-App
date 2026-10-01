//go:build windows

package proc

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW，syscall 套件未匯出此常數
const ctrlBreakEvent = 1          // CTRL_BREAK_EVENT

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	generateCtrlEventProc = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

// SysProcAttr 回傳 Windows 專屬的進程屬性。
// CREATE_NEW_PROCESS_GROUP 讓子進程獨立成一個 process group，
// 使 GenerateConsoleCtrlEvent 能精確定位該群組而不影響父 console。
// CREATE_NO_WINDOW 避免每次 spawn CLI 子進程時彈出主控台黑窗。
func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// KillTree 強制終止目標進程及其所有子孫進程（等同 SIGKILL）。
// 使用 taskkill /F /T 確保 .cmd → node.exe 整條 wrapper 鏈都被殺掉。
func KillTree(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill /F /T /PID %d: %w (output: %s)", pid, err, out)
	}
	return nil
}

// SendInterrupt 對目標進程群組發送 CTRL_BREAK，讓進程有機會優雅退出。
// 用於「第一次按停止」：可能被子進程忽略，此時呼叫端應再次呼叫 KillTree 強制終止。
// 注意：需搭配 SysProcAttr() 的 CREATE_NEW_PROCESS_GROUP 才能正確定位群組；
// CTRL_C 在該旗標下預設被遮罩，因此改用 CTRL_BREAK。
func SendInterrupt(pid int) error {
	r, _, err := generateCtrlEventProc.Call(ctrlBreakEvent, uintptr(pid))
	if r == 0 {
		return fmt.Errorf("GenerateConsoleCtrlEvent(CTRL_BREAK, %d): %w", pid, err)
	}
	return nil
}
