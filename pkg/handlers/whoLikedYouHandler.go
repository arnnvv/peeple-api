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

const maxFullProfiles = 10

type FullProfileLiker struct {
	LikerUserID int32              `json:"liker_user_id"`
	LikeComment *string            `json:"like_comment"`
	IsRose      bool               `json:"is_rose"`
	LikedAt     pgtype.Timestamptz `json:"liked_at"`
	FullProfile *UserProfileData   `json:"profile"`
}

type BasicProfileLiker struct {
	LikerUserID        int32              `json:"liker_user_id"`
	Name               string             `json:"name"`
	FirstProfilePicURL string             `json:"first_profile_pic_url"`
	LikeComment        *string            `json:"like_comment"`
	IsRose             bool               `json:"is_rose"`
	LikedAt            pgtype.Timestamptz `json:"liked_at"`
}

type WhoLikedYouResponse struct {
	Success      bool                `json:"success"`
	Message      string              `json:"message,omitempty"`
	FullProfiles []FullProfileLiker  `json:"full_profiles"`
	OtherLikers  []BasicProfileLiker `json:"other_likers"`
}

type LikerProfileResponse struct {
	Success     bool                    `json:"success"`
	Message     string                  `json:"message,omitempty"`
	LikeDetails *LikeInteractionDetails `json:"like_details,omitempty"`
	Profile     *UserProfileData        `json:"profile,omitempty"`
}

type LikeInteractionDetails struct {
	LikeComment *string `json:"like_comment"`
	IsRose      bool    `json:"is_rose"`
}

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
	likedUserID := int32(claims.UserID) // This is the user receiving the likes

	log.Printf("INFO: GetWhoLikedYouHandler: Fetching likers for user %d", likedUserID)

	likersBasicInfo, err := queries.GetLikersForUser(ctx, likedUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: GetWhoLikedYouHandler: Failed to fetch likers for user %d: %v", likedUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, WhoLikedYouResponse{Success: false, Message: "Error retrieving likes"})
		return
	}

	if errors.Is(err, pgx.ErrNoRows) || len(likersBasicInfo) == 0 {
		log.Printf("INFO: GetWhoLikedYouHandler: No likers found for user %d", likedUserID)
		utils.RespondWithJSON(w, http.StatusOK, WhoLikedYouResponse{
			Success:      true,
			FullProfiles: []FullProfileLiker{}, // Ensure empty slices, not null
			OtherLikers:  []BasicProfileLiker{},
		})
		return
	}

	log.Printf("INFO: GetWhoLikedYouHandler: Found %d likers for user %d", len(likersBasicInfo), likedUserID)

	fullProfiles := make([]FullProfileLiker, 0, maxFullProfiles)

	otherLikersCap := 0
	if len(likersBasicInfo) > maxFullProfiles {
		otherLikersCap = len(likersBasicInfo) - maxFullProfiles
	}
	otherLikers := make([]BasicProfileLiker, 0, otherLikersCap)

	for i, basicInfo := range likersBasicInfo {
		isRose := basicInfo.InteractionType == migrations.LikeInteractionTypeRose
		var commentPtr *string
		if basicInfo.Comment.Valid {
			commentPtr = &basicInfo.Comment.String // Assign address if valid
		}

		likerName := buildFullName(basicInfo.Name, basicInfo.LastName)
		likerPic := getFirstMediaURL(basicInfo.MediaUrls)

		if i < maxFullProfiles {
			log.Printf("DEBUG: Fetching full profile for liker %d (index %d)", basicInfo.LikerUserID, i)
			fullProfileData, profileErr := fetchFullUserProfileData(ctx, queries, basicInfo.LikerUserID)
			if profileErr != nil {
				log.Printf("ERROR: GetWhoLikedYouHandler: Failed to fetch full profile for liker %d: %v. Adding basic info instead.", basicInfo.LikerUserID, profileErr)
				otherLikers = append(otherLikers, BasicProfileLiker{
					LikerUserID:        basicInfo.LikerUserID,
					Name:               likerName,
					FirstProfilePicURL: likerPic,
					LikeComment:        commentPtr,
					IsRose:             isRose,
					LikedAt:            basicInfo.LikedAt,
				})
				continue // Skip adding to fullProfiles
			}

			fullProfiles = append(fullProfiles, FullProfileLiker{
				LikerUserID: basicInfo.LikerUserID,
				LikeComment: commentPtr,
				IsRose:      isRose,
				LikedAt:     basicInfo.LikedAt,
				FullProfile: fullProfileData,
			})
		} else {
			otherLikers = append(otherLikers, BasicProfileLiker{
				LikerUserID:        basicInfo.LikerUserID,
				Name:               likerName,
				FirstProfilePicURL: likerPic,
				LikeComment:        commentPtr,
				IsRose:             isRose,
				LikedAt:            basicInfo.LikedAt,
			})
		}
	}

	log.Printf("INFO: GetWhoLikedYouHandler: Responding with %d full profiles and %d basic profiles for user %d", len(fullProfiles), len(otherLikers), likedUserID)

	utils.RespondWithJSON(w, http.StatusOK, WhoLikedYouResponse{
		Success:      true,
		FullProfiles: fullProfiles,
		OtherLikers:  otherLikers,
	})
}

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
	currentUserLikerID := int32(claims.UserID)

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
	targetLikerUserID := int32(likerID)

	log.Printf("INFO: GetLikerProfileHandler: User %d requesting profile of liker %d", currentUserLikerID, targetLikerUserID)

	likeDetails, err := queries.GetLikeDetails(ctx, migrations.GetLikeDetailsParams{
		LikerUserID: targetLikerUserID,
		LikedUserID: currentUserLikerID,
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

	fullProfileData, err := fetchFullUserProfileData(ctx, queries, targetLikerUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("ERROR: GetLikerProfileHandler: Liker user %d not found despite like existing (data inconsistency?)", targetLikerUserID)
			utils.RespondWithJSON(w, http.StatusNotFound, LikerProfileResponse{Success: false, Message: "Liker user profile not found."})
		} else {
			log.Printf("ERROR: GetLikerProfileHandler: Failed to fetch full profile for liker %d: %v", targetLikerUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, LikerProfileResponse{Success: false, Message: "Error retrieving liker profile"})
		}
		return
	}

	var commentPtr *string
	if likeDetails.Comment.Valid {
		commentPtr = &likeDetails.Comment.String
	}
	interactionDetails := LikeInteractionDetails{
		LikeComment: commentPtr,
		IsRose:      likeDetails.InteractionType == migrations.LikeInteractionTypeRose,
	}

	log.Printf("INFO: GetLikerProfileHandler: Successfully fetched profile for liker %d", targetLikerUserID)
	utils.RespondWithJSON(w, http.StatusOK, LikerProfileResponse{
		Success:     true,
		LikeDetails: &interactionDetails,
		Profile:     fullProfileData,
	})
}
