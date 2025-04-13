package handlers

import (
	"errors"
	"fmt" // Added for FormatHeight
	"log"
	"math" // Added for FormatHeight
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype" // Need this for pgtype checks
)

// ========================================================================
// DTOs and Helpers Defined LOCALLY within the handler (or a dto package)
// ========================================================================

// --- Custom Null Types for Consistent JSON Marshalling ---

// NullString for pgtype.Text
type NullString struct {
	String string `json:"String"`
	Valid  bool   `json:"Valid"`
}

func NewNullString(pt pgtype.Text) *NullString {
	if !pt.Valid {
		return nil // Omit if null (using omitempty in ProfileResponseUser)
	}
	return &NullString{String: pt.String, Valid: true}
}

// NullDate for pgtype.Date
type NullDate struct {
	Time  string `json:"Time"` // Format as YYYY-MM-DD string for JSON
	Valid bool   `json:"Valid"`
}

func NewNullDate(pd pgtype.Date) *NullDate {
	if !pd.Valid || pd.Time.IsZero() { // Also check if time is zero
		return nil
	}
	return &NullDate{Time: pd.Time.Format("2006-01-02"), Valid: true}
}

// NullFloat64 for pgtype.Float8
type NullFloat64 struct {
	Float64 float64 `json:"Float64"`
	Valid   bool    `json:"Valid"`
}

func NewNullFloat64(pf pgtype.Float8) *NullFloat64 {
	if !pf.Valid {
		return nil
	}
	return &NullFloat64{Float64: pf.Float64, Valid: true}
}

// --- Custom JSON structs for Enums ---

// NullGenderEnumJSON for gender
type NullGenderEnumJSON struct {
	GenderEnum migrations.GenderEnum `json:"GenderEnum"` // Use the type from migrations
	Valid      bool                  `json:"Valid"`
}

func NewNullGenderEnumJSON(ng migrations.NullGenderEnum) *NullGenderEnumJSON {
	if !ng.Valid {
		return nil
	}
	return &NullGenderEnumJSON{GenderEnum: ng.GenderEnum, Valid: true}
}

// NullDatingIntentionJSON for dating intention
type NullDatingIntentionJSON struct {
	DatingIntention migrations.DatingIntention `json:"DatingIntention"`
	Valid           bool                       `json:"Valid"`
}

func NewNullDatingIntentionJSON(ndi migrations.NullDatingIntention) *NullDatingIntentionJSON {
	if !ndi.Valid {
		return nil
	}
	return &NullDatingIntentionJSON{DatingIntention: ndi.DatingIntention, Valid: true}
}

// NullReligionJSON for religion
type NullReligionJSON struct {
	Religion migrations.Religion `json:"Religion"`
	Valid    bool                `json:"Valid"`
}

func NewNullReligionJSON(nr migrations.NullReligion) *NullReligionJSON {
	if !nr.Valid {
		return nil
	}
	return &NullReligionJSON{Religion: nr.Religion, Valid: true}
}

// NullHabitJSON for drinking/smoking habits
type NullHabitJSON struct {
	DrinkingSmokingHabits migrations.DrinkingSmokingHabits `json:"DrinkingSmokingHabits"`
	Valid                 bool                             `json:"Valid"`
}

func NewNullHabitJSON(nh migrations.NullDrinkingSmokingHabits) *NullHabitJSON {
	if !nh.Valid {
		return nil
	}
	return &NullHabitJSON{DrinkingSmokingHabits: nh.DrinkingSmokingHabits, Valid: true}
}

// NullAudioPromptJSON for audio prompt question
type NullAudioPromptJSON struct {
	AudioPrompt migrations.AudioPrompt `json:"AudioPrompt"`
	Valid       bool                   `json:"Valid"`
}

func NewNullAudioPromptJSON(nap migrations.NullAudioPrompt) *NullAudioPromptJSON {
	if !nap.Valid {
		return nil
	}
	return &NullAudioPromptJSON{AudioPrompt: nap.AudioPrompt, Valid: true}
}

// Helper function to convert height (assuming DB stores raw float inches)
// to the "F' I\"" string format.
func FormatHeight(height pgtype.Float8) pgtype.Text {
	if !height.Valid {
		return pgtype.Text{Valid: false}
	}
	totalInches := height.Float64
	if totalInches <= 0 {
		return pgtype.Text{Valid: false}
	}
	feet := math.Floor(totalInches / 12)
	inches := math.Round(math.Mod(totalInches, 12))

	if inches == 12 {
		feet++
		inches = 0
	}

	return pgtype.Text{
		String: fmt.Sprintf("%.0f' %.0f\"", feet, inches),
		Valid:  true,
	}
}

// --- API Response Structures ---

// CombinedPrompt structure matching Flutter's expectation
type CombinedPrompt struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// ProfileResponseUser is the DTO for the user object in the response
type ProfileResponseUser struct {
	ID                  int32                    `json:"id"` // Or "ID" if Flutter expects uppercase
	CreatedAt           pgtype.Timestamptz       `json:"created_at"`
	Name                *NullString              `json:"name,omitempty"` // omitempty removes the key if nil
	LastName            *NullString              `json:"last_name,omitempty"`
	Email               string                   `json:"email"`
	DateOfBirth         *NullDate                `json:"date_of_birth,omitempty"`
	Latitude            *NullFloat64             `json:"latitude,omitempty"`
	Longitude           *NullFloat64             `json:"longitude,omitempty"`
	Gender              *NullGenderEnumJSON      `json:"gender,omitempty"`
	DatingIntention     *NullDatingIntentionJSON `json:"dating_intention,omitempty"`
	Height              *NullString              `json:"height,omitempty"` // Sending formatted string
	Hometown            *NullString              `json:"hometown,omitempty"`
	JobTitle            *NullString              `json:"job_title,omitempty"`
	Education           *NullString              `json:"education,omitempty"`
	ReligiousBeliefs    *NullReligionJSON        `json:"religious_beliefs,omitempty"`
	DrinkingHabit       *NullHabitJSON           `json:"drinking_habit,omitempty"`
	SmokingHabit        *NullHabitJSON           `json:"smoking_habit,omitempty"`
	MediaUrls           []string                 `json:"media_urls"` // Keep as simple array
	VerificationStatus  string                   `json:"verification_status"`
	VerificationPic     *NullString              `json:"verification_pic,omitempty"`
	Role                string                   `json:"role"`
	AudioPromptQuestion *NullAudioPromptJSON     `json:"audio_prompt_question,omitempty"`
	AudioPromptAnswer   *NullString              `json:"audio_prompt_answer,omitempty"`
	Prompts             []CombinedPrompt         `json:"prompts"` // Include prompts directly
}

// ProfileResponse is the top-level API response structure
type ProfileResponse struct {
	Success bool                 `json:"success"`
	User    *ProfileResponseUser `json:"user,omitempty"` // Use the DTO struct
	Message string               `json:"message,omitempty"`
}

// ========================================================================
// END of DTOs and Helpers
// ========================================================================

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
		log.Println("ERROR: ProfileHandler: Database connection not available.")
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection error")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID := int32(claims.UserID)

	// 1. Fetch User data from DB using sqlc-generated struct
	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithError(w, http.StatusNotFound, "User not found")
		} else {
			log.Printf("ERROR: ProfileHandler: Error fetching user %d: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Database error fetching user data")
		}
		return
	}

	// 2. Fetch Prompts (Keep this part)
	var combinedPrompts []CombinedPrompt
	// Fetch DateVibes prompts... (code omitted for brevity, same as before)
	dateVibesPrompts, errDV := queries.GetUserDateVibesPrompts(ctx, userID)
	if errDV != nil && !errors.Is(errDV, pgx.ErrNoRows) {
		log.Printf("Error fetching DateVibes prompts for user %d: %v", userID, errDV)
	} else if errDV == nil {
		for _, p := range dateVibesPrompts {
			combinedPrompts = append(combinedPrompts, CombinedPrompt{
				Category: "dateVibes",
				Question: string(p.Question),
				Answer:   p.Answer,
			})
		}
	}
	// Fetch GettingPersonal prompts... (code omitted for brevity, same as before)
	gettingPersonalPrompts, errGP := queries.GetUserGettingPersonalPrompts(ctx, userID)
	if errGP != nil && !errors.Is(errGP, pgx.ErrNoRows) {
		log.Printf("Error fetching GettingPersonal prompts for user %d: %v", userID, errGP)
	} else if errGP == nil {
		for _, p := range gettingPersonalPrompts {
			combinedPrompts = append(combinedPrompts, CombinedPrompt{
				Category: "gettingPersonal",
				Question: string(p.Question),
				Answer:   p.Answer,
			})
		}
	}
	// Fetch MyType prompts... (code omitted for brevity, same as before)
	myTypePrompts, errMT := queries.GetUserMyTypePrompts(ctx, userID)
	if errMT != nil && !errors.Is(errMT, pgx.ErrNoRows) {
		log.Printf("Error fetching MyType prompts for user %d: %v", userID, errMT)
	} else if errMT == nil {
		for _, p := range myTypePrompts {
			combinedPrompts = append(combinedPrompts, CombinedPrompt{
				Category: "myType",
				Question: string(p.Question),
				Answer:   p.Answer,
			})
		}
	}
	// Fetch StoryTime prompts... (code omitted for brevity, same as before)
	storyTimePrompts, errST := queries.GetUserStoryTimePrompts(ctx, userID)
	if errST != nil && !errors.Is(errST, pgx.ErrNoRows) {
		log.Printf("Error fetching StoryTime prompts for user %d: %v", userID, errST)
	} else if errST == nil {
		for _, p := range storyTimePrompts {
			combinedPrompts = append(combinedPrompts, CombinedPrompt{
				Category: "storyTime",
				Question: string(p.Question),
				Answer:   p.Answer,
			})
		}
	}

	// 3. Construct the explicit JSON response DTO structure
	responseUser := ProfileResponseUser{
		ID:                  user.ID, // Use the correct field name if needed
		CreatedAt:           user.CreatedAt,
		Email:               user.Email,
		Name:                NewNullString(user.Name),
		LastName:            NewNullString(user.LastName),
		DateOfBirth:         NewNullDate(user.DateOfBirth),
		Latitude:            NewNullFloat64(user.Latitude),
		Longitude:           NewNullFloat64(user.Longitude),
		Gender:              NewNullGenderEnumJSON(user.Gender),
		DatingIntention:     NewNullDatingIntentionJSON(user.DatingIntention),
		Height:              NewNullString(FormatHeight(user.Height)), // Format height to string
		Hometown:            NewNullString(user.Hometown),
		JobTitle:            NewNullString(user.JobTitle),
		Education:           NewNullString(user.Education),
		ReligiousBeliefs:    NewNullReligionJSON(user.ReligiousBeliefs),
		DrinkingHabit:       NewNullHabitJSON(user.DrinkingHabit),
		SmokingHabit:        NewNullHabitJSON(user.SmokingHabit),
		MediaUrls:           user.MediaUrls, // Assign directly
		VerificationStatus:  string(user.VerificationStatus),
		VerificationPic:     NewNullString(user.VerificationPic),
		Role:                string(user.Role),
		AudioPromptQuestion: NewNullAudioPromptJSON(user.AudioPromptQuestion),
		AudioPromptAnswer:   NewNullString(user.AudioPromptAnswer),
		Prompts:             combinedPrompts, // Assign the combined list
	}

	// 4. Send the response using the DTO structure
	utils.RespondWithJSON(w, http.StatusOK, ProfileResponse{
		Success: true,
		User:    &responseUser,
	})
}
