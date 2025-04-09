package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/handlers"
	"github.com/arnnvv/peeple-api/pkg/token"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	port := os.Getenv("PORT")

	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB()

	server := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	http.HandleFunc("/", token.AuthMiddleware(handlers.ProtectedHandler))
	http.HandleFunc("/token", token.GenerateTokenHandler)
	http.HandleFunc("/upload", token.AuthMiddleware(handlers.GeneratePresignedURLs))
	http.HandleFunc("/audio", token.AuthMiddleware(handlers.GenerateAudioPresignedURL))
	http.HandleFunc("/get-profile", token.AuthMiddleware(handlers.ProfileHandler))
	http.HandleFunc("/api/profile", token.AuthMiddleware(handlers.CreateProfile))
	http.HandleFunc("/api/auth-status", token.AuthMiddleware(handlers.CheckAuthStatus))
	http.HandleFunc("/verify", token.AuthMiddleware(handlers.GenerateVerificationPresignedURL))
	http.HandleFunc("/api/send-otp", handlers.SendOTP)
	http.HandleFunc("/api/verify-otp", handlers.VerifyOTP)
	http.HandleFunc("/api/set-admin", token.AdminAuthMiddleware(handlers.SetAdminHandler))
	http.HandleFunc("/api/admin/verifications", token.AdminAuthMiddleware(handlers.GetPendingVerificationsHandler))
	http.HandleFunc("/api/admin/verify", token.AdminAuthMiddleware(handlers.UpdateVerificationStatusHandler))
	http.HandleFunc("/api/filters", token.AuthMiddleware(handlers.ApplyFiltersHandler))
	http.HandleFunc("/api/app-opened", token.AuthMiddleware(handlers.LogAppOpenHandler))
	http.HandleFunc("/api/homefeed", token.AuthMiddleware(handlers.GetHomeFeedHandler))
	http.HandleFunc("/test", handlers.TestHandler)

	http.HandleFunc("/api/like", token.AuthMiddleware(handlers.LikeHandler))
	http.HandleFunc("/api/dislike", token.AuthMiddleware(handlers.DislikeHandler))
	http.HandleFunc("/api/iap/verify", token.AuthMiddleware(handlers.VerifyPurchaseHandler))
	http.HandleFunc("/chat", handlers.ChatHandler)

	go cleanupExpiredOTPs()

	log.Printf("Server is running on http://localhost:%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func cleanupExpiredOTPs() {
	for db.GetDB() == nil {
		log.Println("Waiting for DB initialization before starting OTP cleanup...")
		time.Sleep(2 * time.Second)
	}
	log.Println("DB initialized, starting OTP cleanup routine.")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		queries := db.GetDB()
		if queries != nil {
			log.Println("Running OTP cleanup...")
			err := queries.CleanOTPs(context.Background())
			if err != nil {
				log.Printf("Error during OTP cleanup: %v", err)
			} else {
				log.Println("OTP cleanup finished.")
			}
		} else {
			log.Println("Skipping OTP cleanup run, DB connection not available.")
		}
	}
}
