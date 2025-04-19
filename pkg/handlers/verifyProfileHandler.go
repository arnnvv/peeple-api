package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	"github.com/jackc/pgx/v5/pgtype"
)

func GenerateVerificationPresignedURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()

	authHeader := r.Header.Get("Authorization")
	targetURL := os.Getenv("TARGET_URL")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}
	userID := int32(claims.UserID)

	_, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error checking user", http.StatusInternalServerError)
		}
		return
	}

	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")
	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		log.Println("Missing AWS configuration")
		http.Error(w, "Missing server configuration", http.StatusInternalServerError)
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
		http.Error(w, "Only image file types are allowed for verification", http.StatusBadRequest)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	key := fmt.Sprintf("verification/%d/%s-%s",
		userID,
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
		// ACL: aws.String("public-read"), // If the final URL needs to be public directly
	})

	presignedURL, err := req.Presign(15 * time.Minute)
	if err != nil {
		http.Error(w, "Failed to generate upload URL", http.StatusInternalServerError)
		return
	}

	publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)

	updateParams := migrations.UpdateUserVerificationDetailsParams{
		ID: userID,
		VerificationPic: pgtype.Text{
			String: publicURL,
			Valid:  true,
		},
		VerificationStatus: migrations.VerificationStatusPending,
	}

	_, err = queries.UpdateUserVerificationDetails(ctx, updateParams)
	if err != nil {
		log.Printf("Failed to store verification URL for user %d: %v", userID, err)
		http.Error(w, "Failed to update verification details in database", http.StatusInternalServerError)
		return
	}

	if targetURL != "" {
		go func() {
			webhookReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, targetURL, nil)
			if err != nil {
				fmt.Printf("Error creating webhook request: %v\n", err)
				return
			}
			webhookReq.Header.Set("Authorization", authHeader)
			// webhookReq.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))
			// webhookReq.Header.Set("X-Event-Type", "verification_pending")

			client := &http.Client{Timeout: 15 * time.Second}
			resp, err := client.Do(webhookReq)
			if err != nil {
				fmt.Printf("Error sending verification webhook for user %d: %v\n", userID, err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					fmt.Printf("Webhook for user %d returned non-2xx status: %d\n", userID, resp.StatusCode)
					// bodyBytes, _ := io.ReadAll(resp.Body)
					// fmt.Printf("Webhook response body: %s\n", string(bodyBytes))
				} else {
					fmt.Printf("Webhook notification sent successfully for user %d.\n", userID)
				}
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"upload_url": presignedURL,
	})
}

func sanitizeFilename(filename string) string {
	name := filepath.Base(filename)
	name = strings.ReplaceAll(name, " ", "_")
	return name
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
