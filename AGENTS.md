# ToxicBot

A Go-based Telegram bot that trolls users in group chats. Combines text generation via list-based selection and LLM (DeepSeek), reacts to messages, stickers, voice messages, and chat events.

## Tech Stack

- **Language**: Go
- **Telegram framework**: `gopkg.in/telebot.v3`
- **Database**: SQLite (`jmoiron/sqlx`, migrations via `golang-migrate/migrate/v4`)
- **Configuration**: `kelseyhightower/envconfig`
- **AI**: DeepSeek API
- **Data sources**: Google Sheets (messages, stickers, voice messages, nicknames)
- **Logging**: `sirupsen/logrus`

## Project Structure

```text
cmd/main.go                              — entry point, dependency injection
db/migrations/                           — SQLite SQL migrations
internal/
  chatsettings/provider.go               — chat settings provider with cache (1 min TTL)
  config/config.go                       — env-based configuration
  domain/chat/settings.go                — ChatSettings domain model
  features/stats/                        — statistics tracking with AES encryption
  handlers/
    handlers.go                          — handler dispatcher (parallel execution)
    bulling/                             — main trolling handler (text responses)
    on_sticker/                          — sticker reaction handler
    on_voice/                            — voice message reaction handler
    on_user_join/                        — new member greeting handler
    on_user_left/                        — member leave reaction handler
    personal/                            — per-user reactions (Igor, Max, Kirill)
    tagger/                              — periodic random user tagger
    settings/                            — /settings command
    stat/                                — /stat command
  infrastructure/
    ai/deepseek/                         — DeepSeek LLM integration
    sheets/                              — Google Sheets data sources
    storage/db/                          — storage layer (SQLite)
  message/                               — message generation engine
  phrase_filter/                         — meaningfulness filter for AI
  usecase/                               — business logic
pkg/                                     — shared utilities (logger, migrator, mapper)
deploy/                                  — Kubernetes manifests
```

## Handlers and Features

### Bulling (`internal/handlers/bulling/`)

Main trolling mechanism. Tracks user message count via a circular list. Triggers when a user sends `threshold_count` messages within the `threshold_time` window. Also triggers on bot mentions and replies to bot messages. Enforces a `cooldown` period between responses to the same user.

### Sticker Reactions (`internal/handlers/on_sticker/`)

Reacts to stickers with probability `sticker_chance`. Replies with a random sticker from the pool (Google Sheets + Telegram sticker packs).

### Voice Reactions (`internal/handlers/on_voice/`)

Reacts to voice messages with probability `voice_chance`. Sends a voice message from Google Sheets with a simulated typing delay (0-15 seconds).

### User Join/Leave

- **Join** (`on_user_join/`): sends a greeting from Google Sheets.
- **Left** (`on_user_left/`): replies with a fixed farewell message.

### Personal (`internal/handlers/personal/`)

User-specific reactions for particular users (Igor — 1/750, Max — 1/200, Kirill — 1/150). User IDs are set via environment variables.

### Tagger (`internal/handlers/tagger/`)

Periodically tags a random chat member with an insult. Uses a priority queue (min-heap) for scheduling. Interval is randomized between `TAGGER_INTERVAL_FROM` and `TAGGER_INTERVAL_TO`.

### Settings (`internal/handlers/settings/`)

`/settings` command — admin-only in group chats:

- `/settings` — view current settings
- `/settings <key> <value>` — modify a setting
- `/settings reset` — reset to defaults

### Stats (`internal/handlers/stat/`)

`/stat` or `/stat YYYY-MM-DD`. Displays interaction statistics formatted with Telegram entities.

## Message Generation

Two strategies (`internal/message/`):

1. **List-Based** — random message from Google Sheets
2. **AI** — DeepSeek API with a system prompt (toxic insults, 1-2 sentences max). Controlled by per-chat `ai_chance` parameter. Falls back to list-based on error.

## Configuration

### Required Environment Variables

| Variable | Description |
|---|---|
| `TELEGRAM_TOKEN` | Bot token from BotFather |
| `SQLITE_FILE_PATH` | Path to SQLite database file |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `GIGACHAT_AUTH_KEY` | GigaChat API key |
| `GOOGLE_CREDENTIALS` | JSON with Google API credentials |
| `GOOGLE_SPREADSHEET_ID` | Google Sheets spreadsheet ID |

### Behavior and Timing

| Variable | Default | Description |
|---|---|---|
| `BULLINGS_THRESHOLD_COUNT` | 5 | Message count threshold to trigger |
| `BULLINGS_THRESHOLD_TIME` | 1m | Time window for message counting |
| `BULLINGS_COOLDOWN` | 1h | Cooldown between responses |
| `BULLINGS_AI_CHANCE` | 0.75 | Probability of AI generation |
| `STICKER_REACTIONS_CHANCE` | 0.4 | Probability of sticker reaction |
| `VOICE_REACTIONS_CHANCE` | 0.8 | Probability of voice reaction |
| `STICKER_SETS` | `static_bulling_by_stickersthiefbot` | Sticker packs (comma-separated) |
| `TAGGER_INTERVAL_FROM` | 10h | Min tagger interval |
| `TAGGER_INTERVAL_TO` | 24h | Max tagger interval |
| `TELEGRAM_LONG_POLL_TIMEOUT` | 10s | Long polling timeout |

### Data Refresh Periods

| Variable | Default |
|---|---|
| `BULLINGS_UPDATE_MESSAGES_PERIOD` | 10m |
| `STICKERS_UPDATE_PERIOD` | 30m |
| `ON_USER_JOIN_UPDATE_MESSAGES_PERIOD` | 10m |
| `VOICE_UPDATE_PERIOD` | 30m |
| `NICKNAMES_UPDATE_PERIOD` | 10m |
| `GOOGLE_CACHE_INTERVAL` | — |

### Personal Handlers

| Variable | Description |
|---|---|
| `IGOR_ID` | Telegram user ID for Igor |
| `MAX_ID` | Telegram user ID for Max |
| `KIRILL_ID` | Telegram user ID for Kirill |

## Per-Chat Settings

Stored in the `chat_settings` SQLite table. Cached by `chatsettings.Provider` with a 1-minute TTL. Nullable fields — global defaults are used when not overridden.

| Parameter | Type | Default |
|---|---|---|
| `threshold_count` | int | 5 |
| `threshold_time` | duration | 1m |
| `cooldown` | duration | 1h |
| `sticker_chance` | float 0.0-1.0 | 0.4 |
| `voice_chance` | float 0.0-1.0 | 0.8 |
| `ai_chance` | float 0.0-1.0 | 0.75 |

## Statistics and Analytics

`response_log` table — logs every interaction. Chat ID and User ID are AES-encrypted (key passed via `-ldflags` at build time). Operation types: `on_text`, `on_sticker`, `on_voice`, `on_user_join`, `on_user_left`, `personal`, `tagger`.

## Building and Running

```bash
go build -ldflags="-X main.AesKeyString=<BASE64_AES_KEY>" -o bot ./cmd/
```

AES key: 16, 24, or 32 bytes, Base64-encoded (raw, no padding).

Migrations run automatically on startup via `migrator.MigrateDB()`.

## Architectural Principles

- **Interfaces** are declared in the consumer package, not the provider
- **`contract.go`** — file containing handler interfaces (for mockgen)
- **Parallel dispatch** — all handlers for the same event run in goroutines
- **Background refresh** — Google Sheets data is periodically refreshed in the background
- **Thread safety** — `sync.RWMutex` for message collections
- **Async statistics** — all `stats.Inc()` calls run asynchronously
