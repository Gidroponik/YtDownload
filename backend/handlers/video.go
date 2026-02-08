package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InfoRequest struct {
	URL  string `json:"url" binding:"required"`
	Mode string `json:"mode"` // "video" or "audio"
}

type FormatInfo struct {
	FormatID string `json:"formatId"`
	Quality  string `json:"quality"`
	Height   int    `json:"height,omitempty"`
	Bitrate  int    `json:"bitrate,omitempty"`
	Size     int64  `json:"size"`
}

type VideoInfoResponse struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Author    string       `json:"author"`
	Duration  string       `json:"duration"`
	Thumbnail string       `json:"thumbnail"`
	Formats   []FormatInfo `json:"formats"`
}

type ytdlpInfo struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Uploader  string        `json:"uploader"`
	Duration  float64       `json:"duration"`
	Thumbnail string        `json:"thumbnail"`
	Formats   []ytdlpFormat `json:"formats"`
}

type ytdlpFormat struct {
	FormatID  string  `json:"format_id"`
	Ext       string  `json:"ext"`
	Height    int     `json:"height"`
	Filesize  int64   `json:"filesize"`
	FilesizeA int64   `json:"filesize_approx"`
	VCodec    string  `json:"vcodec"`
	ACodec    string  `json:"acodec"`
	ABR       float64 `json:"abr"`
	FPS       float64 `json:"fps"`
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

func fetchYtdlpInfo(url string) (*ytdlpInfo, error) {
	cmd := exec.Command("yt-dlp", "-j", "--no-warnings", url)
	output, err := cmd.Output()
	if err != nil {
		errMsg := "unknown error"
		if exitErr, ok := err.(*exec.ExitError); ok {
			errMsg = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	var info ytdlpInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse video info")
	}
	return &info, nil
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

	info, err := fetchYtdlpInfo(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get video info: %v", err)})
		return
	}

	var formats []FormatInfo

	if mode == "audio" {
		formats = buildAudioFormats(info)
	} else {
		formats = buildVideoFormats(info)
	}

	c.JSON(http.StatusOK, VideoInfoResponse{
		ID:        info.ID,
		Title:     info.Title,
		Author:    info.Uploader,
		Duration:  formatDuration(info.Duration),
		Thumbnail: info.Thumbnail,
		Formats:   formats,
	})
}

func buildVideoFormats(info *ytdlpInfo) []FormatInfo {
	var videoFormats []ytdlpFormat
	for _, f := range info.Formats {
		if f.Ext != "mp4" {
			continue
		}
		if f.VCodec == "" || f.VCodec == "none" {
			continue
		}
		if f.Height <= 0 {
			continue
		}
		videoFormats = append(videoFormats, f)
	}

	sort.Slice(videoFormats, func(i, j int) bool {
		return videoFormats[i].Height > videoFormats[j].Height
	})

	seen := map[int]bool{}
	var unique []ytdlpFormat
	for _, f := range videoFormats {
		if seen[f.Height] {
			continue
		}
		seen[f.Height] = true
		unique = append(unique, f)
	}

	if len(unique) > 5 {
		unique = unique[:5]
	}

	var formats []FormatInfo
	for _, f := range unique {
		size := f.Filesize
		if size <= 0 {
			size = f.FilesizeA
		}
		formats = append(formats, FormatInfo{
			FormatID: f.FormatID,
			Quality:  fmt.Sprintf("%dp", f.Height),
			Height:   f.Height,
			Size:     size,
		})
	}
	return formats
}

func buildAudioFormats(info *ytdlpInfo) []FormatInfo {
	var audioFormats []ytdlpFormat
	for _, f := range info.Formats {
		if f.ACodec == "" || f.ACodec == "none" {
			continue
		}
		// Audio-only formats
		if f.VCodec != "" && f.VCodec != "none" {
			continue
		}
		if f.ABR <= 0 {
			continue
		}
		audioFormats = append(audioFormats, f)
	}

	sort.Slice(audioFormats, func(i, j int) bool {
		return audioFormats[i].ABR > audioFormats[j].ABR
	})

	// Deduplicate by bitrate (rounded to nearest 10)
	seen := map[int]bool{}
	var unique []ytdlpFormat
	for _, f := range audioFormats {
		key := int(f.ABR/10) * 10
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, f)
	}

	if len(unique) > 5 {
		unique = unique[:5]
	}

	var formats []FormatInfo
	for _, f := range unique {
		size := f.Filesize
		if size <= 0 {
			size = f.FilesizeA
		}
		formats = append(formats, FormatInfo{
			FormatID: f.FormatID,
			Quality:  fmt.Sprintf("%.0f kbps", f.ABR),
			Bitrate:  int(f.ABR),
			Size:     size,
		})
	}
	return formats
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
	var args []string

	if mode == "audio" {
		ext = "mp3"
		tmpPath := filepath.Join(os.TempDir(), fileID+".%(ext)s")
		sendEvent(ProgressEvent{Stage: "downloading_audio", Percent: 0})
		args = []string{
			"--newline", "--progress", "--no-warnings", "--no-part",
			"-f", formatID,
			"-x", "--audio-format", "mp3",
			"-o", tmpPath,
			videoURL,
		}
	} else {
		tmpPath := filepath.Join(os.TempDir(), fileID+".mp4")
		sendEvent(ProgressEvent{Stage: "downloading_video", Percent: 0})
		args = []string{
			"--newline", "--progress", "--no-warnings", "--no-part",
			"-f", fmt.Sprintf("%s+bestaudio[ext=m4a]/%s+bestaudio/%s", formatID, formatID, formatID),
			"--merge-output-format", "mp4",
			"-o", tmpPath,
			videoURL,
		}
	}

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
