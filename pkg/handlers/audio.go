// FILE: pkg/handlers/audio.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils" // Import utils
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type AudioFileRequest struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	Prompt   string `json:"prompt"`
}

type AudioUploadURL struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Prompt   string `json:"prompt"`
}

var allowedAudioTypes = map[string]bool{
	"audio/mpeg":           true, // MP3
	"audio/wav":            true, // WAV
	"audio/ogg":            true, // OGG Vorbis/Opus
	"audio/webm":           true, // WebM Audio (often Opus or Vorbis)
	"audio/aac":            true, // AAC
	"audio/x-m4a":          true, // M4A (often AAC) - common Apple format
	"audio/mp4":            true, // MP4 audio (can contain AAC, ALAC, etc.)
	"audio/flac":           true, // FLAC
	"audio/opus":           true, // Opus
	"audio/amr":            true, // AMR (common in mobile)
	"audio/basic":          true, // Basic audio (.au, .snd)
	"audio/midi":           true, // MIDI
	"audio/x-aiff":         true, // AIFF
	"audio/x-pn-realaudio": true, // RealAudio
	"audio/x-tta":          true, // True Audio
	"audio/x-wavpack":      true, // WavPack
	"audio/x-ms-wma":       true, // WMA (use with caution, might need specific header checks)
}

var audioPromptMap map[string]migrations.AudioPrompt

func init() {
	audioPromptMap = map[string]migrations.AudioPrompt{
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

func parseMigrationsAudioPrompt(promptStr string) (migrations.AudioPrompt, error) {
	enumValue, ok := audioPromptMap[promptStr]
	if !ok {
		return "", fmt.Errorf("invalid audio prompt value: '%s'", promptStr)
	}
	return enumValue, nil
}

func GenerateAudioPresignedURL(w http.ResponseWriter, r *http.Request) {
	const operation = "handlers.GenerateAudioPresignedURL"
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST", operation) // Use local helper
		return
	}

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		respondWithError(w, http.StatusUnauthorized, "Authentication required: Invalid or missing token claims", operation) // Use local helper
		return
	}
	userID := claims.UserID

	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		log.Printf("[%s] Critical configuration error: Missing one or more AWS environment variables (REGION, ACCESS_KEY_ID, SECRET_ACCESS_KEY, S3_BUCKET)", operation)
		respondWithError(w, http.StatusInternalServerError, "Server configuration error prevents file uploads", operation) // Use local helper
		return
	}

	var requestBody AudioFileRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		log.Printf("[%s] Failed to decode request body: %v", operation, err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body format", operation) // Use local helper
		return
	}
	defer r.Body.Close()

	if requestBody.Filename == "" || requestBody.Type == "" || requestBody.Prompt == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields in request: filename, type, or prompt", operation) // Use local helper
		return
	}

	audioPromptEnum, err := parseMigrationsAudioPrompt(requestBody.Prompt)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), operation) // Use local helper
		return
	}

	if !isValidAudioType(requestBody.Type) {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported audio file type: '%s'", requestBody.Type), operation) // Use local helper
		return
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	})
	if err != nil {
		log.Printf("[%s] Failed to initialize AWS session: %v", operation, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to connect to storage service", operation) // Use local helper
		return
	}

	s3Client := s3.New(sess)

	s3Key := generateS3Key(int32(userID), audioPromptEnum, requestBody.Filename)

	presignedPutURL, permanentObjectURL, err := createPresignedURL(s3Client, s3Bucket, s3Key, requestBody.Type)
	if err != nil {
		log.Printf("[%s] Failed to generate presigned URL for key '%s': %v", operation, s3Key, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to prepare file upload URL", operation) // Use local helper
		return
	}

	queries := db.GetDB()

	err = handleDatabaseOperations(r.Context(), queries, int32(userID), permanentObjectURL, audioPromptEnum)
	if err != nil {
		log.Printf("[%s] Failed to update database for user %d with audio URL '%s': %v", operation, userID, permanentObjectURL, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to save audio information", operation) // Use local helper
		return
	}

	// Use utils.RespondWithJSON for success case too
	utils.RespondWithJSON(w, http.StatusOK, AudioUploadURL{
		Filename: requestBody.Filename,
		Type:     requestBody.Type,
		URL:      presignedPutURL,
		Prompt:   requestBody.Prompt,
	})
}

func generateS3Key(userID int32, prompt migrations.AudioPrompt, filename string) string {
	safeFilename := strings.ReplaceAll(filename, "/", "_")
	promptStr := strings.ToLower(string(prompt))

	return fmt.Sprintf("users/%d/audio/%s/%d-%s",
		userID,
		promptStr,
		time.Now().UnixNano(),
		safeFilename,
	)
}

func createPresignedURL(s3Client *s3.S3, bucket, key, fileType string) (string, string, error) {
	req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
	})

	presignDuration := 15 * time.Minute
	presignedURL, err := req.Presign(presignDuration)
	if err != nil {
		return "", "", fmt.Errorf("failed to presign request for key '%s': %w", key, err)
	}

	region := aws.StringValue(s3Client.Config.Region)
	permanentURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		bucket,
		region,
		key)

	return presignedURL, permanentURL, nil
}
func handleDatabaseOperations(ctx context.Context, queries *migrations.Queries, userID int32, audioURL string, prompt migrations.AudioPrompt) error {
	params := migrations.UpdateAudioPromptParams{
		AudioPromptQuestion: migrations.NullAudioPrompt{
			AudioPrompt: prompt,
			Valid:       true,
		},
		AudioPromptAnswer: pgtype.Text{
			String: audioURL,
			Valid:  true,
		},
		ID: userID,
	}

	_, err := queries.UpdateAudioPrompt(ctx, params)
	if err != nil {
		return fmt.Errorf("sqlc UpdateUserAudioPrompt failed for user %d: %w", userID, err)
	}

	return nil
}

// Local helper matching the signature used within this file
func respondWithError(w http.ResponseWriter, code int, message string, operation string) {
	log.Printf("[%s] Error %d: %s", operation, code, message)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	// Use the shared ErrorResponse structure from utils
	utils.RespondWithJSON(w, code, utils.ErrorResponse{
		Success: false,
		Message: message,
	})
}

func isValidAudioType(mimeType string) bool {
	_, ok := allowedAudioTypes[strings.ToLower(mimeType)]
	return ok
}
