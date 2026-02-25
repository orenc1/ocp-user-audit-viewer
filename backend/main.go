package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ocohen/ocp-user-auditter/backend/api"
	"github.com/ocohen/ocp-user-auditter/backend/db"
)

func main() {
	dsn := getEnv("DATABASE_URL",
		"postgres://audit:audit@localhost:5432/audit?sslmode=disable")
	port := getEnv("PORT", "8080")
	ingestToken := getEnv("INGEST_TOKEN", "")
	accessConfigPath := getEnv("ACCESS_CONFIG_PATH", "/etc/audit-access/access.yaml")

	database, err := db.New(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go database.StartRetentionLoop(ctx, 1*time.Hour)

	accessChecker, err := api.NewAccessChecker(accessConfigPath)
	if err != nil {
		log.Fatalf("Failed to load access config: %v", err)
	}

	handler := api.NewHandler(database)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Forwarded-User"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/healthz", handler.Healthz)

	ingest := r.Group("/api/v1")
	ingest.Use(api.InternalAuthMiddleware(ingestToken))
	ingest.POST("/ingest", handler.Ingest)

	external := r.Group("/api/v1")
	external.Use(api.OAuthMiddleware(accessChecker))
	external.GET("/events", handler.ListEvents)
	external.GET("/events/:id", handler.GetEvent)
	external.GET("/stats", handler.GetStats)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
		os.Exit(0)
	}()

	log.Printf("Starting backend on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
