package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func respondError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"message": msg,
	})
}

type createProfileRequest struct {
	Name             *string                           `json:"name"`
	LastName         *string                           `json:"last_name"`
	DateOfBirth      *string                           `json:"date_of_birth"`
	Latitude         *float64                          `json:"latitude"`
	Longitude        *float64                          `json:"longitude"`
	Gender           *migrations.GenderEnum            `json:"gender"`
	DatingIntention  *migrations.DatingIntention       `json:"dating_intention"`
	Height           *string                           `json:"height"`
	Hometown         *string                           `json:"hometown"`
	JobTitle         *string                           `json:"job_title"`
	Education        *string                           `json:"education"`
	ReligiousBeliefs *migrations.Religion              `json:"religious_beliefs"`
	DrinkingHabit    *migrations.DrinkingSmokingHabits `json:"drinking_habit"`
	SmokingHabit     *migrations.DrinkingSmokingHabits `json:"smoking_habit"`
	Prompts          []promptRequest                   `json:"prompts"`
}

type promptRequest struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Println("\n=== Starting Profile Creation (sqlc) ===")
	defer fmt.Println("=== End Profile Creation (sqlc) ===")

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		respondError(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	userID := int32(claims.UserID)
	fmt.Printf("[Auth] UserID from token: %d\n", userID)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	fmt.Printf("[Request] Raw body: %s\n", string(bodyBytes))

	var reqData createProfileRequest
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		fmt.Printf("[Decode Error] %v\n", err)
		respondError(w, fmt.Sprintf("Invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	q := migrations.New(dbPool)

	_, err = q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Printf("[Error] User lookup failed: User ID %d not found\n", userID)
			respondError(w, "User not found", http.StatusNotFound)
		} else {
			fmt.Printf("[Error] User lookup failed: %v\n", err)
			respondError(w, "Database error checking user", http.StatusInternalServerError)
		}
		return
	}
	fmt.Printf("[Existing User] Found User ID: %d\n", userID)

	updateParams := migrations.UpdateUserProfileParams{
		ID:               userID,
		Name:             pgtype.Text{String: *reqData.Name, Valid: reqData.Name != nil && *reqData.Name != ""},
		LastName:         pgtype.Text{String: *reqData.LastName, Valid: reqData.LastName != nil},
		Latitude:         pgtype.Float8{Float64: *reqData.Latitude, Valid: reqData.Latitude != nil},
		Longitude:        pgtype.Float8{Float64: *reqData.Longitude, Valid: reqData.Longitude != nil},
		Gender:           *reqData.Gender,
		DatingIntention:  migrations.NullDatingIntention{DatingIntention: *reqData.DatingIntention, Valid: reqData.DatingIntention != nil},
		Hometown:         pgtype.Text{String: *reqData.Hometown, Valid: reqData.Hometown != nil},
		JobTitle:         pgtype.Text{String: *reqData.JobTitle, Valid: reqData.JobTitle != nil},
		Education:        pgtype.Text{String: *reqData.Education, Valid: reqData.Education != nil},
		ReligiousBeliefs: migrations.NullReligion{Religion: *reqData.ReligiousBeliefs, Valid: reqData.ReligiousBeliefs != nil},
		DrinkingHabit:    migrations.NullDrinkingSmokingHabits{DrinkingSmokingHabits: *reqData.DrinkingHabit, Valid: reqData.DrinkingHabit != nil},
		SmokingHabit:     migrations.NullDrinkingSmokingHabits{DrinkingSmokingHabits: *reqData.SmokingHabit, Valid: reqData.SmokingHabit != nil},
	}

	if reqData.DateOfBirth != nil && *reqData.DateOfBirth != "" {
		dob, err := time.Parse("2006-01-02", *reqData.DateOfBirth)
		if err != nil {
			respondError(w, "Invalid date_of_birth format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
		updateParams.DateOfBirth = pgtype.Date{Time: dob, Valid: true}
	} else {
		updateParams.DateOfBirth = pgtype.Date{Valid: false}
	}

	if reqData.Height != nil && *reqData.Height != "" {
		heightInches, err := parseHeightString(*reqData.Height)
		if err != nil {
			respondError(w, fmt.Sprintf("Invalid height format: %v", err), http.StatusBadRequest)
			return
		}
		updateParams.Height = pgtype.Float8{Float64: heightInches, Valid: true}
	} else {
		updateParams.Height = pgtype.Float8{Valid: false}
	}

	if err := validateProfileSqlc(updateParams, reqData.Prompts); err != nil {
		fmt.Printf("[Validation Failed] %s\n", err)
		respondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("[Validation] Input data passed validation.")

	fmt.Println("[Database] Starting transaction...")
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		fmt.Printf("[Database Error] Failed to begin transaction: %v\n", err)
		respondError(w, "Database error starting transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := q.WithTx(tx)

	fmt.Println("[Database] Updating user profile...")
	_, err = qtx.UpdateUserProfile(ctx, updateParams)
	if err != nil {
		fmt.Printf("[Database Error] Failed to update user profile: %v\n", err)
		respondError(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}
	fmt.Println("[Database] User profile updated.")

	fmt.Println("[Database] Deleting existing prompts...")
	if err := qtx.DeleteUserDateVibesPrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Printf("[Database Error] Failed to delete Date Vibes prompts: %v\n", err)
		respondError(w, "Failed to update prompts (delete DV)", http.StatusInternalServerError)
		return
	}
	if err := qtx.DeleteUserGettingPersonalPrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Printf("[Database Error] Failed to delete Getting Personal prompts: %v\n", err)
		respondError(w, "Failed to update prompts (delete GP)", http.StatusInternalServerError)
		return
	}
	if err := qtx.DeleteUserMyTypePrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Printf("[Database Error] Failed to delete My Type prompts: %v\n", err)
		respondError(w, "Failed to update prompts (delete MT)", http.StatusInternalServerError)
		return
	}
	if err := qtx.DeleteUserStoryTimePrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fmt.Printf("[Database Error] Failed to delete Story Time prompts: %v\n", err)
		respondError(w, "Failed to update prompts (delete ST)", http.StatusInternalServerError)
		return
	}
	fmt.Println("[Database] Existing prompts deleted.")

	fmt.Printf("[Database] Creating %d new prompts...\n", len(reqData.Prompts))
	for i, p := range reqData.Prompts {
		fmt.Printf("[Database] Processing prompt %d: Category=%s, Question=%s\n", i+1, p.Category, p.Question)
		switch p.Category {
		case "dateVibes":
			promptEnum := migrations.DateVibesPromptType(p.Question)
			_, err = qtx.CreateDateVibesPrompt(ctx, migrations.CreateDateVibesPromptParams{
				UserID:   userID,
				Question: promptEnum,
				Answer:   p.Answer,
			})
		case "gettingPersonal":
			promptEnum := migrations.GettingPersonalPromptType(p.Question)
			_, err = qtx.CreateGettingPersonalPrompt(ctx, migrations.CreateGettingPersonalPromptParams{
				UserID:   userID,
				Question: promptEnum,
				Answer:   p.Answer,
			})
		case "myType":
			promptEnum := migrations.MyTypePromptType(p.Question)
			_, err = qtx.CreateMyTypePrompt(ctx, migrations.CreateMyTypePromptParams{
				UserID:   userID,
				Question: promptEnum,
				Answer:   p.Answer,
			})
		case "storyTime":
			promptEnum := migrations.StoryTimePromptType(p.Question)
			_, err = qtx.CreateStoryTimePrompt(ctx, migrations.CreateStoryTimePromptParams{
				UserID:   userID,
				Question: promptEnum,
				Answer:   p.Answer,
			})
		default:
			fmt.Printf("[Error] Unknown prompt category '%s' during creation\n", p.Category)
			respondError(w, fmt.Sprintf("Internal error: Unknown prompt category %s", p.Category), http.StatusInternalServerError)
			return
		}

		if err != nil {
			fmt.Printf("[Database Error] Failed to create prompt (Cat: %s, Q: %s): %v\n", p.Category, p.Question, err)
			respondError(w, "Failed to save prompts", http.StatusInternalServerError)
			return
		}
	}
	fmt.Println("[Database] New prompts created.")

	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("[Database Error] Failed to commit transaction: %v\n", err)
		respondError(w, "Database error saving profile", http.StatusInternalServerError)
		return
	}
	fmt.Println("[Database] Transaction committed successfully.")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Profile updated successfully",
	})
	fmt.Println("[Success] Profile update complete.")
}

var heightRegex = regexp.MustCompile(`^[4-6]'([0-9]|1[0-1])"$`)

func validateProfileSqlc(params migrations.UpdateUserProfileParams, prompts []promptRequest) error {
	fmt.Println("\n=== Starting Profile Validation (sqlc) ===")
	defer fmt.Println("=== End Profile Validation (sqlc) ===\n")

	if !params.Name.Valid || len(params.Name.String) == 0 || len(params.Name.String) > 20 {
		return fmt.Errorf("name must be present and between 1 and 20 characters")
	}
	fmt.Printf("[Validation] Name: OK ('%s')\n", params.Name.String)

	if params.LastName.Valid && len(params.LastName.String) > 20 {
		return fmt.Errorf("last name must not exceed 20 characters")
	}
	fmt.Printf("[Validation] LastName: OK (Valid: %t, Value: '%s')\n", params.LastName.Valid, params.LastName.String)

	if !params.DateOfBirth.Valid {
		return fmt.Errorf("date of birth is required")
	}
	age := time.Since(params.DateOfBirth.Time).Hours() / 24 / 365
	fmt.Printf("[Validation] DOB: OK (%s), Calculated Age: %.2f\n", params.DateOfBirth.Time.Format("2006-01-02"), age)
	if age < 18 {
		return fmt.Errorf("must be at least 18 years old")
	}

	if !params.Latitude.Valid || !params.Longitude.Valid {
		return fmt.Errorf("latitude and longitude are required")
	}
	fmt.Printf("[Validation] Location: OK (Lat: %.6f, Lon: %.6f)\n", params.Latitude.Float64, params.Longitude.Float64)

	if !params.Height.Valid {
		return fmt.Errorf("height is required")
	}
	fmt.Printf("[Validation] Height: OK (Value: %.2f inches)\n", params.Height.Float64)

	if params.Gender == "" {
		return fmt.Errorf("gender is required")
	}
	fmt.Printf("[Validation] Gender: OK (%s)\n", params.Gender)

	if !params.DatingIntention.Valid {
		return fmt.Errorf("dating intention is required")
	}
	fmt.Printf("[Validation] Dating Intention: OK (%s)\n", params.DatingIntention.DatingIntention)

	if !params.ReligiousBeliefs.Valid {
		return fmt.Errorf("religious beliefs is required")
	}
	fmt.Printf("[Validation] Religious Beliefs: OK (%s)\n", params.ReligiousBeliefs.Religion)

	if !params.DrinkingHabit.Valid {
		return fmt.Errorf("drinking habit is required")
	}
	fmt.Printf("[Validation] Drinking Habit: OK (%s)\n", params.DrinkingHabit.DrinkingSmokingHabits)

	if !params.SmokingHabit.Valid {
		return fmt.Errorf("smoking habit is required")
	}
	fmt.Printf("[Validation] Smoking Habit: OK (%s)\n", params.SmokingHabit.DrinkingSmokingHabits)

	if params.Hometown.Valid && len(params.Hometown.String) > 15 {
		return fmt.Errorf("hometown must not exceed 15 characters")
	}
	fmt.Printf("[Validation] Hometown: OK (Valid: %t, Value: '%s')\n", params.Hometown.Valid, params.Hometown.String)

	if params.JobTitle.Valid && len(params.JobTitle.String) > 15 {
		return fmt.Errorf("job title must not exceed 15 characters")
	}
	fmt.Printf("[Validation] JobTitle: OK (Valid: %t, Value: '%s')\n", params.JobTitle.Valid, params.JobTitle.String)

	if params.Education.Valid && len(params.Education.String) > 15 {
		return fmt.Errorf("education must not exceed 15 characters")
	}
	fmt.Printf("[Validation] Education: OK (Valid: %t, Value: '%s')\n", params.Education.Valid, params.Education.String)

	fmt.Printf("[Validation] Checking prompts. Count: %d\n", len(prompts))
	if len(prompts) == 0 {
		return fmt.Errorf("at least one prompt is required")
	}
	if len(prompts) > 3 {
		return fmt.Errorf("maximum of 3 prompts allowed")
	}

	promptQuestions := make(map[string]bool)
	for i, p := range prompts {
		fmt.Printf("[Validation] Checking prompt %d: Category=%s, Question=%s\n", i+1, p.Category, p.Question)
		if p.Category == "" {
			return fmt.Errorf("prompt %d: category is required", i+1)
		}
		if p.Question == "" {
			return fmt.Errorf("prompt %d: question is required", i+1)
		}
		if strings.TrimSpace(p.Answer) == "" {
			return fmt.Errorf("prompt %d: answer cannot be empty", i+1)
		}
		if len(p.Answer) > 255 {
			return fmt.Errorf("prompt %d: answer exceeds maximum length (255 chars)", i+1)
		}

		isValidPrompt := false
		switch p.Category {
		case "dateVibes":
			_, isValidPrompt = map[migrations.DateVibesPromptType]bool{
				migrations.DateVibesPromptTypeTogetherWeCould:       true,
				migrations.DateVibesPromptTypeFirstRoundIsOnMeIf:    true,
				migrations.DateVibesPromptTypeWhatIOrderForTheTable: true,
				migrations.DateVibesPromptTypeBestSpotInTown:        true,
				migrations.DateVibesPromptTypeBestWayToAskMeOut:     true,
			}[migrations.DateVibesPromptType(p.Question)]
		case "gettingPersonal":
			_, isValidPrompt = map[migrations.GettingPersonalPromptType]bool{
				migrations.GettingPersonalPromptTypeOneThingYouShouldKnow:  true,
				migrations.GettingPersonalPromptTypeLoveLanguage:           true,
				migrations.GettingPersonalPromptTypeDorkiestThing:          true,
				migrations.GettingPersonalPromptTypeDontHateMeIf:           true,
				migrations.GettingPersonalPromptTypeGeekOutOn:              true,
				migrations.GettingPersonalPromptTypeIfLovingThisIsWrong:    true,
				migrations.GettingPersonalPromptTypeKeyToMyHeart:           true,
				migrations.GettingPersonalPromptTypeWontShutUpAbout:        true,
				migrations.GettingPersonalPromptTypeShouldNotGoOutWithMeIf: true,
				migrations.GettingPersonalPromptTypeWhatIfIToldYouThat:     true,
			}[migrations.GettingPersonalPromptType(p.Question)]
		case "myType":
			_, isValidPrompt = map[migrations.MyTypePromptType]bool{
				migrations.MyTypePromptTypeNonNegotiable:              true,
				migrations.MyTypePromptTypeHallmarkOfGoodRelationship: true,
				migrations.MyTypePromptTypeLookingFor:                 true,
				migrations.MyTypePromptTypeWeirdlyAttractedTo:         true,
				migrations.MyTypePromptTypeAllIAskIsThatYou:           true,
				migrations.MyTypePromptTypeWellGetAlongIf:             true,
				migrations.MyTypePromptTypeWantSomeoneWho:             true,
				migrations.MyTypePromptTypeGreenFlags:                 true,
				migrations.MyTypePromptTypeSameTypeOfWeird:            true,
				migrations.MyTypePromptTypeFallForYouIf:               true,
				migrations.MyTypePromptTypeBragAboutYou:               true,
			}[migrations.MyTypePromptType(p.Question)]
		case "storyTime":
			_, isValidPrompt = map[migrations.StoryTimePromptType]bool{
				migrations.StoryTimePromptTypeTwoTruthsAndALie:     true,
				migrations.StoryTimePromptTypeWorstIdea:            true,
				migrations.StoryTimePromptTypeBiggestRisk:          true,
				migrations.StoryTimePromptTypeBiggestDateFail:      true,
				migrations.StoryTimePromptTypeNeverHaveIEver:       true,
				migrations.StoryTimePromptTypeBestTravelStory:      true,
				migrations.StoryTimePromptTypeWeirdestGift:         true,
				migrations.StoryTimePromptTypeMostSpontaneous:      true,
				migrations.StoryTimePromptTypeOneThingNeverDoAgain: true,
			}[migrations.StoryTimePromptType(p.Question)]
		default:
			return fmt.Errorf("prompt %d: unknown category '%s'", i+1, p.Category)
		}

		if !isValidPrompt {
			return fmt.Errorf("prompt %d: question '%s' is not a valid question for category '%s'", i+1, p.Question, p.Category)
		}

		if promptQuestions[p.Question] {
			return fmt.Errorf("prompt question '%s' cannot be used more than once", p.Question)
		}
		promptQuestions[p.Question] = true

		fmt.Printf("[Validation] Prompt %d: OK\n", i+1)
	}

	fmt.Println("[Validation] All checks passed successfully")
	return nil
}

func parseHeightString(heightStr string) (float64, error) {
	if !heightRegex.MatchString(heightStr) {
		return 0, fmt.Errorf("invalid format. Use F'I\" (e.g., 5'10\")")
	}
	parts := strings.Split(strings.TrimSuffix(heightStr, "\""), "'")
	if len(parts) != 2 {
		return 0, fmt.Errorf("internal parsing error")
	}

	feet, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid feet value: %w", err)
	}
	inches, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid inches value: %w", err)
	}

	if feet < 4 || feet > 6 || inches < 0 || inches > 11 {
		return 0, fmt.Errorf("values out of range (4'0\" to 6'11\")")
	}

	totalInches := float64(feet*12 + inches)
	return totalInches, nil
}
