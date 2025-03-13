package main

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/handlers"
	"github.com/arnnvv/peeple-api/pkg/token"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := db.InitDB(os.Getenv("DATABASE_URL")); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
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
	http.HandleFunc("/test", handlers.TestHandler)

	go cleanupExpiredOTPs()

	log.Println("Server is running on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func cleanupExpiredOTPs() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := db.DeleteExpiredOTPs(); err != nil {
			log.Printf("Error cleaning up expired OTPs: %v", err)
		}
	}
}
