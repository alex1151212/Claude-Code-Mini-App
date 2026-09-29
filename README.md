# Claude Code Mini App

> Remote AI coding CLIs from your phone via Telegram. **One Go binary** — REST, WebSocket, and UI, no separate frontend build.

[![Version](https://img.shields.io/badge/version-0.4.0-blue)](#) [![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](#)

[繁體中文](README.zh-TW.md)

## Quick Start

**Requires:** Go 1.25+, Telegram bot token ([@BotFather](https://t.me/BotFather)), and the CLI(s) you use (`claude`, `cursor agent`, `kiro-cli`) installed on the server.

```bash
git clone https://github.com/jerry12122/Claude-Code-Mini-App
cd claude-miniapp
go build -o claude-miniapp ./cmd/server
cp config.example.yaml config.yaml   # set bot_token, whitelist_tg_ids
./claude-miniapp                     # → http://localhost:8080
```

## Features

- **Multi-agent** — Claude Code, Cursor Agent, Kiro CLI, Kiro ACP (interactive permission prompts over Agent Client Protocol); per session (Gemini / Antigravity paused due to headless limits)
- **Live streaming** — WebSocket chat with Markdown; multi-tab sync
- **Code blocks** — syntax highlighting with language tag, one-click copy
- **Quota badge** — Session header shows usage (e.g. Claude `5h 16% · Week 9%`)
- **Sessions** — Multiple conversations, each with its own `work_dir` and permission mode
- **Unread tracking** — Sessions with new activity are marked unread in the list; "read all" to clear
- **Permissions** — Claude denial flow; Kiro ACP mid-turn approval; approve once or switch mode from the UI
- **Auth** — Telegram `initData` + allowlist; optional web login on private IPs
- **Optional shell** — Run commands in `work_dir` (off by default); when on, also shows "Open in VS Code" / "Open folder" buttons in the session header (desktop only)
- **MCP server** — Expose sessions to other agents over Streamable HTTP (`POST /mcp`, off by default): operate sessions, read chat history, and query cross-session activity

## Why this?

| | SSH + terminal | Generic Telegram bot | **This app** |
|---|---|---|---|
| Mobile UX | Poor | Text-only | Mini App UI + streaming |
| Session / `work_dir` | Manual | Usually none | Built-in, persisted |
| Multi CLI | You wire it | One bot, one tool | Claude / Cursor / Kiro / Kiro ACP |
| Deploy | SSH keys | Bot + custom code | Single binary |

## Architecture

```
Telegram Mini App / browser
        ↕ WebSocket
┌──────────────────────────────┐
│  Go binary (Fiber + SQLite)  │
│  spawn CLI per message (no PTY) │
│  QuotaService (cached fetch) │
└──────────────────────────────┘
```

Each user message spawns a short-lived subprocess. Details: [`docs/spec/plan.md`](docs/spec/plan.md), [`docs/spec/headless.md`](docs/spec/headless.md).

## Security

- Keep real secrets out of git; never use `no_auth` in production.
- **`shell.enabled`** grants shell access on the host to authenticated users — enable only on trusted networks. Allowlist rules: [`docs/spec/shell-allowlist-schema.md`](docs/spec/shell-allowlist-schema.md).
- **`mcp_token`** grants full session control (including shell) to any client holding it — treat it like `bot_token` and only expose `/mcp` on trusted networks.

## Documentation

| Topic | Path |
|---|---|
| Spec & API / WebSocket | [`docs/spec/plan.md`](docs/spec/plan.md) |
| Config reference | [`config.example.yaml`](config.example.yaml) |
| Claude / Cursor / Kiro / Antigravity CLI | [`docs/spec/`](docs/spec/) |

> `poc/` (probe scripts, one-off samples) is local investigation scratch, not tracked in the repo.

## License

MIT
