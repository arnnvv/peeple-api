// File: pkg/handlers/whoLikedYouHandler.go

package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxFullProfiles = 10 // Assuming this constant is appropriate

// --- MODIFIED Structs to include LikeID ---

// FullProfileLikerResponseItem represents a liker whose full profile is included.
type FullProfileLikerResponseItem struct {
	LikerUserID int32              `json:"liker_user_id"`
	LikeComment *string            `json:"like_comment"` // Use pointer for nullable string
	IsRose      bool               `json:"is_rose"`
	LikedAt     pgtype.Timestamptz `json:"liked_at"`
	FullProfile *UserProfileData   `json:"profile"` // Assumes UserProfileData is defined (can use fetchFullUserProfileData result)
	LikeID      int32              `json:"like_id"` // <<< ADDED FIELD WITH JSON TAG
}

// BasicProfileLikerResponseItem represents a liker where only basic info is included.
type BasicProfileLikerResponseItem struct {
	LikerUserID        int32              `json:"liker_user_id"`
	Name               string             `json:"name"`
	FirstProfilePicURL string             `json:"first_profile_pic_url"`
	LikeComment        *string            `json:"like_comment"` // Use pointer for nullable string
	IsRose             bool               `json:"is_rose"`
	LikedAt            pgtype.Timestamptz `json:"liked_at"`
	LikeID             int32              `json:"like_id"` // <<< ADDED FIELD WITH JSON TAG
}

// --- END MODIFIED Structs ---

// WhoLikedYouResponse holds the overall structure for the /api/likes/received endpoint.
type WhoLikedYouResponse struct {
	Success      bool                            `json:"success"`
	Message      string                          `json:"message,omitempty"`
	FullProfiles []FullProfileLikerResponseItem  `json:"full_profiles"` // Use modified struct
	OtherLikers  []BasicProfileLikerResponseItem `json:"other_likers"`  // Use modified struct
}

// LikerProfileResponse structure (used by GetLikerProfileHandler, unchanged here)
type LikerProfileResponse struct {
	Success     bool                    `json:"success"`
	Message     string                  `json:"message,omitempty"`
	LikeDetails *LikeInteractionDetails `json:"like_details,omitempty"`
	Profile     *UserProfileData        `json:"profile,omitempty"` // Assumes UserProfileData can be marshalled
}

// LikeInteractionDetails structure (used by GetLikerProfileHandler, unchanged here)
type LikeInteractionDetails struct {
	LikeComment *string `json:"like_comment"` // Use pointer for nullable string
	IsRose      bool    `json:"is_rose"`
}

// GetWhoLikedYouHandler fetches users who liked the authenticated user.
func GetWhoLikedYouHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
	if queries == nil {
		log.Println("ERROR: GetWhoLikedYouHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, WhoLikedYouResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, WhoLikedYouResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, WhoLikedYouResponse{Success: false, Message: "Authentication required"})
		return
	}
	likedUserID := int32(claims.UserID) // This is the user whose likes are being viewed

	log.Printf("INFO: GetWhoLikedYouHandler: Fetching likers for user %d", likedUserID)

	likersBasicInfo, err := queries.GetLikersForUser(ctx, likedUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: GetWhoLikedYouHandler: Failed to fetch likers for user %d: %v", likedUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, WhoLikedYouResponse{Success: false, Message: "Error retrieving likes"})
		return
	}

	// Handle case where there are no likers
	if errors.Is(err, pgx.ErrNoRows) || len(likersBasicInfo) == 0 {
		log.Printf("INFO: GetWhoLikedYouHandler: No likers found for user %d", likedUserID)
		utils.RespondWithJSON(w, http.StatusOK, WhoLikedYouResponse{
			Success:      true,
			FullProfiles: []FullProfileLikerResponseItem{},  // Send empty slices
			OtherLikers:  []BasicProfileLikerResponseItem{}, // Send empty slices
		})
		return
	}

	log.Printf("INFO: GetWhoLikedYouHandler: Found %d likers for user %d", len(likersBasicInfo), likedUserID)

	// Initialize slices with the modified struct types
	fullProfiles := make([]FullProfileLikerResponseItem, 0, maxFullProfiles)
	otherLikersCap := 0
	if len(likersBasicInfo) > maxFullProfiles {
		otherLikersCap = len(likersBasicInfo) - maxFullProfiles
	}
	otherLikers := make([]BasicProfileLikerResponseItem, 0, otherLikersCap)

	// --- Loop and Populate - MODIFIED to include LikeID ---
	for i, basicInfo := range likersBasicInfo {
		isRose := basicInfo.InteractionType == migrations.LikeInteractionTypeRose
		var commentPtr *string
		if basicInfo.Comment.Valid {
			tmp := basicInfo.Comment.String // Create temp var for pointer
			commentPtr = &tmp
		}

		likerName := buildFullName(basicInfo.Name, basicInfo.LastName) // Assumes buildFullName exists
		likerPic := getFirstMediaURL(basicInfo.MediaUrls)              // Assumes getFirstMediaURL exists

		if i < maxFullProfiles {
			log.Printf("DEBUG: Fetching full profile for liker %d (index %d)", basicInfo.LikerUserID, i)
			// Assuming fetchFullUserProfileData returns *UserProfileData, error
			fullProfileData, profileErr := fetchFullUserProfileData(ctx, queries, basicInfo.LikerUserID)
			if profileErr != nil {
				log.Printf("ERROR: GetWhoLikedYouHandler: Failed to fetch full profile for liker %d: %v. Adding basic info instead.", basicInfo.LikerUserID, profileErr)
				// Add to otherLikers if full profile fetch fails
				otherLikers = append(otherLikers, BasicProfileLikerResponseItem{
					LikerUserID:        basicInfo.LikerUserID,
					Name:               likerName,
					FirstProfilePicURL: likerPic,
					LikeComment:        commentPtr,
					IsRose:             isRose,
					LikedAt:            basicInfo.LikedAt,
					LikeID:             basicInfo.LikeID, // <<< ADDED LIKE ID HERE TOO
				})
				continue // Skip adding to full profiles
			}

			// Add to full profiles list
			fullProfiles = append(fullProfiles, FullProfileLikerResponseItem{
				LikerUserID: basicInfo.LikerUserID,
				LikeComment: commentPtr,
				IsRose:      isRose,
				LikedAt:     basicInfo.LikedAt,
				FullProfile: fullProfileData,  // Assign fetched profile data
				LikeID:      basicInfo.LikeID, // <<< ASSIGNED LikeID
			})
		} else {
			// Add to basic likers list
			otherLikers = append(otherLikers, BasicProfileLikerResponseItem{
				LikerUserID:        basicInfo.LikerUserID,
				Name:               likerName,
				FirstProfilePicURL: likerPic,
				LikeComment:        commentPtr,
				IsRose:             isRose,
				LikedAt:            basicInfo.LikedAt,
				LikeID:             basicInfo.LikeID, // <<< ASSIGNED LikeID
			})
		}
	}
	// --- End Loop ---

	log.Printf("INFO: GetWhoLikedYouHandler: Responding with %d full profiles and %d basic profiles for user %d", len(fullProfiles), len(otherLikers), likedUserID)

	// Respond with the populated lists
	utils.RespondWithJSON(w, http.StatusOK, WhoLikedYouResponse{
		Success:      true,
		FullProfiles: fullProfiles,
		OtherLikers:  otherLikers,
	})
}

// GetLikerProfileHandler remains unchanged - it doesn't need to return like_id itself
func GetLikerProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, er := db.GetDB()
	if er != nil {
		log.Println("ERROR: GetLikerProfileHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, LikerProfileResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, LikerProfileResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, LikerProfileResponse{Success: false, Message: "Authentication required"})
		return
	}
	currentUserLikerID := int32(claims.UserID) // This is the user VIEWING the profile

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "api" || pathParts[1] != "liker-profile" {
		log.Printf("ERROR: GetLikerProfileHandler: Invalid URL path structure: %s", r.URL.Path)
		utils.RespondWithJSON(w, http.StatusBadRequest, LikerProfileResponse{Success: false, Message: "Invalid request URL format"})
		return
	}
	likerIDStr := pathParts[len(pathParts)-1]

	likerID, err := strconv.ParseInt(likerIDStr, 10, 32)
	if err != nil {
		log.Printf("ERROR: GetLikerProfileHandler: Invalid liker_user_id in path '%s': %v", likerIDStr, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, LikerProfileResponse{Success: false, Message: "Invalid liker user ID in URL"})
		return
	}
	targetLikerUserID := int32(likerID) // This is the user whose profile is being viewed

	log.Printf("INFO: GetLikerProfileHandler: User %d requesting profile of liker %d", currentUserLikerID, targetLikerUserID)

	// Check if the target user actually liked the current user
	likeDetails, err := queries.GetLikeDetails(ctx, migrations.GetLikeDetailsParams{
		LikerUserID: targetLikerUserID,  // The profile being viewed must be the liker
		LikedUserID: currentUserLikerID, // The current user must be the liked one
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: GetLikerProfileHandler: Like not found from user %d to user %d", targetLikerUserID, currentUserLikerID)
			utils.RespondWithJSON(w, http.StatusNotFound, LikerProfileResponse{Success: false, Message: "This user has not liked you or the like does not exist."})
		} else {
			log.Printf("ERROR: GetLikerProfileHandler: Failed to verify like from %d to %d: %v", targetLikerUserID, currentUserLikerID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, LikerProfileResponse{Success: false, Message: "Error checking like status"})
		}
		return
	}

	// Fetch the full profile of the target liker user
	// Assume fetchFullUserProfileData returns *UserProfileData, error
	fullProfileData, err := fetchFullUserProfileData(ctx, queries, targetLikerUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// This is less likely if the like exists, but handle defensively
			log.Printf("ERROR: GetLikerProfileHandler: Liker user %d not found despite like existing (data inconsistency?)", targetLikerUserID)
			utils.RespondWithJSON(w, http.StatusNotFound, LikerProfileResponse{Success: false, Message: "Liker user profile not found."})
		} else {
			log.Printf("ERROR: GetLikerProfileHandler: Failed to fetch full profile for liker %d: %v", targetLikerUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, LikerProfileResponse{Success: false, Message: "Error retrieving liker profile"})
		}
		return
	}

	// Prepare the like interaction details part of the response
	var commentPtr *string
	if likeDetails.Comment.Valid {
		tmp := likeDetails.Comment.String // Create temp var for pointer
		commentPtr = &tmp
	}
	interactionDetails := LikeInteractionDetails{
		LikeComment: commentPtr,
		IsRose:      likeDetails.InteractionType == migrations.LikeInteractionTypeRose,
	}

	log.Printf("INFO: GetLikerProfileHandler: Successfully fetched profile for liker %d", targetLikerUserID)

	// Respond with success, including both profile and like details
	utils.RespondWithJSON(w, http.StatusOK, LikerProfileResponse{
		Success:     true,
		LikeDetails: &interactionDetails,
		Profile:     fullProfileData, // Send the fetched full profile
	})
}
