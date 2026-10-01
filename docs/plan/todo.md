# 工作清單 (Todo)

> 整體方向見 `docs/plan/roadmap.md`。已完成的項目請移至 `docs/plan/done/`。

---

## 🚀 待執行項目
- [ ] 手機實機驗證：TG WebView（iOS／Android）📎 檔案選擇器、佇列 UI
- [ ] 驗證 Codex／Cursor／Kiro 能以絕對路徑讀取上傳的圖片（Claude 的 Read 工具可讀圖）
- [ ] （另案）`internal/static/uploads` 掛在未驗證的 `app.Static` 下，agent 截圖知道 URL 即可讀

## ✅ 本輪完成（2026-09-30）
- [x] 訊息佇列：執行中送出的訊息排隊、完成後自動續跑、失敗／中斷／拒絕授權暫停（存 DB）
- [x] MCP envelope hop 計數（預設上限 5，`mcp_max_hops` 可設）
- [x] MCP `ask_session`：阻塞等回覆 + timeout
- [x] 上傳圖片／檔案給 agent（存 runtime 目錄的 `workspace/uploads/<session_id>/`，不污染專案 `work_dir`）

---

## 📈 進階規劃
- [x] 桌面視窗殼第一版（Wails，見 `docs/plan/todo/desktop-window.md`）
- [x] 桌面版系統匣：關視窗後從匣裡叫回／結束（見 `docs/plan/todo/desktop-window.md`）
- [ ] 支援更多 AI 工具 (例如 OpenAI o1, deepseek 等，若有 CLI)
- [ ] 檔案總管功能 (瀏覽 work_dir 檔案)
- [ ] 系統資源監控 (CPU/Memory 狀態)
- [x] Kiro ACP：前端已重新啟用（需 kiro-cli >= 2.16.0；`session/load` PoC 通過，見 `poc/kiro-cli/acp_same_cwd_resume_poc.js`／`docs/plan/done/kiro-output-markdown-and-acp.md`）
- [x] Session @mention + MCP 諮詢署名（見 `docs/plan/done/session-mention-mcp.md`）
