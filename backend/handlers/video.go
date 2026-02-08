package handlers

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"yturl/backend/platform"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InfoRequest struct {
	URL  string `json:"url" binding:"required"`
	Mode string `json:"mode"` // "video" or "audio"
}

type VideoInfoResponse struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Author    string                `json:"author"`
	Duration  string                `json:"duration"`
	Thumbnail string                `json:"thumbnail"`
	Platform  platform.Platform     `json:"platform"`
	Formats   []platform.FormatInfo `json:"formats"`
}

type ProgressEvent struct {
	Stage   string  `json:"stage"`
	Percent float64 `json:"percent"`
	FileID  string  `json:"fileId,omitempty"`
	Ext     string  `json:"ext,omitempty"`
	Error   string  `json:"error,omitempty"`
}

var (
	percentRe     = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)
	destinationRe = regexp.MustCompile(`\[download\] Destination:`)
	mergerRe      = regexp.MustCompile(`\[Merger\]`)
	extractRe     = regexp.MustCompile(`\[ExtractAudio\]`)
)

func formatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func GetVideoInfo(c *gin.Context) {
	var req InfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "video"
	}

	p, _ := platform.DetectPlatform(req.URL)

	info, err := platform.FetchInfo(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get video info: %v", err)})
		return
	}

	var formats []platform.FormatInfo
	if mode == "audio" {
		formats = platform.BuildAudioFormats(p, info)
	} else {
		formats = platform.BuildVideoFormats(p, info)
	}

	thumbnail := info.Thumbnail
	if p == platform.Instagram && thumbnail != "" {
		// Instagram CDN blocks cross-origin — proxy through our API
		thumbnail = "/api/video/thumb?url=" + base64.URLEncoding.EncodeToString([]byte(thumbnail))
	}

	c.JSON(http.StatusOK, VideoInfoResponse{
		ID:        info.ID,
		Title:     info.Title,
		Author:    info.Uploader,
		Duration:  formatDuration(info.Duration),
		Thumbnail: thumbnail,
		Platform:  p,
		Formats:   formats,
	})
}

func DownloadVideo(c *gin.Context) {
	videoURL := c.Query("url")
	formatID := c.Query("format")
	mode := c.Query("mode") // "video" or "audio"

	if videoURL == "" || formatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and format required"})
		return
	}

	if mode == "" {
		mode = "video"
	}

	p, _ := platform.DetectPlatform(videoURL)

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	sendEvent := func(evt ProgressEvent) {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	fileID := uuid.New().String()
	ext := "mp4"

	if mode == "audio" {
		ext = "mp3"
		sendEvent(ProgressEvent{Stage: "downloading_audio", Percent: 0})
	} else {
		sendEvent(ProgressEvent{Stage: "downloading_video", Percent: 0})
	}

	tmpPath := filepath.Join(os.TempDir(), fileID+".%(ext)s")
	if mode == "video" {
		tmpPath = filepath.Join(os.TempDir(), fileID+".mp4")
	}

	args := platform.BuildDownloadArgs(p, formatID, tmpPath, videoURL, mode)

	ctx := c.Request.Context()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		sendEvent(ProgressEvent{Stage: "error", Error: "Internal error"})
		return
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		sendEvent(ProgressEvent{Stage: "error", Error: "Failed to start download"})
		return
	}
	pw.Close()

	downloadCount := 0
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := scanner.Text()

		if destinationRe.MatchString(line) {
			downloadCount++
			if downloadCount == 2 && mode == "video" {
				sendEvent(ProgressEvent{Stage: "downloading_audio", Percent: 0})
			}
		}

		if matches := percentRe.FindStringSubmatch(line); len(matches) > 1 {
			pct, _ := strconv.ParseFloat(matches[1], 64)
			stage := "downloading_video"
			if mode == "audio" || downloadCount >= 2 {
				stage = "downloading_audio"
			}
			sendEvent(ProgressEvent{Stage: stage, Percent: pct})
		}

		if mergerRe.MatchString(line) {
			sendEvent(ProgressEvent{Stage: "merging", Percent: -1})
		}
		if extractRe.MatchString(line) {
			sendEvent(ProgressEvent{Stage: "converting", Percent: -1})
		}
	}
	pr.Close()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == nil {
			sendEvent(ProgressEvent{Stage: "error", Error: "Download failed"})
		}
		return
	}

	go func() {
		time.Sleep(10 * time.Minute)
		os.Remove(filepath.Join(os.TempDir(), fileID+"."+ext))
	}()

	sendEvent(ProgressEvent{Stage: "done", FileID: fileID, Ext: ext})
}

func ServeFile(c *gin.Context) {
	id := c.Param("id")

	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Try mp4 first, then mp3
	for _, ext := range []string{"mp4", "mp3"} {
		path := filepath.Join(os.TempDir(), id+"."+ext)
		if _, err := os.Stat(path); err == nil {
			mime := "video/mp4"
			if ext == "mp3" {
				mime = "audio/mpeg"
			}
			c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, id, ext))
			c.Header("Content-Type", mime)
			c.File(path)

			go func() {
				time.Sleep(5 * time.Second)
				os.Remove(path)
			}()
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "File not found or expired"})
}

func ProxyThumbnail(c *gin.Context) {
	encoded := c.Query("url")
	if encoded == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	raw, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	imgURL := string(raw)

	resp, err := http.Get(imgURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch thumbnail"})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "public, max-age=3600")
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
