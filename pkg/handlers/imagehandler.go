package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jackc/pgx/v5"
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

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	userID := int32(claims.UserID)
	ctx := r.Context()

	queries := db.GetDB()

	_, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to retrieve user: %v", err), http.StatusInternalServerError)
		}
		return
	}

	err = queries.ClearUserMediaURLs(ctx, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear existing media URLs: %v", err), http.StatusInternalServerError)
		return
	}

	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		http.Error(w, "Missing AWS configuration", http.StatusInternalServerError)
		return
	}

	var requestBody struct {
		Files []FileRequest `json:"files"`
	}

	err = json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fileCount := len(requestBody.Files)
	if fileCount < 3 {
		http.Error(w, "Requires minimum 3 files", http.StatusBadRequest)
		return
	}
	if fileCount > 6 {
		http.Error(w, "Requires maximum 6 files", http.StatusBadRequest)
		return
	}

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

		publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)
		permanentURLs = append(permanentURLs, publicURL)
	}

	updateParams := migrations.UpdateUserMediaURLsParams{
		MediaUrls: permanentURLs,
		ID:        userID,
	}
	err = queries.UpdateUserMediaURLs(ctx, updateParams)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to store media URLs in database: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]UploadURL{
		"uploads": uploadURLs,
	})
}
