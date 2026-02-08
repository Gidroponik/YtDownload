# YtUrl

YouTube video and audio downloader with Web UI and Telegram Bot, running entirely in Docker.

YouTube загрузчик видео и аудио с веб-интерфейсом и Telegram ботом, полностью работающий в Docker.

---

## Features / Возможности

- **Web UI** — minimalist dark interface, paste a link and download in one click
- **Video** — MP4 format, top 5 quality options, automatic audio+video merging
- **Audio** — MP3 download with quality selection
- **Real-time progress** — SSE-based progress bar showing each download stage
- **Telegram Bot** — send a YouTube link, get a video back (best quality up to 50 MB, ideal for Shorts)
- **Whitelist** — the first user to message the bot becomes the owner; all other users are silently ignored
- **LAN access** — accessible from any device on your local network
- **No dependencies** — everything runs inside Docker (yt-dlp, ffmpeg, Go, nginx)

---

## Screenshots / Скриншоты

### Web UI

![Web UI](web_preview.jpg)

### Telegram Bot

![Telegram Bot](tg_preview.jpg)

---

## Quick Start / Быстрый старт

### 1. Clone / Клонировать

```bash
git clone https://github.com/Gidroponik/YtDownload.git
cd YtUrl
```

### 2. Configure / Настроить

Copy the example config and fill in your values:

Скопируйте пример конфигурации и заполните:

```bash
cp .env.example .env
```

Edit `.env`:

```env
APP_PORT=3080
TELEGRAM_BOT=your_telegram_bot_token
TELEGRAM_OWNER=
```

| Variable | Description |
|---|---|
| `APP_PORT` | Port for the web interface (default: `3080`) |
| `TELEGRAM_BOT` | Telegram Bot API token from [@BotFather](https://t.me/BotFather). Leave empty to disable the bot |
| `TELEGRAM_OWNER` | Auto-filled after the first user writes to the bot. Do not set manually |

### 3. Run / Запустить

```bash
docker compose up -d --build
```

### 4. Open / Открыть

Open in your browser:

```
http://<your-ip>:<APP_PORT>
```

For example: `http://192.168.1.100:3080`

---

## Telegram Bot / Telegram Бот

If `TELEGRAM_BOT` is set in `.env`, a Telegram bot starts automatically alongside the app.

Если `TELEGRAM_BOT` задан в `.env`, Telegram бот запускается автоматически вместе с приложением.

**How it works / Как работает:**

1. Send any YouTube link to the bot
2. The bot finds the best MP4 quality that fits within Telegram's 50 MB file limit
3. Downloads the video, merges audio, and sends the file back

**Whitelist / Белый список:**

> The first user to send a message to the bot is automatically registered as the owner. Their `telegram_id` is saved to `.env`. After that, all messages from other users are silently ignored. This persists across container restarts.
>
> Первый пользователь, написавший боту, автоматически становится владельцем. Его `telegram_id` сохраняется в `.env`. После этого все сообщения от других пользователей молча игнорируются. Настройка сохраняется между перезапусками контейнера.

---

## Architecture / Архитектура

```
┌──────────────┐    ┌──────────────┐
│   Frontend   │    │   Backend    │
│  Vue 3 +     │───▶│  Go (Gin)    │
│  nginx       │    │  yt-dlp      │
│  :80         │    │  ffmpeg      │
│              │    │  :8080       │
└──────────────┘    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ Telegram Bot │
                    │ (optional)   │
                    └──────────────┘
```

- **Frontend**: Vue 3 + Vite + Tailwind CSS, served by nginx with reverse proxy to backend
- **Backend**: Go (Gin), uses yt-dlp standalone binary + ffmpeg for downloading and processing
- **Bot**: Long-polling Telegram bot built into the backend process

---

## Tech Stack / Стек технологий

| Component | Technology |
|---|---|
| Backend | Go 1.22, Gin |
| Frontend | Vue 3, Vite, Tailwind CSS |
| Download engine | yt-dlp (standalone binary) |
| Audio/Video processing | ffmpeg |
| Telegram Bot | go-telegram-bot-api/v5 |
| Containerization | Docker, Docker Compose |
| Reverse proxy | nginx |

---

## License / Лицензия

MIT
