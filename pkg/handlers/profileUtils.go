// pkg/handlers/profile_utils.go (New File)
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// fetchFullUserProfileData retrieves the user's core data and all associated prompts.
// It assumes the user exists. Error handling for non-existent user should be done by the caller.
func fetchFullUserProfileData(ctx context.Context, queries *migrations.Queries, userID int32) (*UserProfileData, error) {
	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		// Let the caller handle pgx.ErrNoRows specifically if needed
		return nil, fmt.Errorf("failed to fetch user %d: %w", userID, err)
	}

	profileData := UserProfileData{User: user} // Initialize with base user data

	// Fetch prompts concurrently for potential minor speedup (optional)
	// Using errgroup might be cleaner for larger numbers of concurrent fetches
	var errDV, errGP, errMT, errST error
	var dvPrompts []migrations.DateVibesPrompt
	var gpPrompts []migrations.GettingPersonalPrompt
	var mtPrompts []migrations.MyTypePrompt
	var stPrompts []migrations.StoryTimePrompt

	// Consider using channels and goroutines if performance is critical,
	// but for 4 queries, sequential might be simpler and acceptable.
	dvPrompts, errDV = queries.GetUserDateVibesPrompts(ctx, userID)
	if errDV != nil && !errors.Is(errDV, pgx.ErrNoRows) {
		log.Printf("Error fetching DateVibes prompts for user %d: %v", userID, errDV)
		// Decide if partial data is acceptable or return error
	}
	profileData.DateVibesPrompts = dvPrompts

	gpPrompts, errGP = queries.GetUserGettingPersonalPrompts(ctx, userID)
	if errGP != nil && !errors.Is(errGP, pgx.ErrNoRows) {
		log.Printf("Error fetching GettingPersonal prompts for user %d: %v", userID, errGP)
	}
	profileData.GettingPersonalPrompts = gpPrompts

	mtPrompts, errMT = queries.GetUserMyTypePrompts(ctx, userID)
	if errMT != nil && !errors.Is(errMT, pgx.ErrNoRows) {
		log.Printf("Error fetching MyType prompts for user %d: %v", userID, errMT)
	}
	profileData.MyTypePrompts = mtPrompts

	stPrompts, errST = queries.GetUserStoryTimePrompts(ctx, userID)
	if errST != nil && !errors.Is(errST, pgx.ErrNoRows) {
		log.Printf("Error fetching StoryTime prompts for user %d: %v", userID, errST)
	}
	profileData.StoryTimePrompts = stPrompts

	// Check if any critical error occurred during prompt fetching if necessary
	// For now, we log errors but return the profile data obtained.

	return &profileData, nil
}

// Helper to safely get the first media URL
func getFirstMediaURL(mediaUrls []string) string {
	if len(mediaUrls) > 0 && mediaUrls[0] != "" {
		return mediaUrls[0]
	}
	return "" // Or a default placeholder URL
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
