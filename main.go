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

	http.HandleFunc("/api/auth/google/verify", handlers.GoogleAuthHandler)
	http.HandleFunc("/api/auth-status", token.AuthMiddleware(handlers.CheckAuthStatus))
	http.HandleFunc("/token", token.GenerateTokenHandler) // Keep for testing/debug if needed

	http.HandleFunc("/", token.AuthMiddleware(handlers.ProtectedHandler)) // Basic protected route

	// --- Profile & Onboarding ---
	http.HandleFunc("/api/profile", token.AuthMiddleware(handlers.CreateProfile))                               // Onboarding Step 2 (details)
	http.HandleFunc("/api/profile/location-gender", token.AuthMiddleware(handlers.UpdateLocationGenderHandler)) // Onboarding Step 1
	http.HandleFunc("/get-profile", token.AuthMiddleware(handlers.ProfileHandler))                              // Fetch own profile
	http.HandleFunc("/api/profile/edit", token.AuthMiddleware(handlers.EditProfileHandler))                     // Edit existing profile

	// --- Media Uploads ---
	http.HandleFunc("/upload", token.AuthMiddleware(handlers.GeneratePresignedURLs))            // Original upload, saves to DB
	http.HandleFunc("/audio", token.AuthMiddleware(handlers.GenerateAudioPresignedURL))         // Audio upload, saves to DB
	http.HandleFunc("/verify", token.AuthMiddleware(handlers.GenerateVerificationPresignedURL)) // Verification image upload, saves to DB
	// *** RENAMED ROUTE AND HANDLER HERE ***
	http.HandleFunc("/api/edit-presigned-urls", token.AuthMiddleware(handlers.GenerateEditPresignedURLs)) // Generate URLs for editing, DOES NOT save to DB

	// --- Feed & Matching ---
	http.HandleFunc("/api/filters", token.AuthMiddleware(handlers.ApplyFiltersHandler))
	http.HandleFunc("/api/get-filters", token.AuthMiddleware(handlers.GetFiltersHandler))
	http.HandleFunc("/api/app-opened", token.AuthMiddleware(handlers.LogAppOpenHandler))
	http.HandleFunc("/api/homefeed", token.AuthMiddleware(handlers.GetHomeFeedHandler))
	http.HandleFunc("/api/quickfeed", token.AuthMiddleware(handlers.GetQuickFeedHandler))
	http.HandleFunc("/api/like", token.AuthMiddleware(handlers.LikeHandler))
	http.HandleFunc("/api/dislike", token.AuthMiddleware(handlers.DislikeHandler))
	http.HandleFunc("/api/unmatch", token.AuthMiddleware(handlers.UnmatchHandler))
	http.HandleFunc("/api/report", token.AuthMiddleware(handlers.ReportHandler))
	http.HandleFunc("/api/likes/received", token.AuthMiddleware(handlers.GetWhoLikedYouHandler))
	http.HandleFunc("/api/liker-profile/", token.AuthMiddleware(handlers.GetLikerProfileHandler)) // Note trailing slash for path matching
	http.HandleFunc("/api/matches", token.AuthMiddleware(handlers.GetMatchesHandler))

	// --- In-App Purchases ---
	http.HandleFunc("/api/iap/verify", token.AuthMiddleware(handlers.VerifyPurchaseHandler))

	// --- Chat ---
	http.HandleFunc("/chat", token.AuthMiddleware(handlers.ChatHandler)) // WebSocket

	// --- Admin ---
	http.HandleFunc("/api/set-admin", token.AdminAuthMiddleware(handlers.SetAdminHandler))
	http.HandleFunc("/api/admin/verifications", token.AdminAuthMiddleware(handlers.GetPendingVerificationsHandler))
	http.HandleFunc("/api/admin/verify", token.AdminAuthMiddleware(handlers.UpdateVerificationStatusHandler))

	// --- Test ---
	http.HandleFunc("/test", handlers.TestHandler)

	log.Printf("Server is running on http://localhost:%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
