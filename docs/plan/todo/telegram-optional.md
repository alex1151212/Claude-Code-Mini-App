# 功能計劃：Telegram 改為可選驗證／通知模組

> 狀態：規劃中（尚未實作）
> 建立日期：2026-10-05
> 目標：不設定 `bot_token` 也能正常啟動並使用（靠反向代理 + 內網 IP 白名單 + web 密碼），Telegram 退化為「可選的登入方式與通知管道」。不重構身分模型。

## 1. 現況盤點

`tg_id` 實際只有三種用途，核心資料（sessions／messages）沒有以 `tg_id` 當擁有者：
1. **白名單**：`users(tg_id, username)`，`IsUserAllowed` 只回答「准不准進」（`internal/db/user.go`）。
2. **通知目標**：`ws.NewHandler(..., botToken, ...)`；`tg.Notify`／`NotifyTask` 與 `ws/notify.go` 已有 `botToken == "" || chatID == 0` 即略過的判斷。
3. **web session 綁定**：`auth.Store.Create(tgID)`，登入時綁定通知對象，0 代表不通知。

已經可用的部分：
- 前端 `core.js`：沒有 `initData` 時 `isTelegram=false`，自動走密碼登入；`window.Telegram?.WebApp` 使用可選鏈，缺少 SDK 不會報錯。
- 多個測試已用空 token 呼叫 `NewHandler`。

阻礙：
| # | 位置 | 問題 |
|---|---|---|
| A | `internal/config/config.go:93` | 無 `bot_token` 且非 `no_auth` 時啟動直接失敗 |
| B | `internal/server/server.go:219-240` | `initData` 分支在 token 為空時仍會用空 key 驗簽，可被偽造 → **必須關閉** |
| C | `internal/auth/ip.go:15` | `RealIP` 無條件信任 `CF-Connecting-IP`／`X-Forwarded-For`；拿掉 TG 後 IP 白名單成為唯一防線，直連即可偽造 |

## 2. 分期

### 2.1 第一期：bot_token 可留空（必做）

- `config.go`：移除 `bot_token` 必填檢查。`no_auth` 維持原義。
- `tg.Verify`：`botToken == ""` 直接回傳錯誤（縱深防禦，不依賴呼叫端）。
- `server.go` `authMiddleware`：`initData != "" && cfg.BotToken != ""` 才進 TG 分支；token 為空時帶 `initData` 的請求**不得放行**，須落到後面的 IP／session 檢查（帶了也沒用，不是直接 401 以外的路徑）。
- 啟動時 `slog.Info` 一行：`Telegram 未設定，僅提供 web 密碼／訪客登入，通知停用`。
- `config.example.yaml`：`bot_token`、`whitelist_tg_ids` 改為註解掉的可選項，並註明留空的行為。

完成標準：
- `bot_token` 留空、`no_auth: false` 可啟動。
- 帶任意偽造的 `X-Telegram-Init-Data`（或 WS `?initData=`）請求 `/sessions`，未登入時得到 401／403，不得 200。
- web 密碼登入後可正常建立 session、送訊息、WS 連線；`tg_id` 為 0 時無 panic、無通知。
- 既有 `go test ./...` 通過。

### 2.2 第二期：RealIP 只信任已知代理（建議與第一期同批，否則 TG 拿掉後白名單可被繞過）

已列於 `todo.md`（共享聊天室對外開放項）。做法：
- 新增 `web.trusted_proxies`（CIDR 清單）。直連來源（`c.IP()`）在清單內才採信 `CF-Connecting-IP`／`X-Forwarded-For`；否則一律用 `c.IP()`。
- 預設值：`127.0.0.0/8`、`::1`（只信任本機代理，如同機的 `cloudflared`），讓現有 Tunnel 部署不用改設定。代理在別台機器時須自行補此 key，否則所有外部請求會被視為該代理的內網 IP（白名單失效，只剩 web 密碼）。

完成標準：
- 直連帶 `X-Forwarded-For: 192.168.1.1` 的外網請求，不會被視為內網。
- 經由設定的代理 IP 來的請求，仍取得真實 client IP。
- `auth` 套件加單元測試覆蓋上述兩點。

### 2.3 第三期：信任上游認證（選做，未決定要不要）

情境：反向代理自己做認證（Cloudflare Access、Authelia 等），app 只需認代理放行。
目前沒有對應模式（`no_auth` 是全開）。若要做：`web.trusted_header`（例如 `Cf-Access-Authenticated-User-Email`）+ `trusted_proxies` 同時成立才放行。
**本期不做**，等第 4 節決策後再決定。

## 3. 不做的事

- 不抽象 `AuthProvider` 介面、不改 `users` 表結構、不把 `tg_id` 改名（只有一種實作，YAGNI；之後真的有第二種身分來源再說）。
- 不動前端：已能在無 Telegram 環境運作。`index.html` 仍載入 `telegram.org` 的 SDK，離線／被擋時載入失敗無影響（已用 `?.` 保護），需實測確認。

## 4. 待決策

1. 沒有 TG 後，要靠「內網 IP 白名單 + web 密碼」，還是需要第 2.3 期的上游認證？
2. `trusted_proxies` 預設值：已傾向 loopback（見 2.2）。若改為空，現有 Tunnel 部署升級後會壞，違反第 6 節契約。
3. 現有使用 TG 登入的部署是否要保證不變？本計劃的設計是向下相容（有 token 時行為不變）。

## 6. config.yaml 契約變更

| key | 變更 | 對現有設定的影響 |
|---|---|---|
| `bot_token` | 必填 → 可選 | 無；留空時以前啟動失敗，現在正常啟動 |
| `whitelist_tg_ids` | 不變（本來就未強制） | 無 |
| `web.trusted_proxies` | **新增**，預設 loopback | 同機代理（`cloudflared`）無需改動；代理在別台機器須補此 key |

其餘 key 不變。除上表外不得新增必填項。

## 5. 風險與驗證

- 最大風險是 B：漏掉任何一個 `initData` 入口（HTTP header、`?initData=` query、WS）。全部都經過 `authMiddleware`，實作後用 grep 確認 `tg.Verify` 只有一個呼叫點。
- 升級行為：2.2 若預設不信任 header，既有走 Cloudflare Tunnel 的部署會全部變成「代理 IP」，需先在 `config.yaml` 補 `trusted_proxies` 才不會被鎖在外面（共享聊天室的訪客走 guest token 不受影響）。
- 手動驗證：用 curl 偽造 header 重現 B、C 兩種繞過，修改前後各跑一次。
