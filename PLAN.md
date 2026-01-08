# Plex-qBittorrent Bandwidth Manager

## Problem Statement

When running both Plex Media Server and qBittorrent on a network with limited upload bandwidth, streaming users experience buffering when qBittorrent consumes upload bandwidth. This service:

- **Detects** when Plex is actively streaming to remote clients
- **Reduces** qBittorrent upload speed during active streams
- **Restores** qBittorrent upload speed when Plex is idle

---

## Design Decisions

**Direct polling vs Tautulli webhooks:** Most existing solutions (qbittorrent_throttle, qbit-unraid-slowdown) use Tautulli as middleware to receive playback webhooks. We poll Plex directly to avoid the extra dependency. Trade-off: slightly more API calls, but simpler deployment.

**Hysteresis:** Existing solutions toggle immediately on each event, which can cause rapid on/off cycling. We add configurable thresholds to require N consecutive checks before state changes.

**Single binary:** Go compiles to a static binary with no runtime dependencies, unlike Python solutions that require pip packages.

---

## Architecture

```
┌─────────────────┐     Poll every N seconds     ┌─────────────────┐
│   Plex Server   │◄────────────────────────────►│                 │
│                 │                              │   plex-helper   │
└─────────────────┘                              │                 │
                                                 │                 │
┌─────────────────┐     Set upload limit         │                 │
│  qBittorrent    │◄────────────────────────────►│                 │
└─────────────────┘                              └─────────────────┘
```

**State machine:** `idle` ↔ `streaming`

- When remote streams detected → set low upload limit
- When no remote streams → restore normal upload limit
- Hysteresis prevents rapid toggling (configurable thresholds)

---

## Project Structure

```
plex-helper/
├── main.go           # Entry point, CLI, main loop, state machine
├── config.go         # Configuration loading
├── plex.go           # Plex API client
├── qbittorrent.go    # qBittorrent API client
├── telegram.go       # Telegram notification client
├── config.example.json
├── go.mod
└── README.md
```

Five source files in root. No external dependencies beyond the standard library.

---

## Module Responsibilities

### `main.go`
- Parse CLI flags: `--config`, `--dry-run`, `--once`, `--verbose`
- Main polling loop with sleep interval
- State machine: track `idle`/`streaming` state with hysteresis counters
- Signal handling for graceful shutdown

### `config.go`
- Load configuration from JSON file
- Environment variable overrides (PLEX_TOKEN, QBITTORRENT_PASSWORD)
- Validation and defaults

### `plex.go`
- `GetRemoteStreamCount()` → count of remote playing streams
- HTTP GET to `/status/sessions` with `X-Plex-Token` header
- Remote detection: `Session.location == "wan"` OR `Player.local == false`
- Only count streams where `Player.state == "playing"`

### `qbittorrent.go`
- `Login()` → authenticate, store SID cookie
- `SetUploadLimit(bytesPerSec)` → set global limit (0 = unlimited)
- `Referer` header required for CSRF protection
- Auto re-login on 403 response

### `telegram.go`
- `NewTelegramClient(token, chatID)` → create client (returns nil if unconfigured)
- `SendMessage(text)` → POST to Telegram Bot API
- Uses `parse_mode: Markdown` for formatting
- Errors logged but never fatal (notifications are best-effort)

---

## Configuration

```json
{
  "plex_url": "http://192.168.1.10:32400",
  "plex_token": "your-plex-token",
  "qbittorrent_url": "http://192.168.1.10:8080",
  "qbittorrent_username": "admin",
  "qbittorrent_password": "password",
  "idle_upload_kbps": 0,
  "streaming_upload_kbps": 500,
  "poll_interval_sec": 10,
  "streaming_threshold": 2,
  "idle_threshold": 3,
  "telegram_bot_token": "",
  "telegram_chat_id": ""
}
```

| Field | Description |
|-------|-------------|
| `plex_url` | Plex server URL |
| `plex_token` | Plex authentication token (can use PLEX_TOKEN env var) |
| `qbittorrent_url` | qBittorrent Web UI URL |
| `qbittorrent_username` | qBittorrent username |
| `qbittorrent_password` | qBittorrent password (can use QBITTORRENT_PASSWORD env var) |
| `idle_upload_kbps` | Upload limit when no streams, KB/s (0 = unlimited) |
| `streaming_upload_kbps` | Upload limit during streams, KB/s |
| `poll_interval_sec` | How often to check Plex (seconds) |
| `streaming_threshold` | Consecutive checks with streams before throttling |
| `idle_threshold` | Consecutive checks without streams before restoring |
| `telegram_bot_token` | Telegram bot token from @BotFather (can use TELEGRAM_BOT_TOKEN env var) |
| `telegram_chat_id` | Telegram chat ID for notifications (can use TELEGRAM_CHAT_ID env var) |

---

## CLI Usage

```bash
# Run with config file
./plex-helper --config config.json

# Single check (don't loop)
./plex-helper --config config.json --once

# Dry run (log only, don't change limits)
./plex-helper --config config.json --dry-run

# Verbose output
./plex-helper --config config.json --verbose
```

---

## API Reference

### Plex: GET `/status/sessions`

**Headers:**
- `X-Plex-Token: <token>`
- `Accept: application/json`

**Remote stream detection:**
- `Session.location == "wan"` OR `Player.local == false`
- `Player.state == "playing"` (ignore paused/buffering)

### qBittorrent: POST `/api/v2/auth/login`

**Form data:** `username`, `password`
**Headers:** `Referer: <qbittorrent_url>`
**Response:** Sets `SID` cookie

### qBittorrent: POST `/api/v2/transfer/setUploadLimit`

**Headers:**
- `Referer: <qbittorrent_url>` (CSRF)
- `Cookie: SID=<session_id>`

**Form data:** `limit` (bytes/sec, 0 = unlimited)

---

## Deployment

### Systemd Service

```ini
[Unit]
Description=Plex-qBittorrent Bandwidth Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/plex-helper --config /etc/plex-helper/config.json
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

---

## Telegram Notifications

Optional notifications when state changes occur.

### Setup

1. **Create a bot:** Message [@BotFather](https://t.me/BotFather) on Telegram
   - Send `/newbot` and follow prompts
   - Save the bot token (format: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

2. **Get your chat ID:**
   - Send any message to your new bot
   - Visit `https://api.telegram.org/bot<TOKEN>/getUpdates`
   - Find `"chat":{"id":123456789}` in the response

3. **Configure:** Add to config.json:
   ```json
   {
     "telegram_bot_token": "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
     "telegram_chat_id": "123456789"
   }
   ```

### Implementation

**New file: `telegram.go`**
- `TelegramClient` struct with bot token and chat ID
- `SendMessage(text string)` → POST to Telegram API
- Markdown formatting support for rich messages

**API endpoint:** `POST https://api.telegram.org/bot<token>/sendMessage`

**Request body:**
```json
{
  "chat_id": "123456789",
  "text": "🔴 *Streaming detected*\nThrottling upload to 500 KB/s",
  "parse_mode": "Markdown",
  "disable_notification": false
}
```

**Integration in `main.go`:**
- On state change `idle → streaming`: Send throttle notification
- On state change `streaming → idle`: Send restore notification
- Notification failures logged but don't block state changes

### Message Format

```
🔴 Streaming detected
Throttling upload to 500 KB/s

🟢 Streaming ended
Restoring upload to unlimited
```

### Config Fields

| Field | Description |
|-------|-------------|
| `telegram_bot_token` | Bot token from @BotFather (can use TELEGRAM_BOT_TOKEN env var) |
| `telegram_chat_id` | Your chat ID (can use TELEGRAM_CHAT_ID env var) |

Notifications are disabled if either field is empty.

---

## API Documentation

See `docs/` for detailed API documentation:
- `docs/plex_api_summary.md` - Plex session detection
- `docs/qbittorrent_api.md` - qBittorrent Web API
