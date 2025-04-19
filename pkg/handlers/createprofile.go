// FILE: pkg/handlers/createprofile.go
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils" // Import utils
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Modified request: Removed location and gender
type createProfileRequest struct {
	Name        *string `json:"name"`
	LastName    *string `json:"last_name"`
	DateOfBirth *string `json:"date_of_birth"`
	// Latitude         *float64                          `json:"latitude"` // REMOVED
	// Longitude        *float64                          `json:"longitude"` // REMOVED
	// Gender           *string                           `json:"gender"` // REMOVED - Use *string if kept, but removed based on req.
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

// Function to respond with error (can be moved to utils if preferred)
func respondError(w http.ResponseWriter, msg string, code int) {
	log.Printf("[ERROR %d] %s", code, msg)
	utils.RespondWithError(w, code, msg) // Use shared error response
}

// CreateProfile handles updating the main profile details *excluding* location and gender.
func CreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fmt.Println("\n=== Starting Profile Creation/Update (Partial) ===")
	defer fmt.Println("=== End Profile Creation/Update (Partial) ===")

	queries, err := db.GetDB()
	if err != nil {
		respondError(w, "Database connection is not available", http.StatusInternalServerError)
		return
	}
	pool, er := db.GetPool()
	if er != nil {
		respondError(w, "Database connection pool is not available for transaction", http.StatusInternalServerError)
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		respondError(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}
	userID := int32(claims.UserID)
	fmt.Printf("[Auth] UserID from token: %d\n", userID)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Rewind body
	fmt.Printf("[Request] Raw body: %s\n", string(bodyBytes))

	var reqData createProfileRequest
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		fmt.Printf("[Decode Error] %v\n", err)
		respondError(w, fmt.Sprintf("Invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	// Check if user exists (still good practice)
	_, err = queries.GetUserByID(ctx, userID)
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

	// --- Parameter Preparation (Removed Gender and Location) ---
	var datingIntentionParam migrations.NullDatingIntention
	if reqData.DatingIntention != nil {
		datingIntentionParam = migrations.NullDatingIntention{DatingIntention: *reqData.DatingIntention, Valid: true}
	} else {
		datingIntentionParam = migrations.NullDatingIntention{Valid: false}
	}

	var religiousBeliefsParam migrations.NullReligion
	if reqData.ReligiousBeliefs != nil {
		religiousBeliefsParam = migrations.NullReligion{Religion: *reqData.ReligiousBeliefs, Valid: true}
	} else {
		religiousBeliefsParam = migrations.NullReligion{Valid: false}
	}

	var drinkingHabitParam migrations.NullDrinkingSmokingHabits
	if reqData.DrinkingHabit != nil {
		drinkingHabitParam = migrations.NullDrinkingSmokingHabits{DrinkingSmokingHabits: *reqData.DrinkingHabit, Valid: true}
	} else {
		drinkingHabitParam = migrations.NullDrinkingSmokingHabits{Valid: false}
	}

	var smokingHabitParam migrations.NullDrinkingSmokingHabits
	if reqData.SmokingHabit != nil {
		smokingHabitParam = migrations.NullDrinkingSmokingHabits{DrinkingSmokingHabits: *reqData.SmokingHabit, Valid: true}
	} else {
		smokingHabitParam = migrations.NullDrinkingSmokingHabits{Valid: false}
	}

	// Use the modified UpdateUserProfileParams structure (regenerated by sqlc)
	// Ensure the parameter order matches the generated migrations.UpdateUserProfileParams struct
	updateParams := migrations.UpdateUserProfileParams{
		ID:       userID,                                                                                            // This will be the last parameter ($12) in the SQL query now
		Name:     pgtype.Text{String: derefString(reqData.Name), Valid: reqData.Name != nil && *reqData.Name != ""}, // $1
		LastName: pgtype.Text{String: derefString(reqData.LastName), Valid: reqData.LastName != nil},                // $2
		// DateOfBirth is handled below ($3)
		// Latitude removed
		// Longitude removed
		// Gender removed
		DatingIntention: datingIntentionParam, // $4
		// Height is handled below ($5)
		Hometown:         pgtype.Text{String: derefString(reqData.Hometown), Valid: reqData.Hometown != nil},   // $6
		JobTitle:         pgtype.Text{String: derefString(reqData.JobTitle), Valid: reqData.JobTitle != nil},   // $7
		Education:        pgtype.Text{String: derefString(reqData.Education), Valid: reqData.Education != nil}, // $8
		ReligiousBeliefs: religiousBeliefsParam,                                                                // $9
		DrinkingHabit:    drinkingHabitParam,                                                                   // $10
		SmokingHabit:     smokingHabitParam,                                                                    // $11
	}

	// Date of Birth Handling (assigns to the correct field in the struct)
	if reqData.DateOfBirth != nil && *reqData.DateOfBirth != "" {
		dob, err := time.Parse("2006-01-02", *reqData.DateOfBirth)
		if err != nil {
			respondError(w, "Invalid date_of_birth format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
		updateParams.DateOfBirth = pgtype.Date{Time: dob, Valid: true} // $3
	} else {
		updateParams.DateOfBirth = pgtype.Date{Valid: false} // $3
	}

	// Height Handling (assigns to the correct field in the struct)
	if reqData.Height != nil && *reqData.Height != "" {
		heightInches, err := parseHeightString(*reqData.Height)
		if err != nil {
			respondError(w, fmt.Sprintf("Invalid height format: %v", err), http.StatusBadRequest)
			return
		}
		updateParams.Height = pgtype.Float8{Float64: heightInches, Valid: true} // $5
	} else {
		updateParams.Height = pgtype.Float8{Valid: false} // $5
	}

	// --- Validation (Modified) ---
	if err := validatePartialProfile(updateParams, reqData.Prompts); err != nil { // Use modified validation
		fmt.Printf("[Validation Failed] %s\n", err)
		respondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("[Validation] Input data passed validation.")

	fmt.Println("[Database] Starting transaction...")
	tx, err := pool.Begin(ctx)
	if err != nil {
		fmt.Printf("[Database Error] Failed to begin transaction: %v\n", err)
		respondError(w, "Database error starting transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)

	fmt.Println("[Database] Updating user profile (partial)...")
	_, err = qtx.UpdateUserProfile(ctx, updateParams) // Call the modified query
	if err != nil {
		fmt.Printf("[Database Error] Failed to update user profile: %v\n", err)
		respondError(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}
	fmt.Println("[Database] User profile updated.")

	// --- Prompt Handling within Transaction (unchanged logic) ---
	fmt.Println("[Database] Deleting existing prompts...")
	// Wrap delete operations in a helper or check errors individually
	if err := deletePrompts(ctx, qtx, userID); err != nil {
		respondError(w, "Failed to update prompts (delete step)", http.StatusInternalServerError)
		return // Exit on error
	}
	fmt.Println("[Database] Existing prompts deleted.")

	fmt.Printf("[Database] Creating %d new prompts...\n", len(reqData.Prompts))
	if err := createPrompts(ctx, qtx, userID, reqData.Prompts); err != nil {
		respondError(w, fmt.Sprintf("Failed to save prompts: %v", err), http.StatusInternalServerError)
		return // Exit on error
	}
	fmt.Println("[Database] New prompts created.")
	// --- End Prompt Handling ---

	if err := tx.Commit(ctx); err != nil {
		fmt.Printf("[Database Error] Failed to commit transaction: %v\n", err)
		respondError(w, "Database error saving profile", http.StatusInternalServerError)
		return
	}
	fmt.Println("[Database] Transaction committed successfully.")

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{ // Use utils
		"success": true,
		"message": "Profile updated successfully",
	})
	fmt.Println("[Success] Profile update complete.")
}

// Helper to delete all prompt types for a user within a transaction
func deletePrompts(ctx context.Context, qtx *migrations.Queries, userID int32) error {
	if err := qtx.DeleteUserDateVibesPrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[Database Error] Failed to delete Date Vibes prompts for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to delete Date Vibes prompts: %w", err)
	}
	if err := qtx.DeleteUserGettingPersonalPrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[Database Error] Failed to delete Getting Personal prompts for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to delete Getting Personal prompts: %w", err)
	}
	if err := qtx.DeleteUserMyTypePrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[Database Error] Failed to delete My Type prompts for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to delete My Type prompts: %w", err)
	}
	if err := qtx.DeleteUserStoryTimePrompts(ctx, userID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[Database Error] Failed to delete Story Time prompts for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to delete Story Time prompts: %w", err)
	}
	return nil
}

// Helper to create new prompts within a transaction
func createPrompts(ctx context.Context, qtx *migrations.Queries, userID int32, prompts []promptRequest) error {
	for i, p := range prompts {
		fmt.Printf("[Database] Processing prompt %d: Category=%s, Question=%s\n", i+1, p.Category, p.Question)
		var promptErr error
		switch p.Category {
		case "dateVibes":
			promptEnum, err := parseDateVibesEnum(p.Question)
			if err != nil {
				promptErr = fmt.Errorf("invalid dateVibes question '%s'", p.Question)
				break
			}
			_, promptErr = qtx.CreateDateVibesPrompt(ctx, migrations.CreateDateVibesPromptParams{UserID: userID, Question: promptEnum, Answer: p.Answer})
		case "gettingPersonal":
			promptEnum, err := parseGettingPersonalEnum(p.Question)
			if err != nil {
				promptErr = fmt.Errorf("invalid gettingPersonal question '%s'", p.Question)
				break
			}
			_, promptErr = qtx.CreateGettingPersonalPrompt(ctx, migrations.CreateGettingPersonalPromptParams{UserID: userID, Question: promptEnum, Answer: p.Answer})
		case "myType":
			promptEnum, err := parseMyTypeEnum(p.Question)
			if err != nil {
				promptErr = fmt.Errorf("invalid myType question '%s'", p.Question)
				break
			}
			_, promptErr = qtx.CreateMyTypePrompt(ctx, migrations.CreateMyTypePromptParams{UserID: userID, Question: promptEnum, Answer: p.Answer})
		case "storyTime":
			promptEnum, err := parseStoryTimeEnum(p.Question)
			if err != nil {
				promptErr = fmt.Errorf("invalid storyTime question '%s'", p.Question)
				break
			}
			_, promptErr = qtx.CreateStoryTimePrompt(ctx, migrations.CreateStoryTimePromptParams{UserID: userID, Question: promptEnum, Answer: p.Answer})
		default:
			promptErr = fmt.Errorf("unknown prompt category '%s'", p.Category)
		}

		if promptErr != nil {
			log.Printf("[Database Error] Failed to create prompt (Cat: %s, Q: %s): %v\n", p.Category, p.Question, promptErr)
			// The transaction will be rolled back by the defer in the main handler
			return fmt.Errorf("failed to create prompt (Cat: %s, Q: %s): %w", p.Category, p.Question, promptErr)
		}
	}
	return nil
}

// --- Helper functions (parse enums, deref*, parseHeightString - keep these unchanged) ---

func parseDateVibesEnum(s string) (migrations.DateVibesPromptType, error) {
	val := migrations.DateVibesPromptType(s)
	switch val {
	case migrations.DateVibesPromptTypeTogetherWeCould,
		migrations.DateVibesPromptTypeFirstRoundIsOnMeIf,
		migrations.DateVibesPromptTypeWhatIOrderForTheTable,
		migrations.DateVibesPromptTypeBestSpotInTown,
		migrations.DateVibesPromptTypeBestWayToAskMeOut:
		return val, nil
	default:
		return "", fmt.Errorf("invalid DateVibesPromptType: %s", s)
	}
}

func parseGettingPersonalEnum(s string) (migrations.GettingPersonalPromptType, error) {
	val := migrations.GettingPersonalPromptType(s)
	switch val {
	case migrations.GettingPersonalPromptTypeOneThingYouShouldKnow,
		migrations.GettingPersonalPromptTypeLoveLanguage,
		migrations.GettingPersonalPromptTypeDorkiestThing,
		migrations.GettingPersonalPromptTypeDontHateMeIf,
		migrations.GettingPersonalPromptTypeGeekOutOn,
		migrations.GettingPersonalPromptTypeIfLovingThisIsWrong,
		migrations.GettingPersonalPromptTypeKeyToMyHeart,
		migrations.GettingPersonalPromptTypeWontShutUpAbout,
		migrations.GettingPersonalPromptTypeShouldNotGoOutWithMeIf,
		migrations.GettingPersonalPromptTypeWhatIfIToldYouThat:
		return val, nil
	default:
		return "", fmt.Errorf("invalid GettingPersonalPromptType: %s", s)
	}
}

func parseMyTypeEnum(s string) (migrations.MyTypePromptType, error) {
	val := migrations.MyTypePromptType(s)
	switch val {
	case migrations.MyTypePromptTypeNonNegotiable,
		migrations.MyTypePromptTypeHallmarkOfGoodRelationship,
		migrations.MyTypePromptTypeLookingFor,
		migrations.MyTypePromptTypeWeirdlyAttractedTo,
		migrations.MyTypePromptTypeAllIAskIsThatYou,
		migrations.MyTypePromptTypeWellGetAlongIf,
		migrations.MyTypePromptTypeWantSomeoneWho,
		migrations.MyTypePromptTypeGreenFlags,
		migrations.MyTypePromptTypeSameTypeOfWeird,
		migrations.MyTypePromptTypeFallForYouIf,
		migrations.MyTypePromptTypeBragAboutYou:
		return val, nil
	default:
		return "", fmt.Errorf("invalid MyTypePromptType: %s", s)
	}
}

func parseStoryTimeEnum(s string) (migrations.StoryTimePromptType, error) {
	val := migrations.StoryTimePromptType(s)
	switch val {
	case migrations.StoryTimePromptTypeTwoTruthsAndALie,
		migrations.StoryTimePromptTypeWorstIdea,
		migrations.StoryTimePromptTypeBiggestRisk,
		migrations.StoryTimePromptTypeBiggestDateFail,
		migrations.StoryTimePromptTypeNeverHaveIEver,
		migrations.StoryTimePromptTypeBestTravelStory,
		migrations.StoryTimePromptTypeWeirdestGift,
		migrations.StoryTimePromptTypeMostSpontaneous,
		migrations.StoryTimePromptTypeOneThingNeverDoAgain:
		return val, nil
	default:
		return "", fmt.Errorf("invalid StoryTimePromptType: %s", s)
	}
}

func derefString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func derefFloat64(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0.0
}

var heightRegex = regexp.MustCompile(`^[4-6]'([0-9]|1[0-1])"$`)

func parseHeightString(heightStr string) (float64, error) {
	if !heightRegex.MatchString(heightStr) {
		return 0, fmt.Errorf("invalid format. Use F'I\" (e.g., 5'10\")")
	}
	parts := strings.Split(strings.TrimSpace(strings.TrimSuffix(heightStr, "\"")), "'")
	if len(parts) != 2 {
		// Attempt to fix common issue like "5' 10\""
		parts = strings.FieldsFunc(strings.TrimSuffix(heightStr, "\""), func(r rune) bool {
			return r == '\'' || r == ' '
		})
		if len(parts) != 2 {
			return 0, fmt.Errorf("internal parsing error, expected format F'I\"")
		}
	}

	feetStr := strings.TrimSpace(parts[0])
	inchesStr := strings.TrimSpace(parts[1])

	feet, err := strconv.Atoi(feetStr)
	if err != nil {
		return 0, fmt.Errorf("invalid feet value '%s': %w", feetStr, err)
	}
	inches, err := strconv.Atoi(inchesStr)
	if err != nil {
		return 0, fmt.Errorf("invalid inches value '%s': %w", inchesStr, err)
	}

	if feet < 4 || feet > 6 {
		return 0, fmt.Errorf("feet value must be between 4 and 6")
	}
	if inches < 0 || inches > 11 {
		return 0, fmt.Errorf("inches value must be between 0 and 11")
	}

	totalInches := float64(feet*12 + inches)
	return totalInches, nil
}

// --- Modified Validation Logic ---
// Validates the fields handled by *this* endpoint (/api/profile)
func validatePartialProfile(params migrations.UpdateUserProfileParams, prompts []promptRequest) error {
	fmt.Println("\n=== Starting Partial Profile Validation ===")
	defer fmt.Println("=== End Partial Profile Validation ===\n")

	// Basic Info (Name, DOB required for this stage)
	if !params.Name.Valid || len(params.Name.String) == 0 {
		return fmt.Errorf("name is required")
	}
	if len(params.Name.String) > 20 {
		return fmt.Errorf("name must not exceed 20 characters")
	}
	fmt.Printf("[Validation] Name: OK ('%s')\n", params.Name.String)

	if params.LastName.Valid && len(params.LastName.String) > 20 {
		return fmt.Errorf("last name must not exceed 20 characters")
	}
	fmt.Printf("[Validation] LastName: OK (Valid: %t, Value: '%s')\n", params.LastName.Valid, params.LastName.String)

	if !params.DateOfBirth.Valid || params.DateOfBirth.Time.IsZero() {
		return fmt.Errorf("date of birth is required and must be valid")
	}
	age := time.Since(params.DateOfBirth.Time).Hours() / 24 / 365.25
	fmt.Printf("[Validation] DOB: OK (%s), Calculated Age: %.2f\n", params.DateOfBirth.Time.Format("2006-01-02"), age)
	if age < 18 {
		return fmt.Errorf("must be at least 18 years old")
	}

	// REMOVED Location validation
	// REMOVED Gender validation

	// Profile Details (Height, Habits, etc. - kept)
	if !params.Height.Valid {
		return fmt.Errorf("height is required")
	}
	fmt.Printf("[Validation] Height: OK (Value: %.2f inches)\n", params.Height.Float64)

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

	// Optional Fields Length Checks (unchanged, limits previously updated)
	if params.Hometown.Valid && len(params.Hometown.String) > 30 {
		return fmt.Errorf("hometown must not exceed 30 characters")
	}
	fmt.Printf("[Validation] Hometown: OK (Valid: %t, Value: '%s', Limit: 30)\n", params.Hometown.Valid, params.Hometown.String)

	if params.JobTitle.Valid && len(params.JobTitle.String) > 30 {
		return fmt.Errorf("job title must not exceed 30 characters")
	}
	fmt.Printf("[Validation] JobTitle: OK (Valid: %t, Value: '%s', Limit: 30)\n", params.JobTitle.Valid, params.JobTitle.String)

	if params.Education.Valid && len(params.Education.String) > 30 {
		return fmt.Errorf("education must not exceed 30 characters")
	}
	fmt.Printf("[Validation] Education: OK (Valid: %t, Value: '%s', Limit: 30)\n", params.Education.Valid, params.Education.String)

	// Prompt Validation (unchanged)
	fmt.Printf("[Validation] Checking prompts. Count: %d\n", len(prompts))
	if len(prompts) == 0 {
		return fmt.Errorf("at least one prompt is required")
	}
	if len(prompts) > 3 {
		return fmt.Errorf("maximum of 3 prompts allowed")
	}

	promptQuestions := make(map[string]bool)
	for i, p := range prompts {
		// ... (prompt validation logic remains exactly the same) ...
		fmt.Printf("[Validation] Checking prompt %d: Category=%s, Question=%s\n", i+1, p.Category, p.Question)
		if p.Category == "" {
			return fmt.Errorf("prompt %d: category is required", i+1)
		}
		if p.Question == "" {
			return fmt.Errorf("prompt %d: question is required", i+1)
		}
		if strings.TrimSpace(p.Answer) == "" {
			return fmt.Errorf("prompt %d: answer cannot be empty or just whitespace", i+1)
		}
		if len(p.Answer) > 255 {
			return fmt.Errorf("prompt %d: answer exceeds maximum length (255 chars)", i+1)
		}

		var parseErr error
		switch p.Category {
		case "dateVibes":
			_, parseErr = parseDateVibesEnum(p.Question)
		case "gettingPersonal":
			_, parseErr = parseGettingPersonalEnum(p.Question)
		case "myType":
			_, parseErr = parseMyTypeEnum(p.Question)
		case "storyTime":
			_, parseErr = parseStoryTimeEnum(p.Question)
		default:
			return fmt.Errorf("prompt %d: unknown category '%s'", i+1, p.Category)
		}
		if parseErr != nil {
			return fmt.Errorf("prompt %d: invalid question '%s' for category '%s'", i+1, p.Question, p.Category)
		}

		questionKey := fmt.Sprintf("%s:%s", p.Category, p.Question)
		if promptQuestions[questionKey] {
			return fmt.Errorf("prompt question '%s' under category '%s' cannot be used more than once", p.Question, p.Category)
		}
		promptQuestions[questionKey] = true
		fmt.Printf("[Validation] Prompt %d: OK\n", i+1)
	}

	fmt.Println("[Validation] All partial profile checks passed successfully")
	return nil
}
