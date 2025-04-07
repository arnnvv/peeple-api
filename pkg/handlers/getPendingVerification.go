package handlers

import (
	"encoding/json"
	"fmt"
	"log" // Import log package
	"net/http"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db" // Import db package
	// "github.com/jackc/pgx/v5/pgxpool" // No longer needed here
)

// var dbPool *pgxpool.Pool // REMOVED: Use db.GetDB() instead

type VerificationRequest struct {
	UserID          uint   `json:"user_id"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"` // Use first image from media_urls
	VerificationURL string `json:"verification_url"`
}

func GetPendingVerificationsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Println("\n=== Starting Get Pending Verifications (sqlc) ===")
	defer fmt.Println("=== End Get Pending Verifications (sqlc) ===")

	// Get DB Queries object
	queries := db.GetDB()
	if queries == nil {
		// Use the existing respondError or create one specific to this handler
		log.Println("[Error] Database pool is not initialized in GetPendingVerificationsHandler")
		http.Error(w, "Internal server error: DB not configured", http.StatusInternalServerError) // Keep it simple for now
		return
	}

	if r.Method != http.MethodGet {
		// respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed) // Keep it simple
		return
	}

	fmt.Println("[Database] Fetching pending verification users...")
	// Use the obtained queries object
	users, err := queries.GetPendingVerificationUsers(ctx, migrations.VerificationStatusPending)
	if err != nil {
		fmt.Printf("[Database Error] Failed to fetch pending users: %v\n", err)
		// respondError(w, "Failed to fetch verification requests", http.StatusInternalServerError)
		http.Error(w, "Failed to fetch verification requests", http.StatusInternalServerError) // Keep it simple
		return
	}
	fmt.Printf("[Database] Found %d users with pending verification status.\n", len(users))

	var verificationRequests []VerificationRequest
	for _, user := range users {
		// Ensure verification picture exists and is valid
		if !user.VerificationPic.Valid || user.VerificationPic.String == "" {
			fmt.Printf("[Filter] Skipping User ID %d: Missing or empty verification_pic\n", user.ID)
			continue
		}

		// Determine the first profile image URL safely
		var profileImageURL string
		if len(user.MediaUrls) > 0 && user.MediaUrls[0] != "" {
			profileImageURL = user.MediaUrls[0]
			// fmt.Printf("[Data] User ID %d: Profile Image URL: %s\n", user.ID, profileImageURL)
		} else {
			// fmt.Printf("[Data] User ID %d: No profile image URL found in media_urls\n", user.ID)
		}

		// Build name safely
		var nameBuilder strings.Builder
		if user.Name.Valid && user.Name.String != "" {
			nameBuilder.WriteString(user.Name.String)
		}
		if user.LastName.Valid && user.LastName.String != "" {
			if nameBuilder.Len() > 0 {
				nameBuilder.WriteString(" ") // Add space only if first name exists
			}
			nameBuilder.WriteString(user.LastName.String)
		}
		name := nameBuilder.String()
		// if name == "" {
		// 	fmt.Printf("[Data] User ID %d: Both name and last name are missing or empty.\n", user.ID)
		// }

		verificationRequests = append(verificationRequests, VerificationRequest{
			UserID:          uint(user.ID),
			Name:            name, // Will be empty if both name/lastname are null/empty
			ProfileImageURL: profileImageURL,
			VerificationURL: user.VerificationPic.String,
		})
		// fmt.Printf("[Result] Added verification request for User ID %d\n", user.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responsePayload := map[string]any{
		"success":               true,
		"verification_requests": verificationRequests,
		"count":                 len(verificationRequests),
	}
	err = json.NewEncoder(w).Encode(responsePayload)
	if err != nil {
		// Log the error, but the header/status is already sent
		fmt.Printf("[Error] Failed to encode response in GetPendingVerificationsHandler: %v\n", err)
	}
	fmt.Printf("[Success] Responded with %d verification requests.\n", len(verificationRequests))
}

// Note: The respondError function used in createprofile.go is not defined here.
// If needed, you can copy it or use http.Error directly as shown in the modified code above.
