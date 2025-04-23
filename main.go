package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/handlers"
	"github.com/arnnvv/peeple-api/pkg/token"
)

type Config struct {
	Port          string
	DatabaseURL   string
	ServerTimeout struct {
		ReadHeader time.Duration
		Write      time.Duration
		Idle       time.Duration
	}
}

func loadConfig() Config {
	cfg := Config{
		Port:        getEnv("PORT", ""),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		ServerTimeout: struct {
			ReadHeader time.Duration
			Write      time.Duration
			Idle       time.Duration
		}{
			ReadHeader: 5 * time.Second,
			Write:      10 * time.Second,
			Idle:       60 * time.Second,
		},
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cfg := loadConfig()

	if err := db.InitDB(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: cfg.ServerTimeout.ReadHeader,
		WriteTimeout:      cfg.ServerTimeout.Write,
		IdleTimeout:       cfg.ServerTimeout.Idle,
		Handler:           setupRoutes(),
	}

	serverCtx, serverStopCtx := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig
		log.Println("Shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("Graceful shutdown timed out.. forcing exit")
			}
		}()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
		serverStopCtx()
	}()

	log.Printf("Server starting on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	<-serverCtx.Done()
	log.Println("Server stopped")
}

func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth/google/verify", handlers.GoogleAuthHandler)
	mux.HandleFunc("/token", token.GenerateTokenHandler)
	mux.HandleFunc("/test", handlers.TestHandler)

	authMiddleware := token.AuthMiddleware
	mux.HandleFunc("/api/auth-status", authMiddleware(handlers.CheckAuthStatus))
	mux.HandleFunc("/api/profile", authMiddleware(handlers.CreateProfile))
	mux.HandleFunc("/api/profile/location-gender", authMiddleware(handlers.UpdateLocationGenderHandler))
	mux.HandleFunc("/get-profile", authMiddleware(handlers.ProfileHandler))
	mux.HandleFunc("/api/profile/edit", authMiddleware(handlers.EditProfileHandler))
	mux.HandleFunc("/upload", authMiddleware(handlers.GeneratePresignedURLs))
	mux.HandleFunc("/audio", authMiddleware(handlers.GenerateAudioPresignedURL))
	mux.HandleFunc("/verify", authMiddleware(handlers.GenerateVerificationPresignedURL))
	mux.HandleFunc("/api/edit-presigned-urls", authMiddleware(handlers.GenerateEditPresignedURLs))
	mux.HandleFunc("/api/filters", authMiddleware(handlers.ApplyFiltersHandler))
	mux.HandleFunc("/api/get-filters", authMiddleware(handlers.GetFiltersHandler))
	mux.HandleFunc("/api/app-opened", authMiddleware(handlers.LogAppOpenHandler))
	mux.HandleFunc("/api/homefeed", authMiddleware(handlers.GetHomeFeedHandler))
	mux.HandleFunc("/api/quickfeed", authMiddleware(handlers.GetQuickFeedHandler))
	mux.HandleFunc("/api/like", authMiddleware(handlers.LikeHandler))
	mux.HandleFunc("/api/dislike", authMiddleware(handlers.DislikeHandler))
	mux.HandleFunc("/api/unmatch", authMiddleware(handlers.UnmatchHandler))
	mux.HandleFunc("/api/report", authMiddleware(handlers.ReportHandler))
	mux.HandleFunc("/api/likes/received", authMiddleware(handlers.GetWhoLikedYouHandler))
	mux.HandleFunc("/api/liker-profile/", authMiddleware(handlers.GetLikerProfileHandler))
	mux.HandleFunc("/api/matches", authMiddleware(handlers.GetMatchesHandler))
	mux.HandleFunc("/api/iap/verify", authMiddleware(handlers.VerifyPurchaseHandler))
	mux.HandleFunc("/chat", authMiddleware(handlers.ChatHandler))
	mux.HandleFunc("/api/conversation", authMiddleware(handlers.GetConversationHandler))
	mux.HandleFunc("/api/chat/upload", authMiddleware(handlers.GenerateChatMediaPresignedURL))
	mux.HandleFunc("/api/chat/mark-read-until", authMiddleware(handlers.MarkReadUntilHandler))
	mux.HandleFunc("/api/unread-chat-count", authMiddleware(handlers.GetUnreadCountHandler))
	mux.HandleFunc("/api/me/update-online", authMiddleware(handlers.UpdateOnlineStatusHandler))
	mux.HandleFunc("/api/user/last-online", authMiddleware(handlers.FetchLastOnlineHandler))

	adminMiddleware := token.AdminAuthMiddleware
	mux.HandleFunc("/api/set-admin", adminMiddleware(handlers.SetAdminHandler))
	mux.HandleFunc("/api/admin/verifications", adminMiddleware(handlers.GetPendingVerificationsHandler))
	mux.HandleFunc("/api/admin/verify", adminMiddleware(handlers.UpdateVerificationStatusHandler))

	mux.HandleFunc("/", authMiddleware(handlers.ProtectedHandler))

	return mux
}
