package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var youtubeRe = regexp.MustCompile(`(?i)(https?://)?(www\.)?(youtube\.com|youtu\.be|m\.youtube\.com)/.+`)

const maxFileSize = 50 * 1024 * 1024 // 50 MB

var (
	ownerID   int64
	ownerOnce sync.Once
	ownerMu   sync.Mutex
)

type ytdlpFormat struct {
	FormatID  string  `json:"format_id"`
	Ext       string  `json:"ext"`
	Height    int     `json:"height"`
	Filesize  int64   `json:"filesize"`
	FilesizeA int64   `json:"filesize_approx"`
	VCodec    string  `json:"vcodec"`
	ACodec    string  `json:"acodec"`
	ABR       float64 `json:"abr"`
}

type ytdlpInfo struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Formats []ytdlpFormat `json:"formats"`
}

func Start(token string) {
	// Load owner from env
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
		// First user becomes the owner
		ownerID = userID
		log.Printf("[telegram] owner registered: %d", userID)
		saveOwnerToEnv(userID)
		ownerMu.Unlock()
	} else if ownerID != userID {
		// Not the owner — ignore silently
		ownerMu.Unlock()
		return
	} else {
		ownerMu.Unlock()
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	if !youtubeRe.MatchString(text) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Send me a YouTube link.")
		reply.ReplyToMessageID = msg.MessageID
		api.Send(reply)
		return
	}

	url := youtubeRe.FindString(text)

	info, err := getInfo(url)
	if err != nil {
		sendError(api, msg, fmt.Sprintf("Failed to get video info: %v", err))
		return
	}

	formatID := pickBestFormat(info)
	if formatID == "" {
		sendError(api, msg, "No suitable format found under 50 MB.")
		return
	}

	fileID := uuid.New().String()
	tmpPath := filepath.Join(os.TempDir(), fileID+".mp4")
	defer os.Remove(tmpPath)

	cmd := exec.Command("yt-dlp",
		"--no-warnings", "--no-part",
		"-f", fmt.Sprintf("%s+bestaudio[ext=m4a]/%s+bestaudio/%s", formatID, formatID, formatID),
		"--merge-output-format", "mp4",
		"-o", tmpPath,
		url,
	)
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
		// Insert before the last empty line if exists
		lines = append(lines, fmt.Sprintf("TELEGRAM_OWNER=%d", id))
	}

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		log.Printf("[telegram] cannot write .env: %v", err)
	} else {
		log.Printf("[telegram] saved owner %d to .env", id)
	}
}

func getInfo(url string) (*ytdlpInfo, error) {
	cmd := exec.Command("yt-dlp", "-j", "--no-warnings", url)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var info ytdlpInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func pickBestFormat(info *ytdlpInfo) string {
	type candidate struct {
		formatID string
		height   int
		size     int64
	}

	var candidates []candidate
	for _, f := range info.Formats {
		if f.Ext != "mp4" || f.VCodec == "" || f.VCodec == "none" || f.Height <= 0 {
			continue
		}
		size := f.Filesize
		if size <= 0 {
			size = f.FilesizeA
		}
		estimated := int64(float64(size) * 1.15)
		candidates = append(candidates, candidate{
			formatID: f.FormatID,
			height:   f.Height,
			size:     estimated,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].height > candidates[j].height
	})

	for _, c := range candidates {
		if c.size > 0 && c.size <= maxFileSize {
			return c.formatID
		}
	}

	for _, c := range candidates {
		if c.height <= 720 {
			return c.formatID
		}
	}

	if len(candidates) > 0 {
		return candidates[len(candidates)-1].formatID
	}

	return ""
}

func sendError(api *tgbotapi.BotAPI, msg *tgbotapi.Message, text string) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID
	api.Send(reply)
}
