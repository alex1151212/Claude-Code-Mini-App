## 下載

| 平台 | 檔案 | 啟動 |
|---|---|---|
| Windows（桌面版視窗＋系統匣，推薦） | `claude-miniapp-@TAG@-windows-amd64.zip` | 雙擊 `claude-miniapp-desktop.exe` |
| Windows（純 server，用瀏覽器／Telegram 連） | 同上 zip | 執行 `claude-miniapp.exe` |
| Linux（純 server） | `claude-miniapp-@TAG@-linux-amd64.tar.gz` | `./claude-miniapp` |

## 首次設定

1. **解壓整個資料夾**：執行檔必須與 `internal/static/`（前端）放在一起，不能只搬 exe。
2. 把 `config.example.yaml` 複製為 `config.yaml`，填入 `bot_token` 與 `whitelist_tg_ids`。只在本機使用、不對外開放時，可改設 `no_auth: true` 跳過驗證。
3. 先在同一台機器裝好你要用的 CLI（`claude`、`codex`、`kiro-cli`、`cursor agent`）。
4. 啟動後開 <http://localhost:8080>（桌面版會自動開視窗）。

> **Windows 注意**
> - 執行檔未簽章，SmartScreen 可能跳「Windows 已保護您的電腦」→ 點「其他資訊」→「仍要執行」。
> - 桌面版需要 WebView2 Runtime：Windows 11 內建，Windows 10 請先[安裝](https://developer.microsoft.com/microsoft-edge/webview2/)。
> - 桌面版關閉視窗只會縮到系統匣，要結束請在匣圖示按右鍵 →「結束」（或 Ctrl+Q）。
> - 驗證檔案完整性：`checksums.txt` 內有 SHA256。

---

## 更新內容

