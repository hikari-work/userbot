# USERBOT - Telegram Userbot in Go

A powerful Telegram userbot written in Go using [gotd](https://github.com/gotd/td) and [gotgproto](https://github.com/celestix/gotgproto).

## Deployment

### Method 1: Docker (Recommended & Lightweight)

1. **Configure Environment**
   ```bash
   cp .env.sample .env
   # Open .env and fill in your API_ID, API_HASH, PHONE_NUMBER, etc.
   ```

2. **Run with Docker Compose**
   ```bash
   docker compose up -d
   ```
   *This will pull the precompiled lightweight image from GitHub Container Registry (GHCR), spin up Redis, and run the userbot. Session files will be persisted in `./sessions` and downloads in `./downloads`.*

3. **Update the Bot**
   ```bash
   docker compose pull
   docker compose up -d
   ```

### Method 2: Local Manual Build

1. **Configure Environment**
   ```bash
   cp .env.sample .env
   # Open .env and fill in your API_ID, API_HASH, PHONE_NUMBER, etc.
   ```

2. **Build and Run**
   ```bash
   go mod tidy
   go build -o userbot
   ./userbot
   ```

## Requirements & Libraries

### System Binaries & Tools
- **Go** (version 1.20 or newer)
- **Redis Server** (for prefix, anti-flood state, and whitelist caching)
- **FFmpeg** (required for the Voice Chat module to stream audio)
- **yt-dlp** (required for Voice Chat audio extraction and media downloader)

### Core Go Libraries
- **[gotd](https://github.com/gotd/td)** - Pure Go MTProto client implementation
- **[gotgproto](https://github.com/celestix/gotgproto)** - Easy helper wrapper library for gotd
- **[go-redis](https://github.com/redis/go-redis)** - Redis database client
- **[pion-webrtc](https://github.com/pion/webrtc)** - WebRTC implementation for voice chat streaming
- **[sqlite](https://github.com/glebarez/sqlite)** - SQLite database driver for session storage
- **[zap](https://github.com/uber-go/zap)** - Logging framework

## Developer Guide

To learn how to create and add new commands/modules, see [adding_module.md](./adding_module.md).
