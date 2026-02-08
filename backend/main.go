package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"yturl/backend/bot"
	"yturl/backend/handlers"
)

func main() {
	// Start Telegram bot if token is set
	if token := os.Getenv("TELEGRAM_BOT"); token != "" {
		go bot.Start(token)
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:   []string{"Content-Disposition", "Content-Length"},
	}))

	api := r.Group("/api/video")
	{
		api.POST("/info", handlers.GetVideoInfo)
		api.GET("/download", handlers.DownloadVideo)
		api.GET("/file/:id", handlers.ServeFile)
	}

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
