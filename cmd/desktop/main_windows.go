//go:build windows

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/server"
)

func main() {
	chdirToExeIfConfigured()

	syncLogs := logging.Init()
	defer syncLogs()

	started, err := server.Start(context.Background())
	if err != nil {
		slog.Error(err.Error())
		messageBox("Claude Code Mini App", err.Error())
		os.Exit(1)
	}

	icon := favicon()

	var show func()
	app := application.New(application.Options{
		Name:        "Claude Code Mini App",
		Description: "本機視窗殼，網頁與 Telegram Mini App 仍走同一個 port",
		Icon:        icon,
		Assets:      application.AlphaAssets,
		Logger:      slog.Default(),
		LogLevel:    slog.LevelInfo,
		OnShutdown: func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := started.Shutdown(shutCtx); err != nil {
				slog.Error("關閉 HTTP 服務失敗: " + err.Error())
			}
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.jerry12122.claude-miniapp",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if show != nil {
					show()
				}
			},
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Claude Code Mini App",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		URL:              started.URL() + "/",
		BackgroundColour: application.NewRGBA(0x15, 0x16, 0x1e, 255),
		// 預設在非 production 建置會開 devtools；這裡關掉，避免一般使用時出現。
		DevToolsEnabled: false,
		Windows: application.WindowsWindow{
			DisableMenu: true,
		},
		KeyBindings: map[string]func(application.Window){
			"ctrl+q": func(application.Window) { app.Quit() },
		},
	})
	show = func() {
		window.UnMinimise()
		window.Show()
		window.Focus()
	}
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("Claude Code Mini App")
	if icon != nil {
		tray.SetIcon(icon)
	}
	tray.OnClick(show)
	tray.OnDoubleClick(show)
	trayMenu := app.NewMenu()
	trayMenu.Add("開啟視窗").OnClick(func(*application.Context) { show() })
	trayMenu.AddSeparator()
	trayMenu.Add("結束").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(trayMenu)

	if err := app.Run(); err != nil {
		slog.Error(err.Error())
		messageBox("Claude Code Mini App", err.Error())
		os.Exit(1)
	}
}

// chdirToExeIfConfigured 在打包後的 exe 旁邊找到 config.yaml 時，把工作目錄切過去。
// go run 的 exe 在暫存目錄、旁邊沒有設定檔，就維持目前工作目錄（專案根）。
func chdirToExeIfConfigured() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		return
	}
	_ = os.Chdir(dir)
}

func messageBox(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	const flags = uintptr(0x00000010 | 0x00010000) // MB_ICONERROR | MB_SETFOREGROUND
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
}

// favicon 讀前端同一個圖示。工作目錄在 chdir 之後，與靜態檔路徑一致。
func favicon() []byte {
	b, err := os.ReadFile(filepath.Join("internal", "static", "favicon.ico"))
	if err != nil {
		slog.Error("讀取 favicon 失敗: " + err.Error())
		return nil
	}
	return b
}
