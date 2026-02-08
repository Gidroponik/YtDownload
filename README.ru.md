# YtDownload

**[English](README.md)** | **Русский**

Загрузчик видео и аудио с YouTube с веб-интерфейсом и Telegram ботом. Полностью работает в Docker.

### Возможности

- **Web UI** — минималистичный тёмный интерфейс, вставьте ссылку и скачайте в один клик
- **Видео** — формат MP4, топ-5 вариантов качества, автоматическое объединение аудио и видео
- **Аудио** — скачивание в MP3 с выбором качества
- **Прогресс в реальном времени** — прогресс-бар с отображением каждого этапа загрузки
- **Telegram Бот** — отправьте ссылку на YouTube, получите видео в ответ (лучшее качество до 50 МБ, идеально для Shorts)
- **Белый список** — первый написавший боту становится владельцем; остальные пользователи молча игнорируются
- **Доступ по локальной сети** — доступен с любого устройства в вашей сети
- **Без зависимостей** — всё работает внутри Docker (yt-dlp, ffmpeg, Go, nginx)

### Скриншоты

| Web UI | Telegram Бот |
|---|---|
| ![Web UI](web_preview.jpg) | ![Telegram Bot](tg_preview.jpg) |

### Необходимое ПО

| Зависимость | Скачать |
|---|---|
| **Docker Desktop** | [Windows](https://docs.docker.com/desktop/setup/install/windows-install/) / [Mac](https://docs.docker.com/desktop/setup/install/mac-install/) / [Linux](https://docs.docker.com/desktop/setup/install/linux/) |
| **Git** | [git-scm.com](https://git-scm.com/downloads) |

> Убедитесь, что Docker Desktop **запущен** перед началом установки.

### Быстрый старт (сборка из исходников)

**1. Клонировать**

```bash
git clone https://github.com/Gidroponik/YtDownload.git
cd YtDownload
```

**2. Настроить**

```bash
cp .env.example .env
```

Отредактируйте `.env`:

```env
APP_PORT=3080
TELEGRAM_BOT=токен_вашего_бота
TELEGRAM_OWNER=
```

| Переменная | Описание |
|---|---|
| `APP_PORT` | Порт веб-интерфейса (по умолчанию: `3080`) |
| `TELEGRAM_BOT` | Токен Telegram бота от [@BotFather](https://t.me/BotFather). Оставьте пустым, чтобы отключить бота |
| `TELEGRAM_OWNER` | Заполняется автоматически после первого сообщения боту. Не заполняйте вручную |

**3. Запустить**

```bash
docker compose up -d --build
```

**4. Открыть**

```
http://<ваш-ip>:<APP_PORT>
```

Пример: `http://192.168.1.100:3080`

### Быстрый старт (Docker Hub)

Готовые образы доступны на Docker Hub:

| Образ | Ссылка |
|---|---|
| Backend | [gidro777/ytdownload-backend](https://hub.docker.com/r/gidro777/ytdownload-backend) |
| Frontend | [gidro777/ytdownload-frontend](https://hub.docker.com/r/gidro777/ytdownload-frontend) |

**1.** Создайте `docker-compose.yml`:

```yaml
services:
  backend:
    image: gidro777/ytdownload-backend:latest
    environment:
      - TELEGRAM_BOT=${TELEGRAM_BOT:-}
      - TELEGRAM_OWNER=${TELEGRAM_OWNER:-}
    volumes:
      - ./.env:/app/.env
    restart: unless-stopped

  frontend:
    image: gidro777/ytdownload-frontend:latest
    ports:
      - "0.0.0.0:${APP_PORT:-3080}:80"
    depends_on:
      - backend
    restart: unless-stopped
```

**2.** Создайте файл `.env`:

```env
APP_PORT=3080
TELEGRAM_BOT=токен_вашего_бота
TELEGRAM_OWNER=
```

**3.** Запустите:

```bash
docker compose up -d
```

### Telegram Бот

Если `TELEGRAM_BOT` задан в `.env`, Telegram бот запускается автоматически вместе с приложением.

**Как работает:**

1. Отправьте любую ссылку на YouTube боту
2. Бот находит лучшее качество MP4, которое вписывается в лимит Telegram 50 МБ
3. Скачивает видео, объединяет с аудио и отправляет файл в ответ

> **Белый список:** Первый пользователь, написавший боту, автоматически становится владельцем. Его `telegram_id` сохраняется в `.env` и сохраняется между перезапусками контейнера. Все сообщения от других пользователей молча игнорируются.

### Архитектура

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
                    │ Telegram Бот │
                    │ (опционально)│
                    └──────────────┘
```

### Стек технологий

| Компонент | Технология |
|---|---|
| Backend | Go 1.22, Gin |
| Frontend | Vue 3, Vite, Tailwind CSS |
| Движок загрузки | yt-dlp (standalone binary) |
| Обработка аудио/видео | ffmpeg |
| Telegram Бот | go-telegram-bot-api/v5 |
| Контейнеризация | Docker, Docker Compose |
| Reverse proxy | nginx |

### Лицензия

MIT
