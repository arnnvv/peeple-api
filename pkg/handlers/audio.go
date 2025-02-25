package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type AudioFileRequest struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
}

type AudioUploadURL struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

var allowedAudioTypes = map[string]bool{
	"audio/mpeg":   true, // MP3
	"audio/wav":    true, // WAV
	"audio/ogg":    true, // OGG
	"audio/webm":   true, // WEBM
	"audio/aac":    true, // AAC
	"audio/x-m4a":  true, // M4A
	"audio/x-aiff": true, // AIFF
	"audio/flac":   true, // FLAC
}

func isValidAudioType(mimeType string) bool {
	return allowedAudioTypes[strings.ToLower(mimeType)]
}

func GenerateAudioPresignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get claims from middleware context
	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	// Get user from database
	var user db.UserModel
	if result := db.DB.Preload("AudioPrompt").Where("id = ?", claims.UserID).First(&user); result.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// AWS Configuration
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		http.Error(w, "Missing AWS configuration", http.StatusInternalServerError)
		return
	}

	// Decode request body
	var requestBody AudioFileRequest
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate audio file type
	if !isValidAudioType(requestBody.Type) {
		http.Error(w, fmt.Sprintf("Audio type %s is not allowed", requestBody.Type), http.StatusBadRequest)
		return
	}

	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))
	svc := s3.New(sess)

	// Create unique S3 key with timestamp
	timestamp := time.Now().Format("20060102-150405")
	key := fmt.Sprintf("audio_uploads/%s/%s-%s",
		time.Now().Format("2006-01-02"),
		timestamp,
		requestBody.Filename,
	)

	// Generate presigned URL
	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(requestBody.Type),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		http.Error(w, "Failed to generate presigned URL", http.StatusInternalServerError)
		return
	}

	// Create permanent URL
	permanentURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)

	// Update audio prompt in database
	audioPrompt := db.AudioPromptModel{
		UserID:   user.ID,
		AudioURL: permanentURL,
	}

	// Use transaction for data consistency
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete existing audio prompt if exists
	if user.AudioPrompt != nil {
		if err := tx.Delete(user.AudioPrompt).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to remove existing audio prompt", http.StatusInternalServerError)
			return
		}
	}

	// Create new audio prompt
	if err := tx.Create(&audioPrompt).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to save audio URL", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Database operation failed", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AudioUploadURL{
		Filename: requestBody.Filename,
		Type:     requestBody.Type,
		URL:      url,
	})
}
