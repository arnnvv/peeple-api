package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

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

func GeneratePresignedURLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get AWS configuration from environment
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	// Validate configuration
	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		http.Error(w, "Missing AWS configuration", http.StatusInternalServerError)
		return
	}

	// Decode JSON request body
	var requestBody struct {
		Files []FileRequest `json:"files"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate file count between 3-6
	fileCount := len(requestBody.Files)
	if fileCount < 3 {
		http.Error(w, "Requires minnimum 3 files", http.StatusBadRequest)
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
	datePrefix := time.Now().Format("2006-01-02")

	for _, file := range requestBody.Files {
		if file.Filename == "" || file.Type == "" {
			http.Error(w, "Filename and type are required for all files",
				http.StatusBadRequest)
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
			http.Error(w, fmt.Sprintf("Failed to generate URL for %s: %v",
				file.Filename, err), http.StatusInternalServerError)
			return
		}

		uploadURLs = append(uploadURLs, UploadURL{
			Filename: file.Filename,
			Type:     file.Type,
			URL:      url,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]UploadURL{
		"uploads": uploadURLs,
	})
}
