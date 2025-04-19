package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type NullString struct {
	String string `json:"String"`
	Valid  bool   `json:"Valid"`
}

func NewNullString(pt pgtype.Text) *NullString {
	if !pt.Valid {
		return nil
	}
	return &NullString{String: pt.String, Valid: true}
}

type NullDate struct {
	Time  string `json:"Time"`
	Valid bool   `json:"Valid"`
}

func NewNullDate(pd pgtype.Date) *NullDate {
	if !pd.Valid || pd.Time.IsZero() {
		return nil
	}
	return &NullDate{Time: pd.Time.Format("2006-01-02"), Valid: true}
}

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

type NullGenderEnumJSON struct {
	GenderEnum migrations.GenderEnum `json:"GenderEnum"`
	Valid      bool                  `json:"Valid"`
}

func NewNullGenderEnumJSON(ng migrations.NullGenderEnum) *NullGenderEnumJSON {
	if !ng.Valid {
		return nil
	}
	return &NullGenderEnumJSON{GenderEnum: ng.GenderEnum, Valid: true}
}

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

type CombinedPrompt struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type ProfileResponseUser struct {
	ID                  int32                    `json:"id"`
	CreatedAt           pgtype.Timestamptz       `json:"created_at"`
	Name                *NullString              `json:"name,omitempty"`
	LastName            *NullString              `json:"last_name,omitempty"`
	Email               string                   `json:"email"`
	DateOfBirth         *NullDate                `json:"date_of_birth,omitempty"`
	Latitude            *NullFloat64             `json:"latitude,omitempty"`
	Longitude           *NullFloat64             `json:"longitude,omitempty"`
	Gender              *NullGenderEnumJSON      `json:"gender,omitempty"`
	DatingIntention     *NullDatingIntentionJSON `json:"dating_intention,omitempty"`
	Height              *NullString              `json:"height,omitempty"`
	Hometown            *NullString              `json:"hometown,omitempty"`
	JobTitle            *NullString              `json:"job_title,omitempty"`
	Education           *NullString              `json:"education,omitempty"`
	ReligiousBeliefs    *NullReligionJSON        `json:"religious_beliefs,omitempty"`
	DrinkingHabit       *NullHabitJSON           `json:"drinking_habit,omitempty"`
	SmokingHabit        *NullHabitJSON           `json:"smoking_habit,omitempty"`
	MediaUrls           []string                 `json:"media_urls"`
	VerificationStatus  string                   `json:"verification_status"`
	VerificationPic     *NullString              `json:"verification_pic,omitempty"`
	Role                string                   `json:"role"`
	AudioPromptQuestion *NullAudioPromptJSON     `json:"audio_prompt_question,omitempty"`
	AudioPromptAnswer   *NullString              `json:"audio_prompt_answer,omitempty"`
	Prompts             []CombinedPrompt         `json:"prompts"`
}

type ProfileResponse struct {
	Success bool                 `json:"success"`
	User    *ProfileResponseUser `json:"user,omitempty"`
	Message string               `json:"message,omitempty"`
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
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

	var combinedPrompts []CombinedPrompt
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

	responseUser := ProfileResponseUser{
		ID:                  user.ID,
		CreatedAt:           user.CreatedAt,
		Email:               user.Email,
		Name:                NewNullString(user.Name),
		LastName:            NewNullString(user.LastName),
		DateOfBirth:         NewNullDate(user.DateOfBirth),
		Latitude:            NewNullFloat64(user.Latitude),
		Longitude:           NewNullFloat64(user.Longitude),
		Gender:              NewNullGenderEnumJSON(user.Gender),
		DatingIntention:     NewNullDatingIntentionJSON(user.DatingIntention),
		Height:              NewNullString(FormatHeight(user.Height)),
		Hometown:            NewNullString(user.Hometown),
		JobTitle:            NewNullString(user.JobTitle),
		Education:           NewNullString(user.Education),
		ReligiousBeliefs:    NewNullReligionJSON(user.ReligiousBeliefs),
		DrinkingHabit:       NewNullHabitJSON(user.DrinkingHabit),
		SmokingHabit:        NewNullHabitJSON(user.SmokingHabit),
		MediaUrls:           user.MediaUrls,
		VerificationStatus:  string(user.VerificationStatus),
		VerificationPic:     NewNullString(user.VerificationPic),
		Role:                string(user.Role),
		AudioPromptQuestion: NewNullAudioPromptJSON(user.AudioPromptQuestion),
		AudioPromptAnswer:   NewNullString(user.AudioPromptAnswer),
		Prompts:             combinedPrompts,
	}

	utils.RespondWithJSON(w, http.StatusOK, ProfileResponse{
		Success: true,
		User:    &responseUser,
	})
}
