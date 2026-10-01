# 功能計畫書：桌面視窗殼（Wails）

> 狀態：**第一版已做**（2026-09-30）。系統匣未做，見文末。
> 建立日期：2026-09-30
> 規模：小到中（能開的第一版約 1–2 天；含系統匣與打包約 3–5 天）

---

## 0. 背景

現在是單一 Go binary：`cmd/server` 用 Fiber 同時提供靜態頁、REST、WebSocket。使用者要另外開一個主控台行程（黑盒子），再用瀏覽器或 Telegram Mini App 連上去。

桌面版要取代的是那個黑盒子，不是產品本身。同一個行程繼續聽 port，瀏覽器與 Mini App 照舊連；多出來的只有一個 WebView2 視窗，打開 `http://127.0.0.1:<port>/`。

`docs/plan/roadmap.md` 的「明確不做」原本把「內嵌瀏覽器／port 轉發」寫在一起。那條指的是把外部網站嵌進來、或做 port 轉發。本計畫不屬於那類：視窗只載入自己的 Fiber，不新增第二套 UI。

## 1. 目標

- 新增 `cmd/desktop`。啟動後沒有主控台視窗，直接看到現有網頁。
- 同一個行程繼續 `Listen`，區域網路瀏覽器與 Telegram Mini App 行為不變。
- `go run ./cmd/server`、`go build -o claude-miniapp.exe ./cmd/server` 維持純 Go、不需 CGO。

## 2. 非目標

- 不把前端改成 Wails binding，不重寫 `internal/static`。
- 不把靜態檔 `go:embed`。前端仍是 no-build，桌面版跟伺服器版一樣從磁碟讀 `./internal/static`。
- 不移除 `cmd/server`。
- 不做 macOS / Linux 桌面包。第一版只保證 Windows（WebView2）。
- 不做「關視窗後手機仍可用、但視窗在另一台機器」之類的遠端桌面。手機要連，仍是這台電腦上的同一個 port，跟今天一樣。

## 3. 架構

```
cmd/server/main.go     薄入口：logging → server.Start → Wait
cmd/desktop/main.go    薄入口：chdir 到 exe 目錄 → 單例 → server.Start
                       → Wails 視窗載入 http://127.0.0.1:<port>/
                       → 系統匣；關視窗只隱藏；匣內「結束」才 Shutdown

internal/server        從 cmd/server/main.go 搬出的 Fiber 啟動（現有路由原樣）
```

前端判斷不用改。`internal/static/js/core/core.js` 已經用 `window.Telegram` 區分 Mini App 與網頁登入。WebView2 沒有 Telegram 物件，會走現有密碼登入。預設 `web.allowed_cidrs` 含 `127.0.0.0/8`，本機視窗過得了內網檢查。

設定檔與靜態檔路徑維持相對工作目錄（`config.Load` 讀 `.`，`app.Static("/", "./internal/static")`）。桌面入口在 `Start` 之前 `os.Chdir` 到 exe 所在目錄，這兩個檔都不用改。打包時 exe、`config.yaml`、`internal/static/`、`logs/` 放在一起。

日誌已同時寫 stdout 與 `logs/server.log`。桌面版用 `-H windowsgui` 拿掉主控台後，仍看 `logs/server.log`。

### 啟動介面

`cmd/server` 今天把整個 Fiber 組裝寫在 `main()`，而且 `app.Listen` 會把行程卡住。桌面入口需要「綁定成功才開視窗」以及「結束時關掉 HTTP」。抽出來的形狀：

```go
func Start(ctx context.Context) (*Server, error) // 綁定成功才回傳
func (s *Server) URL() string                    // http://127.0.0.1:<port>
func (s *Server) Wait() error                    // 給 cmd/server 阻塞
func (s *Server) Shutdown(ctx context.Context) error
```

實作用 `net.Listen` 再交給 Fiber 的 `Listener`，這樣回傳時 port 已經綁定。`cmd/server` 的行為維持：讀設定失敗、bind 失敗都是 log 之後退出。

### 視窗怎麼載入頁面

Wails 的資產伺服器不拿來編譯現有前端。視窗裡放一頁極小的殼，啟動後導向 `URL()`。

這是本計畫唯一要先證明的點。WebView2 若拒絕從 Wails 來源跳到 `http://127.0.0.1`，就改成資產伺服器反向代理到 Fiber（同一 origin，WebSocket 要轉 upgrade）。代理是備案，POC 失敗才做，不提前寫。

### 行程生命週期

關視窗只隱藏，不結束行程，手機才不會跟著斷。再執行一次 exe 會聚焦既有視窗（Wails single instance）。結束用系統匣「結束」或 Ctrl+Q，先 `Shutdown` HTTP 再退出。

系統匣用 Wails v3 的 `SystemTray`（跟視窗同一個訊息迴圈）。左鍵叫回視窗，右鍵是「開啟視窗」與「結束」。

port 已被另一個行程占用（例如還開著 `cmd/server`）：對話框告知後退出。

### 建置

`cmd/desktop` 使用 Wails v3。v2 沒帶 `production` 會跳出「will not build without the correct build tags」；v3 沒有這道檢查。

- 日常 `go build ./...`、`go test ./...` 不會編到 Wails，也不需要 CGO。
- 桌面版：`go build -ldflags "-H windowsgui" -o claude-miniapp-desktop.exe ./cmd/desktop`。`-tags production` 只關掉 Wails debug log。
- Windows 目標不需 gcc（WebView2 loader 是純 Go）。Win11 已有 WebView2 Runtime；Win10 沒有的話要另外裝。

## 4. Phase

### Phase 0 — POC（阻塞後面全部）

Wails v2 的 WebView2 只攔截 `wails.localhost`。導向 `http://127.0.0.1` 會交給 WebView 預設處理，因此第一版用殼頁 `location.replace`，沒有做反向代理。

- [x] 以 Wails 原始碼確認跨 host 請求不會被資產伺服器吃掉
- [ ] 視窗能完成網頁登入（需本機點一次）
- [ ] WebSocket 能串流一則訊息（需本機點一次）

### Phase 1 — 抽出 `internal/server`（行為不變）

- [x] 將 `cmd/server/main.go` 的 Fiber 組裝搬到 `internal/server`
- [x] `cmd/server/main.go` 改成呼叫 `Start` + `Wait`
- [ ] `go run ./cmd/server`：登入、開 session、串流，與抽出前相同（路由未改，尚未人工點過）

### Phase 2 — `cmd/desktop`

- [x] Wails v3，Windows 建置不必再帶 `production` 才進得了程式
- [x] exe 旁邊有 `config.yaml` 時 chdir；`go run` 維持專案根目錄
- [x] `Start` 成功後殼頁導向 `URL()`
- [x] 關視窗隱藏（`HideWindowOnClose`）
- [x] 再執行一次 exe 叫回視窗（single instance）
- [x] 系統匣「結束」或 Ctrl+Q 才 `Shutdown`
- [x] port 占用時 MessageBox 並退出
- [x] 文件記載 `-H windowsgui`；log 仍是工作目錄下的 `logs/server.log`
- [x] 系統匣（Wails v3 `SystemTray`，左鍵叫回、右鍵結束）

### Phase 3 — 文件

- [x] README 加一段桌面版：建置需求、目錄要跟 exe 放一起、手機仍連同一個 port
- [x] `config.example.yaml` 不必為桌面版加新欄位

## 5. 驗收

1. `go run ./cmd/server` 與抽出前相同，且 `go test ./...` 不需要 CGO。
2. 桌面 exe 雙擊後沒有主控台，直接是現有 UI，可以登入並完成一輪串流。
3. 桌面行程執行中，同一台機器的瀏覽器打開同一 port，看到同一批 session。
4. 關掉視窗後，瀏覽器那個分頁的 WebSocket 仍在；選單「結束」後，port 才放開。
5. exe 目錄沒有 `config.yaml` 時，錯誤寫進該目錄的 `logs/server.log`（或在 chdir 之後、logging 之前就能看到的失敗訊息），而不是靜默退出。

## 6. 風險

| 風險 | 處理 |
|---|---|
| WebView 不允許跳到 localhost | Phase 0 決定。失敗則資產伺服器反代 Fiber，仍不改前端 |
| `go build ./...` 被 Wails / CGO 拖下水 | build tag，預設編譯略過 `cmd/desktop` |
| 捷徑的工作目錄不是 exe 目錄，找不到設定與靜態檔 | 桌面入口先 chdir，不改 `config.Load` |
| 前端 CDN（React、Babel、Tailwind）斷網就空白 | 與現在用瀏覽器打開相同，本計畫不 vendoring |
| 兩次雙擊搶 port | 第二個行程聚焦既有視窗並退出。port 被 `cmd/server` 占用時，對話框後退出 |

## 7. 第一版取捨

系統匣改接 Wails v3 內建的 `SystemTray`，不再自管 `Shell_NotifyIcon`。生命週期：

- 關視窗：隱藏，HTTP 繼續聽，手機不斷線，匣圖示還在
- 左鍵點匣、或再執行一次 exe：叫回同一個視窗
- 結束：匣選單「結束」，或 Ctrl+Q，然後才 `Shutdown`
