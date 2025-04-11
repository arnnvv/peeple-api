// FILE: main.go (Resolved)
package main

import (
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
	if port == "" {
		port = "8080" // Default port if not set
		log.Printf("Warning: PORT environment variable not set. Defaulting to %s", port)
	}

	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB()

	// Configure server
	server := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// --- Authentication Routes ---
	http.HandleFunc("/api/auth/google/verify", handlers.GoogleAuthHandler)
	http.HandleFunc("/api/auth-status", token.AuthMiddleware(handlers.CheckAuthStatus))
	http.HandleFunc("/token", token.GenerateTokenHandler) // Debug/Test

	// --- Core Profile & Feature Routes (Protected by internal JWT) ---
	http.HandleFunc("/", token.AuthMiddleware(handlers.ProtectedHandler)) // Example
	http.HandleFunc("/upload", token.AuthMiddleware(handlers.GeneratePresignedURLs))
	http.HandleFunc("/audio", token.AuthMiddleware(handlers.GenerateAudioPresignedURL))
	http.HandleFunc("/get-profile", token.AuthMiddleware(handlers.ProfileHandler))
	http.HandleFunc("/api/profile", token.AuthMiddleware(handlers.CreateProfile))                               // Onboarding Step 2
	http.HandleFunc("/api/profile/location-gender", token.AuthMiddleware(handlers.UpdateLocationGenderHandler)) // Onboarding Step 1
	http.HandleFunc("/verify", token.AuthMiddleware(handlers.GenerateVerificationPresignedURL))
	http.HandleFunc("/api/filters", token.AuthMiddleware(handlers.ApplyFiltersHandler))
	http.HandleFunc("/api/app-opened", token.AuthMiddleware(handlers.LogAppOpenHandler))
	http.HandleFunc("/api/homefeed", token.AuthMiddleware(handlers.GetHomeFeedHandler))   // Filtered Feed
	http.HandleFunc("/api/quickfeed", token.AuthMiddleware(handlers.GetQuickFeedHandler)) // Simple Feed
	http.HandleFunc("/api/like", token.AuthMiddleware(handlers.LikeHandler))
	http.HandleFunc("/api/dislike", token.AuthMiddleware(handlers.DislikeHandler))
	http.HandleFunc("/api/iap/verify", token.AuthMiddleware(handlers.VerifyPurchaseHandler))
	http.HandleFunc("/api/liker-profile/", token.AuthMiddleware(handlers.GetLikerProfileHandler))
	http.HandleFunc("/api/likes/received", token.AuthMiddleware(handlers.GetWhoLikedYouHandler))

	// --- Chat Route (from partner branch) ---
	http.HandleFunc("/chat", token.AuthMiddleware(handlers.ChatHandler)) // Added Chat Handler

	// --- Admin Routes (Protected by Admin middleware) ---
	http.HandleFunc("/api/set-admin", token.AdminAuthMiddleware(handlers.SetAdminHandler))
	http.HandleFunc("/api/admin/verifications", token.AdminAuthMiddleware(handlers.GetPendingVerificationsHandler))
	http.HandleFunc("/api/admin/verify", token.AdminAuthMiddleware(handlers.UpdateVerificationStatusHandler))

	// --- Basic Test Route ---
	http.HandleFunc("/test", handlers.TestHandler)

	log.Printf("Server is running on http://localhost:%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// Removed OTP cleanup function
