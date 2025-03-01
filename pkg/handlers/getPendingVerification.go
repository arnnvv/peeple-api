
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
)

// VerificationRequest represents user data needed for verification
type VerificationRequest struct {
	UserID          uint   `json:"user_id"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
	VerificationURL string `json:"verification_url"`
}

// GetPendingVerificationsHandler handles the request to get all pending verifications
func GetPendingVerificationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Find all users with pending verification status
	var users []db.UserModel
	result := db.DB.Where("verification_status = ?", enums.VerificationStatusPending).Find(&users)
	if result.Error != nil {
		http.Error(w, "Failed to fetch verification requests", http.StatusInternalServerError)
		return
	}

	// Build response data
	var verificationRequests []VerificationRequest
	for _, user := range users {
		// Skip users with missing required data
		if user.VerificationPic == nil || *user.VerificationPic == "" {
			continue
		}

		// Get profile image (first media URL or empty string)
		var profileImageURL string
		if len(user.MediaURLs) > 0 {
			profileImageURL = user.MediaURLs[0]
		}

		// Build user's name (handle nil values)
		name := ""
		if user.Name != nil {
			name = *user.Name
		}
		if user.LastName != nil && *user.LastName != "" {
			if name != "" {
				name += " "
			}
			name += *user.LastName
		}

		// Add to request list
		verificationRequests = append(verificationRequests, VerificationRequest{
			UserID:          user.ID,
			Name:            name,
			ProfileImageURL: profileImageURL,
			VerificationURL: *user.VerificationPic,
		})
	}

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"verification_requests": verificationRequests,
		"count":        len(verificationRequests),
	})
}
