# Claude Code Mini App

> A self-hosted **multi-agent console** for Claude Code, Cursor Agent, Codex and Kiro. Run and supervise all your coding agents in one place — from a desktop window, any browser, or a Telegram Mini App on your phone. **One Go binary**, no separate frontend build.

[![Version](https://img.shields.io/badge/version-0.7.0-blue)](#) [![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](#)

[繁體中文](README.zh-TW.md)

![Agent Console: sessions for Claude, Codex, Kiro and Cursor grouped by project, with a streaming chat on the right](docs/images/console.png)

<sub>Demo data on a throwaway instance. The UI is shown in Traditional Chinese.</sub>

Your agents keep running on the machine where your code lives; this app is the console in front of them. Each conversation is a **session** bound to a `work_dir`, an agent, and a permission mode, so you can juggle several agents across several projects, see which ones are working or waiting for approval, and step in from wherever you are.

| Way in | For | Auth |
|---|---|---|
| **Desktop window** (Windows) | Daily use at your PC — tray icon, Ctrl+P switcher | local |
| **Browser** | Any device on your LAN, or remote through a reverse proxy / tunnel | web password (restricted to `web.allowed_cidrs`) |
| **Telegram Mini App** | Away from your desk — phone push on errors, approvals on the go | Telegram `initData` + allowlist |

All three talk to the same server and see the same sessions in real time.

## Quick Start

**Requires:** the CLI(s) you use (`claude`, `cursor agent`, `codex`, `kiro-cli`) installed and logged in on the machine, plus a Telegram bot token ([@BotFather](https://t.me/BotFather)) — or `no_auth: true` for local-only use.

**Download (recommended):** grab the zip / tar.gz from [Releases](https://github.com/jerry12122/Claude-Code-Mini-App/releases/latest), extract the whole folder (the executable needs `internal/static/` beside it), copy `config.example.yaml` to `config.yaml`, then run `claude-miniapp-desktop.exe` (Windows desktop) or `claude-miniapp` (server only).

**From source** (Go 1.25+):

```bash
git clone https://github.com/jerry12122/Claude-Code-Mini-App
cd Claude-Code-Mini-App
go build -o claude-miniapp ./cmd/server
cp config.example.yaml config.yaml   # set bot_token, whitelist_tg_ids
./claude-miniapp                     # → http://localhost:8080
```

## Desktop window (Windows)

Same process, plus a window that loads the existing UI. The browser and Telegram Mini App keep using the same port. Closing the window hides it and leaves a tray icon: left-click shows the window, right-click offers Show or Quit. Ctrl+Q quits as well. F5 / Ctrl+R reloads the UI and F12 opens DevTools (a no-op with `-tags production`).

Win11 already includes the WebView2 runtime. `go build ./cmd/server` stays pure Go. The desktop build uses Wails v3 (the tray is built in):

```bash
go build -ldflags "-H windowsgui" -o claude-miniapp-desktop.exe ./cmd/desktop
```

Put the exe next to `config.yaml` and `internal/static/` (same layout as the server binary). From a repo checkout, `go run ./cmd/desktop` uses the working directory's config. Add `-tags production` to turn off Wails debug logging.

Frontend files are served from disk, so editing `internal/static/` only needs a reload (F5), not a rebuild. If a change still doesn't show up, use Settings → General → "Clear cache & reload".

## Features

- **Multi-agent** — Claude Code, Cursor Agent, Codex, Kiro ACP (interactive permission prompts over Agent Client Protocol); per session. Existing Kiro CLI sessions keep working, but new ones use Kiro ACP (Gemini / Antigravity paused due to headless limits). Model lists are fetched from each CLI on startup (Claude included) and can be refreshed without a restart from Settings → General
- **Live streaming** — WebSocket chat with Markdown; multi-tab sync
- **Code blocks** — syntax highlighting with language tag, one-click copy
- **Quota badge** — Session header shows usage (e.g. Claude `5h 16% · Week 9%`)
- **Sessions** — Multiple conversations, each with its own agent, `work_dir` and permission mode
- **Temporary sharing** — Share a session with a friend via link + 6-digit PIN + nickname (default 1 hour, up to 7 days). Guests join from any browser as read-only `viewer` or full-access `editor`; messages show who said what, and ending the share kicks everyone out at once
- **Message queue** — Messages sent while a task is running are queued (persisted in the DB, survives restarts) and run in order; the queue pauses on failure or interrupt, and can be resumed or edited
- **Quick switcher** — Ctrl/Cmd+P opens a VS Code-style palette to jump to any session (search by name, directory, or branch)
- **Attachments** — 📎 upload or paste images / text files; they show as removable chips (with thumbnails) above the input and are handed to the agent by path (stored under `workspace/uploads/`, not your `work_dir`); drag & drop on desktop
- **Unread tracking** — Sessions with new activity are marked unread in the list; "read all" to clear. The list updates in real time over a `/events` WebSocket (30 s polling as a fallback); the tab title shows the unread count, and a toast appears when another session finishes or needs approval
- **Log viewer** — Settings → Logs streams the server log live (level filter, search, pause, copy; toggle Debug at runtime). Handy for the desktop build, which has no console; only the log since the current start is shown, the full history stays in `logs/server.log`
- **Message actions** — Copy / forward buttons sit beside each message's timestamp; long-press a message for a floating menu (copy, plus forward on incoming messages)
- **Chat UX** — IME-safe Enter, Enter inserts a newline on touch devices, scrolling up isn't yanked back by streaming (with a "jump to latest" button), deleted sessions can be undone for 5 seconds
- **Permissions** — Claude denial flow shows the full command / file content; Kiro ACP mid-turn approval; approve once, or (for edit tools only) allow and auto-accept edits
- **Auth** — Telegram `initData` + allowlist; optional web login on private IPs
- **Optional shell** — Run commands in `work_dir` (off by default); when on, also shows "Open in VS Code" / "Open folder" buttons in the session header (desktop only)
- **MCP server** — Expose sessions to other agents over Streamable HTTP (`POST /mcp`, off by default): operate sessions, read chat history, and query cross-session activity; `ask_session` asks another session and waits for its answer, with a hop limit (`mcp_max_hops`) to stop agent-to-agent loops

## Why this?

| | SSH + tmux | Generic Telegram bot | **This app** |
|---|---|---|---|
| Multiple agents, one view | One terminal each | One bot, one tool | Claude / Cursor / Codex / Kiro ACP side by side |
| Mobile UX | Poor | Text-only | Mini App UI + streaming |
| Session / `work_dir` | Manual | Usually none | Built-in, persisted, unread + approval badges |
| Collaboration | Share a terminal | None | Temporary PIN link, viewer / editor |
| Agent-to-agent | You wire it | None | Built-in MCP server (`ask_session`) |
| Deploy | SSH keys | Bot + custom code | Single binary |

## Architecture

```
Desktop window · Browser · Telegram Mini App · Share guests
        ↕ WebSocket / REST          MCP clients ↔ POST /mcp
┌─────────────────────────────────┐
│   Go binary (Fiber + SQLite)    │
│   Sessions · queue · sharing    │
│   spawn CLI per message (no PTY)│
│   QuotaService (cached fetch)   │
└────────────────┬────────────────┘
                 ↓
  claude · cursor agent · codex · kiro-cli (ACP)
```

Each user message spawns a short-lived subprocess. Details: [`docs/spec/plan.md`](docs/spec/plan.md), [`docs/spec/headless.md`](docs/spec/headless.md).

## Security

- Keep real secrets out of git; never use `no_auth` in production.
- **`shell.enabled`** grants shell access on the host to authenticated users — enable only on trusted networks. Allowlist rules: [`docs/spec/shell-allowlist-schema.md`](docs/spec/shell-allowlist-schema.md).
- **Shares** — anyone holding the link and PIN can join until it expires or you end it; an `editor` guest has the same power as you (sending messages, approving tool use), so prefer `viewer` unless you trust them.
- **`mcp_token`** grants full session control (including shell) to any client holding it — treat it like `bot_token` and only expose `/mcp` on trusted networks.
- **Uploads** (`POST /sessions/:id/uploads`) let any authenticated user write files of any type and size (no size, count or extension limits; server-generated names, empty files rejected) into `./workspace/uploads/<session_id>/` beside the runtime (not into your project's `work_dir`), independent of `shell.enabled`.

## Documentation

| Topic | Path |
|---|---|
| Spec & API / WebSocket | [`docs/spec/plan.md`](docs/spec/plan.md) |
| Config reference | [`config.example.yaml`](config.example.yaml) |
| Claude / Cursor / Codex / Kiro / Antigravity CLI | [`docs/spec/`](docs/spec/) |

> `poc/` (probe scripts, one-off samples) is local investigation scratch, not tracked in the repo.

## Testing

This project is tested with BrowserStack.

## License

MIT
