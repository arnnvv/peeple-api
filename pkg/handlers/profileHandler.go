package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/jackc/pgx/v5"
)

type UserProfileData struct {
	migrations.User
	DateVibesPrompts       []migrations.DateVibesPrompt       `json:"dateVibesPrompts,omitempty"`
	GettingPersonalPrompts []migrations.GettingPersonalPrompt `json:"gettingPersonalPrompts,omitempty"`
	MyTypePrompts          []migrations.MyTypePrompt          `json:"myTypePrompts,omitempty"`
	StoryTimePrompts       []migrations.StoryTimePrompt       `json:"storyTimePrompts,omitempty"`
	// AudioPrompt is already included in migrations.User (AudioPromptQuestion, AudioPromptAnswer)
}

type ProfileResponse struct {
	Success bool             `json:"success"`
	User    *UserProfileData `json:"user,omitempty"`
	Error   string           `json:"error,omitempty"`
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := ProfileResponse{}
	ctx := r.Context()
	queries := db.GetDB()

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ProfileResponse{Success: false, Error: "Unauthorized"})
		return
	}
	userID := int32(claims.UserID)

	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			resp.Success = false
			resp.Error = "User not found"
			w.WriteHeader(http.StatusNotFound)
		} else {
			resp.Success = false
			resp.Error = "Database error fetching user data"
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	var profileData UserProfileData
	profileData.User = user

	dateVibesPrompts, err := queries.GetUserDateVibesPrompts(ctx, userID)
	if err != nil && err != pgx.ErrNoRows {
		log.Printf("Error fetching DateVibes prompts for user %d: %v", userID, err)
	}
	profileData.DateVibesPrompts = dateVibesPrompts

	gettingPersonalPrompts, err := queries.GetUserGettingPersonalPrompts(ctx, userID)
	if err != nil && err != pgx.ErrNoRows {
		log.Printf("Error fetching GettingPersonal prompts for user %d: %v", userID, err)
	}
	profileData.GettingPersonalPrompts = gettingPersonalPrompts

	myTypePrompts, err := queries.GetUserMyTypePrompts(ctx, userID)
	if err != nil && err != pgx.ErrNoRows {
		log.Printf("Error fetching MyType prompts for user %d: %v", userID, err)
	}
	profileData.MyTypePrompts = myTypePrompts

	storyTimePrompts, err := queries.GetUserStoryTimePrompts(ctx, userID)
	if err != nil && err != pgx.ErrNoRows {
		log.Printf("Error fetching StoryTime prompts for user %d: %v", userID, err)
	}
	profileData.StoryTimePrompts = storyTimePrompts

	resp.Success = true
	resp.User = &profileData
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
