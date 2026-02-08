package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"yturl/backend/platform"

	"github.com/google/uuid"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxFileSize = 50 * 1024 * 1024 // 50 MB

var (
	ownerID     int64
	ownerMu     sync.Mutex
	botUsername string
	botUserMu   sync.RWMutex
)

// GetUsername returns the bot's username, or empty string if not started.
func GetUsername() string {
	botUserMu.RLock()
	defer botUserMu.RUnlock()
	return botUsername
}

func Start(token string) {
	if id := os.Getenv("TELEGRAM_OWNER"); id != "" {
		if parsed, err := strconv.ParseInt(id, 10, 64); err == nil {
			ownerID = parsed
			log.Printf("[telegram] owner set: %d", ownerID)
		}
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("[telegram] failed to start bot: %v", err)
		return
	}

	botUserMu.Lock()
	botUsername = api.Self.UserName
	botUserMu.Unlock()

	log.Printf("[telegram] bot started: @%s", api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go handleMessage(api, update.Message)
	}
}

func handleMessage(api *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	userID := msg.From.ID

	ownerMu.Lock()
	if ownerID == 0 {
		ownerID = userID
		log.Printf("[telegram] owner registered: %d", userID)
		saveOwnerToEnv(userID)
		ownerMu.Unlock()
	} else if ownerID != userID {
		ownerMu.Unlock()
		return
	} else {
		ownerMu.Unlock()
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	p, url := platform.DetectPlatform(text)
	if p == platform.Unknown {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Send me a YouTube, TikTok, or Instagram link.")
		reply.ReplyToMessageID = msg.MessageID
		api.Send(reply)
		return
	}

	// Show "uploading video" status throughout the entire process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sendTypingLoop(ctx, api, msg.Chat.ID)

	info, err := platform.FetchInfo(url)
	if err != nil {
		sendError(api, msg, fmt.Sprintf("Failed to get video info: %v", err))
		return
	}

	formatID := platform.PickBestTelegramFormat(p, info, maxFileSize)
	if formatID == "" {
		sendError(api, msg, "No suitable format found under 50 MB.")
		return
	}

	fileID := uuid.New().String()
	tmpPath := filepath.Join(os.TempDir(), fileID+".mp4")
	defer os.Remove(tmpPath)

	args := platform.BuildTelegramDownloadArgs(p, formatID, tmpPath, url)
	cmd := exec.Command("yt-dlp", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		sendError(api, msg, "Download failed.")
		return
	}

	stat, err := os.Stat(tmpPath)
	if err != nil || stat.Size() > maxFileSize {
		sendError(api, msg, fmt.Sprintf("File too large (%.1f MB). Telegram limit is 50 MB.", float64(stat.Size())/(1024*1024)))
		return
	}

	video := tgbotapi.NewVideo(msg.Chat.ID, tgbotapi.FilePath(tmpPath))
	video.ReplyToMessageID = msg.MessageID
	video.SupportsStreaming = true
	if _, err := api.Send(video); err != nil {
		sendError(api, msg, fmt.Sprintf("Failed to send video: %v", err))
	}
}

func saveOwnerToEnv(id int64) {
	envPath := "/app/.env"

	content, err := os.ReadFile(envPath)
	if err != nil {
		log.Printf("[telegram] cannot read .env: %v", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "TELEGRAM_OWNER=") {
			lines[i] = fmt.Sprintf("TELEGRAM_OWNER=%d", id)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("TELEGRAM_OWNER=%d", id))
	}

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		log.Printf("[telegram] cannot write .env: %v", err)
	} else {
		log.Printf("[telegram] saved owner %d to .env", id)
	}
}

func sendTypingLoop(ctx context.Context, api *tgbotapi.BotAPI, chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatUploadVideo)
	api.Send(action)
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			api.Send(action)
		}
	}
}

func sendError(api *tgbotapi.BotAPI, msg *tgbotapi.Message, text string) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID
	api.Send(reply)
}
