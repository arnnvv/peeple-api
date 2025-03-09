package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
)

type VerificationRequest struct {
	UserID          uint   `json:"user_id"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
	VerificationURL string `json:"verification_url"`
}

func GetPendingVerificationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var users []db.UserModel
	result := db.DB.Where("verification_status = ?", enums.VerificationStatusPending).Find(&users)
	if result.Error != nil {
		http.Error(w, "Failed to fetch verification requests", http.StatusInternalServerError)
		return
	}

	var verificationRequests []VerificationRequest
	for _, user := range users {
		if user.VerificationPic == nil || *user.VerificationPic == "" {
			continue
		}

		var profileImageURL string
		if len(user.MediaURLs) > 0 {
			profileImageURL = user.MediaURLs[0]
		}

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

		verificationRequests = append(verificationRequests, VerificationRequest{
			UserID:          user.ID,
			Name:            name,
			ProfileImageURL: profileImageURL,
			VerificationURL: *user.VerificationPic,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":               true,
		"verification_requests": verificationRequests,
		"count":                 len(verificationRequests),
	})
}
