package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GenerateVerificationPresignedURL(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	targetURL := os.Getenv("TARGET_URL")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	var user db.UserModel
	if result := db.DB.Where("id = ?", claims.UserID).First(&user); result.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
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

	var fileReq FileRequest
	if err := json.NewDecoder(r.Body).Decode(&fileReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if fileReq.Filename == "" || fileReq.Type == "" {
		http.Error(w, "Filename and type are required", http.StatusBadRequest)
		return
	}

	if !isImageType(fileReq.Type) {
		http.Error(w, "Only image files are allowed", http.StatusBadRequest)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	key := fmt.Sprintf("verification/%d/%s-%s",
		claims.UserID,
		timestamp,
		sanitizeFilename(fileReq.Filename))

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))

	svc := s3.New(sess)
	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileReq.Type),
	})

	presignedURL, err := req.Presign(15 * time.Minute)
	if err != nil {
		http.Error(w, "Failed to generate presigned URL", http.StatusInternalServerError)
		return
	}

	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)
	user.VerificationPic = &publicURL

	if targetURL != "" {
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			fmt.Printf("Error creating webhook request: %v\n", err)
		} else {
			req.Header.Set("Authorization", authHeader)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Error sending verification webhook: %v\n", err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					fmt.Printf("Webhook returned non-200 status: %d\n", resp.StatusCode)
				}
			}
		}
	}

	pendingStatus := enums.VerificationStatusPending
	user.VerificationStatus = &pendingStatus

	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Failed to store verification URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"upload_url": presignedURL,
	})
}

func sanitizeFilename(filename string) string {
	return strings.ReplaceAll(filepath.Base(filename), " ", "_")
}

func isImageType(mimeType string) bool {
	imageTypes := map[string]bool{
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"image/tiff":    true,
		"image/svg+xml": true,
	}
	return imageTypes[strings.ToLower(mimeType)]
}
