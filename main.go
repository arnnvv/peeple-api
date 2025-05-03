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
	"github.com/arnnvv/peeple-api/pkg/pbsb"
	"github.com/arnnvv/peeple-api/pkg/ratelimit"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/ws"
	"github.com/go-redis/redis_rate/v10"
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
		Port:        getEnv("PORT", "8081"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		ServerTimeout: struct {
			ReadHeader time.Duration
			Write      time.Duration
			Idle       time.Duration
		}{
			ReadHeader: 5 * time.Second,
			Write:      10 * time.Second,
			Idle:       90 * time.Second,
		},
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("FATAL: DATABASE_URL environment variable not set.")
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if defaultValue == "" {
		log.Printf("Warning: Environment variable %s not set and no default value provided.", key)
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

	redisClient, err := pbsb.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		} else {
			log.Println("Redis client closed.")
		}
	}()

	rateLimiter := redis_rate.NewLimiter(redisClient)
	log.Println("Redis Rate Limiter initialized.")

	queries, err := db.GetDB()
	if err != nil {
		log.Fatalf("Failed to get DB queries for Hub: %v", err)
	}
	hub := ws.NewHub(queries, redisClient, rateLimiter)
	go hub.Run()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: cfg.ServerTimeout.ReadHeader,
		WriteTimeout:      cfg.ServerTimeout.Write,
		IdleTimeout:       cfg.ServerTimeout.Idle,
		Handler:           setupRoutes(hub, rateLimiter),
	}

	serverCtx, serverStopCtx := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig
		log.Println("Shutdown signal received")
		log.Println("Stopping Hub...")
		hub.Stop()
		log.Println("Hub stopped.")
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

func setupRoutes(hub *ws.Hub, limiter *redis_rate.Limiter) http.Handler {
	mux := http.NewServeMux()

	authMiddlewareFunc := token.AuthMiddleware
	adminAuthMiddlewareFunc := token.AdminAuthMiddleware

	generalRateLimitMW := ratelimit.RateLimiterMiddleware(limiter, 10, 20)
	editRateLimitMW := ratelimit.RateLimiterMiddleware(limiter, 2, 5)
	feedRateLimitMW := ratelimit.RateLimiterMiddleware(limiter, 5, 10)
	uploadRateLimitMW := ratelimit.RateLimiterMiddleware(limiter, 3, 6)

	apply := func(h http.HandlerFunc, mw ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}

	adapt := func(mw func(http.Handler) http.Handler) func(http.HandlerFunc) http.HandlerFunc {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return mw(next).ServeHTTP
		}
	}

	adaptGeneralRateLimit := adapt(generalRateLimitMW)
	adaptEditRateLimit := adapt(editRateLimitMW)
	adaptFeedRateLimit := adapt(feedRateLimitMW)
	adaptUploadRateLimit := adapt(uploadRateLimitMW)

	actualChatHandler := handlers.ChatHandler(hub)
	mux.HandleFunc("/chat", authMiddlewareFunc(actualChatHandler))

	mux.HandleFunc("/api/auth/google/verify", handlers.GoogleAuthHandler)
	mux.HandleFunc("/token", token.GenerateTokenHandler)
	mux.HandleFunc("/test", handlers.TestHandler)

	mux.HandleFunc("/api/auth-status", apply(handlers.CheckAuthStatus, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/profile", apply(handlers.CreateProfile, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/profile/location-gender", apply(handlers.UpdateLocationGenderHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/get-profile", apply(handlers.ProfileHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/profile/edit", apply(handlers.EditProfileHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/upload", apply(handlers.GeneratePresignedURLs, adaptUploadRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/audio", apply(handlers.GenerateAudioPresignedURL, adaptUploadRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/verify", apply(handlers.GenerateVerificationPresignedURL, adaptUploadRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/edit-presigned-urls", apply(handlers.GenerateEditPresignedURLs, adaptUploadRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/filters", apply(handlers.ApplyFiltersHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/get-filters", apply(handlers.GetFiltersHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/app-opened", apply(handlers.LogAppOpenHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/homefeed", apply(handlers.GetHomeFeedHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/quickfeed", apply(handlers.GetQuickFeedHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/report", apply(handlers.ReportHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/likes/received", apply(handlers.GetWhoLikedYouHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/likes/seen-until", apply(handlers.MarkLikesSeenUntilHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/liker-profile/", apply(handlers.GetLikerProfileHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/matches", apply(handlers.GetMatchesHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/iap/verify", apply(handlers.VerifyPurchaseHandler, adaptEditRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/conversation", apply(handlers.GetConversationHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/chat/upload", apply(handlers.GenerateChatMediaPresignedURL, adaptUploadRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/unread-chat-count", apply(handlers.GetUnreadCountHandler, adaptFeedRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/user/last-online", apply(handlers.FetchLastOnlineHandler, adaptFeedRateLimit, authMiddlewareFunc))

	mux.HandleFunc("/api/set-admin", apply(handlers.SetAdminHandler, adaptGeneralRateLimit, adminAuthMiddlewareFunc))
	mux.HandleFunc("/api/admin/verifications", apply(handlers.GetPendingVerificationsHandler, adaptGeneralRateLimit, adminAuthMiddlewareFunc))
	mux.HandleFunc("/api/admin/verify", apply(handlers.UpdateVerificationStatusHandler, adaptGeneralRateLimit, adminAuthMiddlewareFunc))

	mux.HandleFunc("/", apply(handlers.ProtectedHandler, adaptGeneralRateLimit, authMiddlewareFunc))

	mux.HandleFunc("/api/analytics/summary", apply(handlers.GetAnalyticsSummaryHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/analytics/spotlight", apply(handlers.GetSpotlightAnalyticsHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/analytics/log-like-profile-view", apply(handlers.LogLikeProfileViewHandler, adaptGeneralRateLimit, authMiddlewareFunc))
	mux.HandleFunc("/api/analytics/log-photo-views", apply(handlers.LogPhotoViewsHandler, adaptGeneralRateLimit, authMiddlewareFunc))

	return mux
}
