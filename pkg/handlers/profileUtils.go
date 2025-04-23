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

type UserProfileData struct {
	migrations.User

	DateVibesPrompts       []migrations.DateVibesPrompt       `json:"dateVibesPrompts,omitempty"`
	GettingPersonalPrompts []migrations.GettingPersonalPrompt `json:"gettingPersonalPrompts,omitempty"`
	MyTypePrompts          []migrations.MyTypePrompt          `json:"myTypePrompts,omitempty"`
	StoryTimePrompts       []migrations.StoryTimePrompt       `json:"storyTimePrompts,omitempty"`

	Prompts []CombinedPrompt `json:"prompts,omitempty"`
}

func fetchFullUserProfileData(ctx context.Context, queries *migrations.Queries, userID int32) (*UserProfileData, error) {
	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("utils: failed to fetch user %d: %w", userID, err)
	}

	profileData := UserProfileData{User: user}

	var errDV, errGP, errMT, errST error
	var dvPrompts []migrations.DateVibesPrompt
	var gpPrompts []migrations.GettingPersonalPrompt
	var mtPrompts []migrations.MyTypePrompt
	var stPrompts []migrations.StoryTimePrompt
	var combinedPromptsFetch []CombinedPrompt

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
	profileData.Prompts = combinedPromptsFetch

	return &profileData, nil
}

func getFirstMediaURL(mediaUrls []string) string {
	if len(mediaUrls) > 0 && mediaUrls[0] != "" {
		return mediaUrls[0]
	}
	return ""
}

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
