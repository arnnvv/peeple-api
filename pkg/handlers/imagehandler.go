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

type FileRequest struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
}

type UploadURL struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"image/tiff":      true,
	"image/svg+xml":   true,
	"video/mp4":       true,
	"video/mpeg":      true,
	"video/ogg":       true,
	"video/webm":      true,
	"video/x-msvideo": true,
	"video/quicktime": true,
}

func isAllowedFileType(mimeType string) bool {
	return allowedMimeTypes[strings.ToLower(mimeType)]
}

func GeneratePresignedURLs(w http.ResponseWriter, r *http.Request) {
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

	// Get user from database using UserID
	var user db.UserModel
	result := db.DB.Where("id = ?", claims.UserID).First(&user)
	if result.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Clear existing media URLs
	user.MediaURLs = []string{}
	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Failed to clear existing media URLs", http.StatusInternalServerError)
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
	var requestBody struct {
		Files []FileRequest `json:"files"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate file count
	fileCount := len(requestBody.Files)
	if fileCount < 3 {
		http.Error(w, "Requires minimum 3 files", http.StatusBadRequest)
		return
	}
	if fileCount > 6 {
		http.Error(w, "Requires maximum 6 files", http.StatusBadRequest)
		return
	}

	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))
	svc := s3.New(sess)
	var uploadURLs []UploadURL
	var permanentURLs []string
	datePrefix := time.Now().Format("2006-01-02")

	for _, file := range requestBody.Files {
		if file.Filename == "" || file.Type == "" {
			http.Error(w, "Filename and type are required for all files", http.StatusBadRequest)
			return
		}

		if !isAllowedFileType(file.Type) {
			http.Error(w, fmt.Sprintf("File type %s is not allowed", file.Type), http.StatusBadRequest)
			return
		}

		// Create S3 key for the file
		key := fmt.Sprintf("uploads/%s/%s", datePrefix, file.Filename)

		req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
			Bucket:      aws.String(s3Bucket),
			Key:         aws.String(key),
			ContentType: aws.String(file.Type),
		})

		url, err := req.Presign(15 * time.Minute)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate URL for %s: %v", file.Filename, err), http.StatusInternalServerError)
			return
		}

		uploadURLs = append(uploadURLs, UploadURL{
			Filename: file.Filename,
			Type:     file.Type,
			URL:      url,
		})

		// Form the permanent (public) URL to be stored in the database.
		// Adjust the URL format based on your S3 configuration or CloudFront setup.
		publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)
		permanentURLs = append(permanentURLs, publicURL)
	}

	// Update the user's media URLs with the permanent links
	user.MediaURLs = permanentURLs
	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Failed to store media URLs in database", http.StatusInternalServerError)
		return
	}

	// Respond with the presigned upload URLs for the client to use
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]UploadURL{
		"uploads": uploadURLs,
	})
}
