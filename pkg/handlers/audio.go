package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"gorm.io/gorm"
)

type AudioFileRequest struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	Prompt   string `json:"prompt"` // Added prompt field
}

type AudioUploadURL struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Prompt   string `json:"prompt"`
}

var allowedAudioTypes = map[string]bool{
	"audio/mpeg":   true,
	"audio/wav":    true,
	"audio/ogg":    true,
	"audio/webm":   true,
	"audio/aac":    true,
	"audio/x-m4a":  true,
	"audio/x-aiff": true,
	"audio/flac":   true,
}

func GenerateAudioPresignedURL(w http.ResponseWriter, r *http.Request) {
	const operation = "audio_upload"

	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed", operation)
		return
	}

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required", operation)
		return
	}

	// Validate AWS configuration
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")
	
	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		respondWithError(w, http.StatusInternalServerError,
			"Missing AWS configuration", operation)
		return
	}

	var requestBody AudioFileRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format", operation)
		return
	}

	// Validate audio prompt
	audioPrompt, err := enums.ParseAudioPrompt(requestBody.Prompt)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, 
			fmt.Sprintf("Invalid audio prompt: %v", err), operation)
		return
	}

	if !isValidAudioType(requestBody.Type) {
		respondWithError(w, http.StatusBadRequest,
			fmt.Sprintf("Unsupported audio type: %s", requestBody.Type), operation)
		return
	}

	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to initialize AWS session", operation)
		return
	}

	svc := s3.New(sess)
	key := generateS3Key(claims.UserID, audioPrompt, requestBody.Filename)

	presignedURL, permanentURL, err := createPresignedURL(svc, s3Bucket, key, requestBody.Type)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Failed to generate upload URL", operation)
		return
	}

	// Database operation
	if err := handleDatabaseOperations(r.Context(), claims.UserID, permanentURL, audioPrompt); err != nil {
		respondWithError(w, http.StatusInternalServerError,
			err.Error(), operation)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AudioUploadURL{
		Filename: requestBody.Filename,
		Type:     requestBody.Type,
		URL:      presignedURL,
		Prompt:   requestBody.Prompt,
	})
}

func generateS3Key(userID uint, prompt enums.AudioPrompt, filename string) string {
	return fmt.Sprintf("users/%d/audio/%s/%d-%s",
		userID,
		strings.ToLower(string(prompt)),
		time.Now().UnixNano(),
		filename,
	)
}

func createPresignedURL(svc *s3.S3, bucket, key, fileType string) (string, string, error) {
	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("presign error: %w", err)
	}

	return url, fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", 
		bucket, 
		*svc.Config.Region, 
		key), nil
}

func handleDatabaseOperations(ctx context.Context, userID uint, audioURL string, prompt enums.AudioPrompt) error {
	tx := db.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("transaction begin failed: %w", tx.Error)
	}
	defer tx.Rollback()

	// Upsert operation
	var existing db.AudioPromptModel
	if err := tx.Where("user_id = ? AND prompt = ?", userID, prompt).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("database lookup failed: %w", err)
		}
	}

	if existing.ID != 0 {
		if err := tx.Model(&existing).Update("audio_url", audioURL).Error; err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	} else {
		newPrompt := db.AudioPromptModel{
			UserID:   userID,
			AudioURL: audioURL,
			Prompt:   prompt,
		}
		if err := tx.Create(&newPrompt).Error; err != nil {
			return fmt.Errorf("create failed: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func respondWithError(w http.ResponseWriter, code int, message string, operation string) {
	log.Printf("[%s] Error %d: %s", operation, code, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"error":   http.StatusText(code),
		"message": message,
	})
}

func isValidAudioType(mimeType string) bool {
	return allowedAudioTypes[strings.ToLower(mimeType)]
}
