# 功能計畫書：Codex headless CLI 整合更新（對齊 0.159.x）

> 狀態：已完成（2026-09-30）
> 建立日期：2026-09-30
> 取代方向：暫不做 `codex-acp-migration.md`（ACP 需 Node 轉譯層，僅在需要中途授權時再評估）
> 關聯：`internal/codex/`、`internal/quota/codex.go`、`docs/spec/codex-cli.md`

---

## 1. 實測結果（codex-cli 0.159.2，2026-09-30，Windows）

| # | 測試 | 結果 |
|---|---|---|
| T1 | 現行新 thread 參數 `exec --json --skip-git-repo-check --yolo -C <dir>` | ✅ exit 0；`--yolo` 仍為**隱藏別名**（help 已不列） |
| T2 | `--dangerously-bypass-approvals-and-sandbox` + 指令執行 | ✅ 正常；`command_execution` item 帶 `command` / `aggregated_output` / `exit_code` / `status` |
| T3 | 現行 resume 參數 `exec resume <id> --json --skip-git-repo-check --yolo` | ✅ 上下文正確續接，`thread.started` 回傳同一 thread_id；resume 仍**不支援 `-C`** |
| T4 | `-m <model> -c model_reasoning_effort=low` | ✅ 正常 |
| T5 | 無效 model | ⚠️ 見 2.2：錯誤重複且訊息為巢狀 JSON 字串 |
| T6 | 現行 quota prompt | ❌ 見 2.1：模型回「unavailable」，每次耗 ~38k tokens |
| — | `~/.codex/models_cache.json` | ✅ 結構未變（`models[].slug` / `display_name`） |
| — | 事件頂層型別 | ✅ 未變：`thread.started` / `turn.started` / `item.started` / `item.completed` / `turn.completed` / `turn.failed` / `error` |
| — | `turn.completed.usage` | 新增 `cache_write_input_tokens`（無害，未映射） |
| — | stderr | 即使 stdin 關閉仍印 `Reading additional input from stdin...` |
| — | 既有測試 `go test ./internal/codex/... ./internal/quota/...` | ✅ PASS |

**結論：runner 主流程在新版仍可用，沒有壞。** 真正的問題是 quota 失效，以及幾個體驗瑕疵。

## 2. 發現的問題

### 2.1 Quota fetch 失效且浪費額度（最高優先）

`CodexFetcher.Fetch()` 是叫模型「回報你的用量」，實測模型無法取得，回覆 `5-hour usage: unavailable`，regex 對不到 → fallback 到 `FromCodexTurnUsage`，UI 顯示成 `Tokens 38392`（這次 fetch 本身消耗的 token），毫無意義，且每次 fetch 都真的花一輪模型額度。

**可靠來源已找到**：codex 每回合都會把 `rate_limits` 寫進本機 rollout 檔
`~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<thread_id>.jsonl`，`type=event_msg` / `payload.type=token_count`：

```json
"rate_limits": {
  "primary":   {"used_percent": 2.0, "window_minutes": 300,   "resets_at": 1790767809},
  "secondary": {"used_percent": 0.0, "window_minutes": 10080, "resets_at": 1791354609},
  "plan_type": "plus"
}
```

primary = 5 小時窗、secondary = 週窗，直接對應現有 `formatCodexDisplay` 的 `session` / `weekly`。讀檔零成本、不需呼叫模型。

### 2.2 錯誤事件重複且不可讀

無效 model 時事件序列為：`item.completed(type=error)` → `error` → `turn.failed` → exit 1。現行 runner：
- `error` 與 `turn.failed` 各發一次 `EventError`（WS 廣播兩次）
- 因未收到 `turn.completed`，再以 stderr 組一次錯誤，而 stderr 只有 `Reading additional input from stdin...`，得到誤導訊息
- `message` 本身是巢狀 JSON 字串（`{"type":"error","status":400,"error":{"message":"..."}}`），`PreferErrorText` 取較長者，最終 UI 顯示原始 JSON

### 2.3 多段 agent_message 黏在一起

T2 中模型先回 `I’ll run the command.`，再回 `probe123`，兩段各發 `EventDelta` 且無分隔，前端會顯示 `I’ll run the command.probe123`。

### 2.4 使用隱藏別名 `--yolo`

目前可用，但已從 help 移除，未來版本移除時會直接壞掉。

### 2.5 文件過時

`docs/spec/codex-cli.md` 記錄的是 0.142.5 的行為（例如 `approval_policy` 說明、quota 以 status prompt 取得）。

## 3. 實作任務

### Phase 1 — Quota 改讀 rollout 檔（2.1）

- [x] `internal/quota/codex.go`：`Fetch()` 改讀 `$CODEX_HOME/sessions`（預設 `~/.codex/sessions`）最新含 `rate_limits` 的 rollout 檔，取最後一筆，**不再呼叫模型**
  - 偏離原計畫：原打算只看今天/昨天目錄，但 resume 舊 thread 會寫回建立當天的目錄，改為全樹 `WalkDir` 依 mtime 排序（ponytail 註解記錄上限：檔案數達數萬再改）
  - 依 `window_minutes` 分類（≥ 1 週為 weekly）而非 primary/secondary 位置；`resets_at` 已過視為 0%
- [x] 移除 `parseCodexQuotaJSONL` / `FromCodexStatusText` / `FromCodexTurnUsage` / `formatCodexDisplay` 的 tokens 分支及對應測試
- [x] 支援 `CODEX_HOME`
- [x] 測試：`usage.TestFromCodexRolloutLine`（含重置歸零、null）、`quota.TestLatestCodexRateLimits`（跨日期目錄 mtime、略過無 rate_limits 檔、不完整尾行）
- [x] `agent.TypeCodex` 加入 `Service.GetAll()` / `Warmup()`

### Phase 2 — 錯誤處理（2.2）

- [x] `events.go` `ErrorMessage()`：解開巢狀 API 錯誤 JSON（`unwrapAPIError`）
- [x] `runner.go`：`error` 事件只記錄，`turn.failed` 或結束時未見 `turn.completed` 才回報，`dispatchState.emitError` 保證每回合一次
- [x] `classifyRunnerError()`：過濾 stdin 提示
- [x] 測試：T5 實測序列、可恢復錯誤後成功、stderr fallback

### Phase 3 — 小修（2.3、2.4）

- [x] 第二段起的 `agent_message` 前補 `\n\n`
- [x] `--yolo` → `--dangerously-bypass-approvals-and-sandbox`（新 thread 與 resume；quota 已不再 spawn）

### Phase 4 — 文件（2.5）

- [x] `docs/spec/codex-cli.md` 實測結論更新為 0.159.2
- [x] `docs/plan/todo/codex-acp-migration.md` 標註暫緩

## 4. 不在範圍內（可另開）

- `command_execution` 映射成 `EventToolStarted` / `EventToolCompleted`（顯示指令與輸出；目前只有「執行指令中…」）
- 中途授權（需 ACP 或 app-server，見 `codex-acp-migration.md`）
- 前端 `useChatSocket` 未處理 WS `error` 訊息（所有 agent 共通；錯誤目前只見於 TG 通知與 log）

## 5. 驗收（2026-09-30 實機 codex-cli 0.159.2）

- [x] Quota：`5h 2% · Week 0%`，fetch 20ms，不 spawn codex；跑完兩輪後讀到 `5h 4% · Week 1%`
- [x] 無效 model：只有 1 個 EventError，內容 `The 'no-such-model-xyz' model is not supported when using Codex with a ChatGPT account.`
- [x] 含工具呼叫的回覆分段：`Running now.\n\n\nprobe123`
- [x] `go test ./internal/...` 全數 PASS
- [x] 新 thread + resume 正常（resume 正確回答 `probe123`）
