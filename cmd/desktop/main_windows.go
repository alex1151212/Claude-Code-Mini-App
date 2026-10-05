//go:build windows

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		RawMessageHandler: func(w application.Window, msg string, origin *application.OriginInfo) {
			handleTitleBarMessage(w, msg, origin, started.URL())
		},
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
		URL:              started.URL() + "/?desktop=1",
		BackgroundColour: application.NewRGBA(0x15, 0x16, 0x1e, 255),
		// 預設在非 production 建置會開 devtools；這裡關掉，避免一般使用時出現。
		DevToolsEnabled: false,
		// 無邊框：原生標題列只能是純色，拿掉後由網頁自己畫透明標題列，背景圖才能鋪到最上緣。
		// ?desktop=1 讓前端知道要顯示標題列（internal/static/js/core/desktop-titlebar.js）。
		Frameless: true,
		Windows: application.WindowsWindow{
			DisableMenu: true,
		},
		KeyBindings: map[string]func(application.Window){
			"ctrl+q": func(application.Window) { app.Quit() },
			// DisableMenu 後 WebView2 內建的重整快捷鍵不一定生效；前端改了靠這個重載。
			"f5":     func(w application.Window) { w.Reload() },
			"ctrl+r": func(w application.Window) { w.Reload() },
			"f12":    func(w application.Window) { w.OpenDevTools() }, // -tags production 時為空實作
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

// handleTitleBarMessage 處理網頁標題列按鈕送來的訊息（chrome.webview.postMessage）。
// 拖曳與邊緣縮放走 Wails 內建的 wails:drag / wails:resize:*，不經過這裡。
// 只認本機頁面送來的訊息，頁面裡其他來源的 iframe 不能操作視窗。
func handleTitleBarMessage(w application.Window, msg string, origin *application.OriginInfo, baseURL string) {
	if origin == nil || !strings.HasPrefix(origin.Origin, baseURL+"/") {
		return
	}
	switch msg {
	case "ra:window:minimise":
		w.Minimise()
	case "ra:window:toggle-maximise":
		w.ToggleMaximise()
	case "ra:window:close":
		// 觸發 WindowClosing，跟原生關閉一樣只是藏到系統匣。
		w.Close()
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
