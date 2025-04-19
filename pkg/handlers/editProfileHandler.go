package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AudioPromptRequest struct {
	Question  *string `json:"question"`
	AnswerUrl *string `json:"answer_url"`
}

type EditProfileRequest struct {
	Name             *string  `json:"name"`
	LastName         *string  `json:"last_name"`
	DateOfBirth      *string  `json:"date_of_birth"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	Gender           *string  `json:"gender"`
	DatingIntention  *string  `json:"dating_intention"`
	Height           *string  `json:"height"`
	Hometown         *string  `json:"hometown"`
	JobTitle         *string  `json:"job_title"`
	Education        *string  `json:"education"`
	ReligiousBeliefs *string  `json:"religious_beliefs"`
	DrinkingHabit    *string  `json:"drinking_habit"`
	SmokingHabit     *string  `json:"smoking_habit"`

	Prompts     *[]promptRequest    `json:"prompts,omitempty"`
	AudioPrompt *AudioPromptRequest `json:"audio_prompt,omitempty"`
	MediaUrls   *[]string           `json:"media_urls,omitempty"`
}

type EditProfileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var heightEditRegex = regexp.MustCompile(`^[4-6]'([0-9]|1[0-1])"$`)

var audioPromptMapEdit map[string]migrations.AudioPrompt

func init() {
	audioPromptMapEdit = map[string]migrations.AudioPrompt{
		"canWeTalkAbout":                  migrations.AudioPromptCanWeTalkAbout,
		"captionThisPhoto":                migrations.AudioPromptCaptionThisPhoto,
		"caughtInTheAct":                  migrations.AudioPromptCaughtInTheAct,
		"changeMyMindAbout":               migrations.AudioPromptChangeMyMindAbout,
		"chooseOurFirstDate":              migrations.AudioPromptChooseOurFirstDate,
		"commentIfYouveBeenHere":          migrations.AudioPromptCommentIfYouveBeenHere,
		"cookWithMe":                      migrations.AudioPromptCookWithMe,
		"datingMeIsLike":                  migrations.AudioPromptDatingMeIsLike,
		"datingMeWillLookLike":            migrations.AudioPromptDatingMeWillLookLike,
		"doYouAgreeOrDisagreeThat":        migrations.AudioPromptDoYouAgreeOrDisagreeThat,
		"dontHateMeIfI":                   migrations.AudioPromptDontHateMeIfI,
		"dontJudgeMe":                     migrations.AudioPromptDontJudgeMe,
		"mondaysAmIRight":                 migrations.AudioPromptMondaysAmIRight,
		"aBoundaryOfMineIs":               migrations.AudioPromptABoundaryOfMineIs,
		"aDailyEssential":                 migrations.AudioPromptADailyEssential,
		"aDreamHomeMustInclude":           migrations.AudioPromptADreamHomeMustInclude,
		"aFavouriteMemoryOfMine":          migrations.AudioPromptAFavouriteMemoryOfMine,
		"aFriendsReviewOfMe":              migrations.AudioPromptAFriendsReviewOfMe,
		"aLifeGoalOfMine":                 migrations.AudioPromptALifeGoalOfMine,
		"aQuickRantAbout":                 migrations.AudioPromptAQuickRantAbout,
		"aRandomFactILoveIs":              migrations.AudioPromptARandomFactILoveIs,
		"aSpecialTalentOfMine":            migrations.AudioPromptASpecialTalentOfMine,
		"aThoughtIRecentlyHadInTheShower": migrations.AudioPromptAThoughtIRecentlyHadInTheShower,
		"allIAskIsThatYou":                migrations.AudioPromptAllIAskIsThatYou,
		"guessWhereThisPhotoWasTaken":     migrations.AudioPromptGuessWhereThisPhotoWasTaken,
		"helpMeIdentifyThisPhotoBomber":   migrations.AudioPromptHelpMeIdentifyThisPhotoBomber,
		"hiFromMeAndMyPet":                migrations.AudioPromptHiFromMeAndMyPet,
		"howIFightTheSundayScaries":       migrations.AudioPromptHowIFightTheSundayScaries,
		"howHistoryWillRememberMe":        migrations.AudioPromptHowHistoryWillRememberMe,
		"howMyFriendsSeeMe":               migrations.AudioPromptHowMyFriendsSeeMe,
		"howToPronounceMyName":            migrations.AudioPromptHowToPronounceMyName,
		"iBeatMyBluesBy":                  migrations.AudioPromptIBeatMyBluesBy,
		"iBetYouCant":                     migrations.AudioPromptIBetYouCant,
		"iCanTeachYouHowTo":               migrations.AudioPromptICanTeachYouHowTo,
		"iFeelFamousWhen":                 migrations.AudioPromptIFeelFamousWhen,
		"iFeelMostSupportedWhen":          migrations.AudioPromptIFeelMostSupportedWhen,
	}
}

func EditProfileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries, _ := db.GetDB()
	pool, _ := db.GetPool()

	if queries == nil || pool == nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection not available")
		return
	}

	if r.Method != http.MethodPatch {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use PATCH")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}
	userID := int32(claims.UserID)
	log.Printf("[EditProfile] Started for UserID: %d", userID)

	var reqData EditProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		log.Printf("[EditProfile] Error decoding request for user %d: %v", userID, err)
		utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}
	defer r.Body.Close()

	currentUser, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[EditProfile] User not found: %d", userID)
			utils.RespondWithError(w, http.StatusNotFound, "User not found")
		} else {
			log.Printf("[EditProfile] Error fetching user %d: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching user data")
		}
		return
	}
	log.Printf("[EditProfile] Fetched current data for UserID: %d", userID)

	updateProfileParams := migrations.UpdateUserProfileParams{ /* ... as before ... */
		ID:               userID,
		Name:             currentUser.Name,
		LastName:         currentUser.LastName,
		DateOfBirth:      currentUser.DateOfBirth,
		DatingIntention:  currentUser.DatingIntention,
		Height:           currentUser.Height,
		Hometown:         currentUser.Hometown,
		JobTitle:         currentUser.JobTitle,
		Education:        currentUser.Education,
		ReligiousBeliefs: currentUser.ReligiousBeliefs,
		DrinkingHabit:    currentUser.DrinkingHabit,
		SmokingHabit:     currentUser.SmokingHabit,
	}
	updateLocationGenderParams := migrations.UpdateUserLocationGenderParams{ /* ... as before ... */
		ID:        userID,
		Latitude:  currentUser.Latitude,
		Longitude: currentUser.Longitude,
		Gender:    currentUser.Gender,
	}
	updateAudioParams := migrations.UpdateAudioPromptParams{
		ID:                  userID,
		AudioPromptQuestion: currentUser.AudioPromptQuestion,
		AudioPromptAnswer:   currentUser.AudioPromptAnswer,
	}
	updateMediaParams := migrations.UpdateUserMediaURLsParams{
		ID:        userID,
		MediaUrls: currentUser.MediaUrls,
	}

	profileNeedsUpdate := false
	locationGenderNeedsUpdate := false
	promptsNeedUpdate := false
	audioPromptNeedsUpdate := false
	mediaUrlsNeedUpdate := false

	if reqData.Name != nil {
		trimmedName := strings.TrimSpace(*reqData.Name)
		if updateProfileParams.Name.String != trimmedName || !updateProfileParams.Name.Valid {
			updateProfileParams.Name = pgtype.Text{String: trimmedName, Valid: true}
			profileNeedsUpdate = true
			log.Printf("[EditProfile %d] Updating Name", userID)
		}
	}

	if reqData.Prompts != nil {
		log.Printf("[EditProfile %d] Processing Prompts update request", userID)
		newPrompts := *reqData.Prompts
		if err := validatePromptsInput(newPrompts); err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid prompts data: %v", err))
			return
		}
		promptsNeedUpdate = true
		log.Printf("[EditProfile %d] Marking Prompts for update", userID)
	}

	if reqData.AudioPrompt != nil {
		log.Printf("[EditProfile %d] Processing Audio Prompt update request", userID)
		audioReq := reqData.AudioPrompt
		hasChanges := false

		if audioReq.Question != nil {
			audioQEnum, err := parseAudioPromptEnum(*audioReq.Question)
			if err != nil {
				utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid audio_prompt question: %v", err))
				return
			}
			if !updateAudioParams.AudioPromptQuestion.Valid || updateAudioParams.AudioPromptQuestion.AudioPrompt != audioQEnum {
				updateAudioParams.AudioPromptQuestion = migrations.NullAudioPrompt{AudioPrompt: audioQEnum, Valid: true}
				hasChanges = true
				log.Printf("[EditProfile %d] Updating Audio Prompt Question", userID)
			}
		}

		if audioReq.AnswerUrl != nil {
			trimmedURL := strings.TrimSpace(*audioReq.AnswerUrl)
			_, err := url.ParseRequestURI(trimmedURL)
			if err != nil || trimmedURL == "" {
				utils.RespondWithError(w, http.StatusBadRequest, "Invalid audio_prompt answer URL format")
				return
			}
			if !updateAudioParams.AudioPromptAnswer.Valid || updateAudioParams.AudioPromptAnswer.String != trimmedURL {
				updateAudioParams.AudioPromptAnswer = pgtype.Text{String: trimmedURL, Valid: trimmedURL != ""}
				hasChanges = true
				log.Printf("[EditProfile %d] Updating Audio Prompt Answer URL", userID)
			}
		}

		if reqData.AudioPrompt != nil && (audioReq.Question == nil || audioReq.AnswerUrl == nil) {
			if audioReq.Question == nil && updateAudioParams.AudioPromptAnswer.Valid {
				if !updateAudioParams.AudioPromptQuestion.Valid {
					utils.RespondWithError(w, http.StatusBadRequest, "Audio prompt question is required when providing an answer URL")
					return
				}
			} else if audioReq.AnswerUrl == nil && updateAudioParams.AudioPromptQuestion.Valid {
				if !updateAudioParams.AudioPromptAnswer.Valid {
					utils.RespondWithError(w, http.StatusBadRequest, "Audio prompt answer URL is required when providing a question")
					return
				}
			}
			if audioReq.Question != nil && *audioReq.Question == "" {
				updateAudioParams.AudioPromptQuestion.Valid = false
				if updateAudioParams.AudioPromptAnswer.Valid {
					updateAudioParams.AudioPromptAnswer.Valid = false
					log.Printf("[EditProfile %d] Clearing Audio Prompt Answer URL due to question clearing", userID)
					hasChanges = true
				}
			}
			if audioReq.AnswerUrl != nil && *audioReq.AnswerUrl == "" {
				updateAudioParams.AudioPromptAnswer.Valid = false
				if updateAudioParams.AudioPromptQuestion.Valid {
					updateAudioParams.AudioPromptQuestion.Valid = false
					log.Printf("[EditProfile %d] Clearing Audio Prompt Question due to answer clearing", userID)
					hasChanges = true
				}
			}
		}

		if hasChanges {
			audioPromptNeedsUpdate = true
			log.Printf("[EditProfile %d] Marking Audio Prompt for update", userID)
		}
	}

	if reqData.MediaUrls != nil {
		log.Printf("[EditProfile %d] Processing Media URLs update request", userID)
		newUrls := *reqData.MediaUrls
		if err := validateMediaUrlsInput(newUrls); err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid media_urls data: %v", err))
			return
		}

		if !stringSlicesEqual(updateMediaParams.MediaUrls, newUrls) {
			updateMediaParams.MediaUrls = newUrls
			mediaUrlsNeedUpdate = true
			log.Printf("[EditProfile %d] Marking Media URLs for update", userID)
		}
	}

	if !profileNeedsUpdate && !locationGenderNeedsUpdate && !promptsNeedUpdate && !audioPromptNeedsUpdate && !mediaUrlsNeedUpdate {
		log.Printf("[EditProfile %d] No changes detected in request.", userID)
		utils.RespondWithJSON(w, http.StatusOK, EditProfileResponse{
			Success: true,
			Message: "No changes detected in profile data",
		})
		return
	}

	log.Printf("[EditProfile %d] Starting transaction. Flags - Profile: %t, LocGen: %t, Prompts: %t, Audio: %t, Media: %t",
		userID, profileNeedsUpdate, locationGenderNeedsUpdate, promptsNeedUpdate, audioPromptNeedsUpdate, mediaUrlsNeedUpdate)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("[EditProfile %d] Error starting transaction: %v", userID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error starting transaction")
		return
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)

	if profileNeedsUpdate {
		log.Printf("[EditProfile %d] Executing UpdateUserProfile...", userID)
		_, err = qtx.UpdateUserProfile(ctx, updateProfileParams)
		if err != nil {
			log.Printf("[EditProfile %d] Error executing UpdateUserProfile: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update profile details")
			return
		}
		log.Printf("[EditProfile %d] UpdateUserProfile successful.", userID)
	}

	if locationGenderNeedsUpdate {
		log.Printf("[EditProfile %d] Executing UpdateUserLocationGender...", userID)
		_, err = qtx.UpdateUserLocationGender(ctx, updateLocationGenderParams)
		if err != nil {
			log.Printf("[EditProfile %d] Error executing UpdateUserLocationGender: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update location/gender")
			return
		}
		log.Printf("[EditProfile %d] UpdateUserLocationGender successful.", userID)
	}

	if promptsNeedUpdate {
		log.Printf("[EditProfile %d] Executing Prompt Updates (Delete/Create)...", userID)
		if err := deletePrompts(ctx, qtx, userID); err != nil {
			log.Printf("[EditProfile %d] Error deleting existing prompts: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update prompts (delete step)")
			return
		}
		if err := createPrompts(ctx, qtx, userID, *reqData.Prompts); err != nil {
			log.Printf("[EditProfile %d] Error creating new prompts: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save new prompts: %v", err))
			return
		}
		log.Printf("[EditProfile %d] Prompts updated successfully.", userID)
	}

	if audioPromptNeedsUpdate {
		log.Printf("[EditProfile %d] Executing UpdateAudioPrompt...", userID)
		_, err = qtx.UpdateAudioPrompt(ctx, updateAudioParams)
		if err != nil {
			log.Printf("[EditProfile %d] Error executing UpdateAudioPrompt: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update audio prompt")
			return
		}
		log.Printf("[EditProfile %d] UpdateAudioPrompt successful.", userID)
	}

	if mediaUrlsNeedUpdate {
		log.Printf("[EditProfile %d] Executing UpdateUserMediaURLs...", userID)
		err = qtx.UpdateUserMediaURLs(ctx, updateMediaParams)
		if err != nil {
			log.Printf("[EditProfile %d] Error executing UpdateUserMediaURLs: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update media URLs")
			return
		}
		log.Printf("[EditProfile %d] UpdateUserMediaURLs successful.", userID)
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("[EditProfile %d] Error committing transaction: %v", userID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error saving profile changes")
		return
	}

	log.Printf("[EditProfile %d] Transaction committed successfully.", userID)
	utils.RespondWithJSON(w, http.StatusOK, EditProfileResponse{
		Success: true,
		Message: "Profile updated successfully",
	})
}

func parseAudioPromptEnum(s string) (migrations.AudioPrompt, error) {
	enumValue, ok := audioPromptMapEdit[s]
	if !ok {
		return "", fmt.Errorf("invalid audio prompt question value: '%s'", s)
	}
	return enumValue, nil
}

func validatePromptsInput(prompts []promptRequest) error {
	fmt.Printf("[Validation] Checking prompts. Count: %d\n", len(prompts))
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
			return fmt.Errorf("prompt %d: answer cannot be empty or just whitespace", i+1)
		}
		if utf8.RuneCountInString(p.Answer) > 255 {
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
			return fmt.Errorf("prompt %d: invalid question '%s' for category '%s': %w", i+1, p.Question, p.Category, parseErr)
		}

		questionKey := fmt.Sprintf("%s:%s", p.Category, p.Question)
		if promptQuestions[questionKey] {
			return fmt.Errorf("prompt question '%s' under category '%s' cannot be used more than once in the same request", p.Question, p.Category)
		}
		promptQuestions[questionKey] = true
		fmt.Printf("[Validation] Prompt %d: OK\n", i+1)
	}
	return nil
}

func validateMediaUrlsInput(urls []string) error {
	count := len(urls)
	if count < 3 {
		return fmt.Errorf("at least 3 media items (images/videos) are required")
	}
	if count > 6 {
		return fmt.Errorf("maximum of 6 media items allowed")
	}
	for i, u := range urls {
		trimmedU := strings.TrimSpace(u)
		if trimmedU == "" {
			return fmt.Errorf("media URL at index %d cannot be empty", i)
		}
		_, err := url.ParseRequestURI(trimmedU)
		if err != nil {
			return fmt.Errorf("invalid URL format for media item at index %d: %w", i, err)
		}
	}
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// - parseHeightStringEdit (defined above)
// - parseDatingIntentionEnum
// - parseReligionEnum
// - parseDrinkingSmokingEnum
// - deletePrompts (from createprofile or refactored)
// - createPrompts (from createprofile or refactored)
// - parseDateVibesEnum, parseGettingPersonalEnum, parseMyTypeEnum, parseStoryTimeEnum (from createprofile or refactored)
