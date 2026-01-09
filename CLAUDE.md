# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

plex-helper is a bandwidth manager that automatically adjusts qBittorrent upload speeds based on Plex streaming activity. When Plex is streaming to remote clients, it reduces qBittorrent's upload limit to prevent buffering. When Plex is idle, it restores normal upload speeds.

## Build Commands

```bash
go build -o plex-helper .    # Build binary
go build .                   # Build (uses module name)
./plex-helper --help         # Show CLI options
```

## Architecture

Event-driven state machine with webhook support and fallback polling:
- `main.go` - CLI, event loop, state transitions
- `config.go` - JSON config loading, env var overrides
- `state.go` - Thread-safe application state
- `server.go` - HTTP server (health endpoint + Plex webhook handler)
- `plex.go` - Plex API client, remote stream detection
- `qbittorrent.go` - qBittorrent API client, cookie auth, CSRF handling
- `telegram.go` - Optional Telegram notifications

## Key API Details

**Plex**: Use `X-Plex-Token` header and `Accept: application/json`. Check `Session.location` ("lan"/"wan") and `Player.local` (bool) to identify remote streams.

**qBittorrent**: Requires `Referer` header matching the request URL for CSRF protection. Login returns `SID` cookie. Speed limits are in bytes/sec (0 = unlimited).

## API Documentation

Detailed API docs are in `docs/`:
- `docs/plex_api_summary.md` - Plex session detection and authentication
- `docs/qbittorrent_api.md` - qBittorrent Web API and CSRF requirements
