# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

New API is an LLM gateway and AI asset management system written in Go (backend) with a React frontend. It provides a unified API interface for 40+ AI providers (OpenAI, Claude, Gemini, etc.) with user management, billing, and monitoring.

## Build Commands

### Development
```bash
# Backend only (Go)
go run main.go

# Frontend only (in web/ directory)
cd web && bun install && bun run dev

# Full build (frontend + backend)
make all
```

### Frontend
```bash
cd web
bun install                    # Install dependencies
bun run dev                    # Dev server with HMR
bun run build                  # Production build
bun run lint                   # Check formatting (prettier)
bun run lint:fix               # Fix formatting
bun run eslint                 # Run ESLint
```

### Docker
```bash
docker-compose up -d           # Start with PostgreSQL + Redis
docker build -t new-api .      # Build image
```

## Architecture

### Request Flow
```
Client → Router (Gin) → Middleware (auth, rate-limit) → Controller → Service → Relay → External AI Provider
```

### Key Directories

- **`/controller`** - HTTP request handlers for REST API endpoints
- **`/relay`** - AI provider integration layer with format conversion
  - `/relay/channel/` - Individual provider adapters (openai/, claude/, gemini/, etc.)
- **`/model`** - GORM database models (channel, user, token, task, pricing)
- **`/service`** - Business logic (quota calculation, token counting, channel selection)
- **`/middleware`** - HTTP middleware (auth, rate limiting, CORS)
- **`/router`** - Route configuration (api-router.go, relay-router.go)
- **`/common`** - Shared utilities and constants
- **`/dto`** - Data transfer objects for request/response
- **`/setting`** - Configuration management (pricing ratios, model settings)
- **`/web`** - React frontend (Vite + Semi UI + Tailwind)

### Provider Adapters

Adding a new AI provider requires:
1. Create adapter in `/relay/channel/<provider>/`
2. Implement the adaptor interface with request/response conversion
3. Register in `/relay/relay_adaptor.go`
4. Add channel type constant in `/constant/`

### Format Conversion

The relay layer handles automatic format conversion:
- OpenAI ↔ Claude Messages format
- OpenAI ↔ Gemini format
- Streaming SSE handling across providers

Key files: `/service/convert.go`, `/relay/*_handler.go`

### Database

Supports SQLite (default), MySQL (≥5.7.8), PostgreSQL (≥9.6). Models use GORM with caching layer.

Key models:
- `channel` - API provider configurations with keys
- `user` - User accounts with quotas
- `token` - API access tokens
- `task` - Async task tracking (Midjourney, Suno, video generation)
- `pricing` - Per-model pricing configuration

### Billing System

- Pre-consumption quota validation before requests
- Token counting for input/output (including images/audio)
- Cache billing support for providers with cached tokens
- Key files: `/service/quota.go`, `/service/token_counter.go`

## Environment Variables

Key configuration (see `.env.example`):
- `SQL_DSN` - Database connection string
- `REDIS_CONN_STRING` - Redis for caching
- `SESSION_SECRET` - Required for multi-instance deployment
- `CRYPTO_SECRET` - Required when using Redis
- `STREAMING_TIMEOUT` - Stream response timeout (default: 300s)

## Frontend Notes

- Uses Semi Design System (`@douyinfe/semi-ui`)
- i18n with `react-i18next` - translations in `web/src/i18n/`
- API client in `web/src/services/`
- Dev server proxies to backend on port 3000
