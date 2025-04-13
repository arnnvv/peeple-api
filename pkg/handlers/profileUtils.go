// pkg/handlers/profileUtils.go
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings" // Needed for buildFullName & FormatHeight

	"github.com/arnnvv/peeple-api/migrations"
	// "github.com/arnnvv/peeple-api/pkg/db" // Not needed directly if queries is passed in
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype" // Needed for pgtype checks
)

// --- Define the internal representation of full profile data ---
// This combines the base user info with fetched prompts.
// Used internally and by handlers needing the complete data structure.
type UserProfileData struct {
	// Embed the base user struct fetched by GetUserByID
	migrations.User

	// Add slices for the fetched prompts (matching old structure)
	// These are added AFTER fetching the base user.
	// Use omitempty for cleaner JSON if these are embedded in responses directly,
	// though whoLikedYouHandler might construct its JSON differently.
	DateVibesPrompts       []migrations.DateVibesPrompt       `json:"dateVibesPrompts,omitempty"`
	GettingPersonalPrompts []migrations.GettingPersonalPrompt `json:"gettingPersonalPrompts,omitempty"`
	MyTypePrompts          []migrations.MyTypePrompt          `json:"myTypePrompts,omitempty"`
	StoryTimePrompts       []migrations.StoryTimePrompt       `json:"storyTimePrompts,omitempty"`

	// --- Add the combined prompts field for convenience if needed by callers ---
	// This duplicates data but matches the structure used in ProfileHandler's DTO
	Prompts []CombinedPrompt `json:"prompts,omitempty"`
}

// fetchFullUserProfileData retrieves the user's core data and all associated prompts.
// It assumes the user exists. Error handling for non-existent user should be done by the caller.
// Returns the locally defined UserProfileData struct.
func fetchFullUserProfileData(ctx context.Context, queries *migrations.Queries, userID int32) (*UserProfileData, error) {
	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		// Let the caller handle pgx.ErrNoRows specifically if needed
		return nil, fmt.Errorf("utils: failed to fetch user %d: %w", userID, err)
	}

	// Initialize with base user data
	profileData := UserProfileData{User: user}

	// Fetch prompts concurrently or sequentially
	var errDV, errGP, errMT, errST error
	var dvPrompts []migrations.DateVibesPrompt
	var gpPrompts []migrations.GettingPersonalPrompt
	var mtPrompts []migrations.MyTypePrompt
	var stPrompts []migrations.StoryTimePrompt
	var combinedPromptsFetch []CombinedPrompt // For the combined list

	// --- Fetch Prompts (Same as before) ---
	dvPrompts, errDV = queries.GetUserDateVibesPrompts(ctx, userID)
	if errDV != nil && !errors.Is(errDV, pgx.ErrNoRows) {
		log.Printf("utils: Error fetching DateVibes prompts for user %d: %v", userID, errDV)
	} else if errDV == nil {
		profileData.DateVibesPrompts = dvPrompts
		for _, p := range dvPrompts {
			combinedPromptsFetch = append(combinedPromptsFetch, CombinedPrompt{
				Category: "dateVibes", Question: string(p.Question), Answer: p.Answer,
			})
		}
	}

	gpPrompts, errGP = queries.GetUserGettingPersonalPrompts(ctx, userID)
	if errGP != nil && !errors.Is(errGP, pgx.ErrNoRows) {
		log.Printf("utils: Error fetching GettingPersonal prompts for user %d: %v", userID, errGP)
	} else if errGP == nil {
		profileData.GettingPersonalPrompts = gpPrompts
		for _, p := range gpPrompts {
			combinedPromptsFetch = append(combinedPromptsFetch, CombinedPrompt{
				Category: "gettingPersonal", Question: string(p.Question), Answer: p.Answer,
			})
		}
	}

	mtPrompts, errMT = queries.GetUserMyTypePrompts(ctx, userID)
	if errMT != nil && !errors.Is(errMT, pgx.ErrNoRows) {
		log.Printf("utils: Error fetching MyType prompts for user %d: %v", userID, errMT)
	} else if errMT == nil {
		profileData.MyTypePrompts = mtPrompts
		for _, p := range mtPrompts {
			combinedPromptsFetch = append(combinedPromptsFetch, CombinedPrompt{
				Category: "myType", Question: string(p.Question), Answer: p.Answer,
			})
		}
	}

	stPrompts, errST = queries.GetUserStoryTimePrompts(ctx, userID)
	if errST != nil && !errors.Is(errST, pgx.ErrNoRows) {
		log.Printf("utils: Error fetching StoryTime prompts for user %d: %v", userID, errST)
	} else if errST == nil {
		profileData.StoryTimePrompts = stPrompts
		for _, p := range stPrompts {
			combinedPromptsFetch = append(combinedPromptsFetch, CombinedPrompt{
				Category: "storyTime", Question: string(p.Question), Answer: p.Answer,
			})
		}
	}
	// Assign the combined list
	profileData.Prompts = combinedPromptsFetch

	// Check if any critical error occurred during prompt fetching if necessary

	return &profileData, nil
}

// --- Other helpers can stay here ---

// Helper to safely get the first media URL
func getFirstMediaURL(mediaUrls []string) string {
	if len(mediaUrls) > 0 && mediaUrls[0] != "" {
		return mediaUrls[0]
	}
	return ""
}

// Helper to build full name safely
func buildFullName(name, lastName pgtype.Text) string {
	var fullName strings.Builder
	if name.Valid && name.String != "" {
		fullName.WriteString(name.String)
	}
	if lastName.Valid && lastName.String != "" {
		if fullName.Len() > 0 {
			fullName.WriteString(" ")
		}
		fullName.WriteString(lastName.String)
	}
	return fullName.String()
}

// Helper function to convert height (assuming DB stores raw float inches)
// to the "F' I\"" string format. (Copied from profileHandler, maybe keep only one copy?)

// You might also need the CombinedPrompt struct here if UserProfileData uses it
// Or keep it defined in both handlers if preferred
// type CombinedPrompt struct { ... } // Defined in profileHandler.go
