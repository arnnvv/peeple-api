package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
)

type VerificationRequest struct {
	UserID          uint   `json:"user_id"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
	VerificationURL string `json:"verification_url"`
}

func GetPendingVerificationsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Println("\n=== Starting Get Pending Verifications (sqlc) ===")
	defer fmt.Println("=== End Get Pending Verifications (sqlc) ===")

	if r.Method != http.MethodGet {
		respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if dbPool == nil {
		fmt.Println("[Error] Database pool is not initialized")
		respondError(w, "Internal server error: DB not configured", http.StatusInternalServerError)
		return
	}
	q := migrations.New(dbPool)

	fmt.Println("[Database] Fetching pending verification users...")
	users, err := q.GetPendingVerificationUsers(ctx, migrations.VerificationStatusPending)
	if err != nil {
		fmt.Printf("[Database Error] Failed to fetch pending users: %v\n", err)
		respondError(w, "Failed to fetch verification requests", http.StatusInternalServerError)
		return
	}
	fmt.Printf("[Database] Found %d users with pending verification status.\n", len(users))

	var verificationRequests []VerificationRequest
	for _, user := range users {
		if !user.VerificationPic.Valid || user.VerificationPic.String == "" {
			fmt.Printf("[Filter] Skipping User ID %d: Missing or empty verification_pic\n", user.ID)
			continue
		}

		var profileImageURL string
		if len(user.MediaUrls) > 0 && user.MediaUrls[0] != "" { // Added check for empty string in URL
			profileImageURL = user.MediaUrls[0]
			fmt.Printf("[Data] User ID %d: Profile Image URL: %s\n", user.ID, profileImageURL)
		} else {
			fmt.Printf("[Data] User ID %d: No profile image URL found in media_urls\n", user.ID)
		}

		var nameBuilder strings.Builder
		if user.Name.Valid && user.Name.String != "" {
			nameBuilder.WriteString(user.Name.String)
			fmt.Printf("[Data] User ID %d: Name: %s\n", user.ID, user.Name.String)
		}
		if user.LastName.Valid && user.LastName.String != "" {
			if nameBuilder.Len() > 0 {
				nameBuilder.WriteString(" ")
			}
			nameBuilder.WriteString(user.LastName.String)
			fmt.Printf("[Data] User ID %d: Last Name: %s\n", user.ID, user.LastName.String)
		}
		name := nameBuilder.String()
		if name == "" {
			fmt.Printf("[Data] User ID %d: Both name and last name are missing or empty.\n", user.ID)
		}

		verificationRequests = append(verificationRequests, VerificationRequest{
			UserID:          uint(user.ID),
			Name:            name,
			ProfileImageURL: profileImageURL,
			VerificationURL: user.VerificationPic.String,
		})
		fmt.Printf("[Result] Added verification request for User ID %d\n", user.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]any{
		"success":               true,
		"verification_requests": verificationRequests,
		"count":                 len(verificationRequests),
	})
	if err != nil {
		fmt.Printf("[Error] Failed to encode response: %v\n", err)
	}
	fmt.Printf("[Success] Responded with %d verification requests.\n", len(verificationRequests))
}
