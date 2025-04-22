package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// type FileRequest struct {
//  Filename string `json:"filename"`
//  Type     string `json:"type"`
// }

type ChatMediaUploadResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	PresignedURL string `json:"presigned_url,omitempty"`
	ObjectURL    string `json:"object_url,omitempty"`
	Filename     string `json:"filename,omitempty"`
	Type         string `json:"type,omitempty"`
}

func GenerateChatMediaPresignedURL(w http.ResponseWriter, r *http.Request) {
	const operation = "handlers.GenerateChatMediaPresignedURL"
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST", operation)
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		respondWithError(w, http.StatusUnauthorized, "Authentication required: Invalid or missing token claims", operation)
		return
	}
	userID := claims.UserID

	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		log.Printf("[%s] Critical configuration error: Missing one or more AWS environment variables", operation)
		respondWithError(w, http.StatusInternalServerError, "Server configuration error prevents file uploads", operation)
		return
	}

	var req FileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[%s] Failed to decode request body for user %d: %v", operation, userID, err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body format", operation)
		return
	}
	defer r.Body.Close()

	if req.Filename == "" || req.Type == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields in request: filename, type", operation)
		return
	}

	if !isAllowedFileType(req.Type) {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported file type for chat: '%s'", req.Type), operation)
		return
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	})
	if err != nil {
		log.Printf("[%s] Failed to initialize AWS session for user %d: %v", operation, userID, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to connect to storage service", operation)
		return
	}
	s3Client := s3.New(sess)

	timestamp := time.Now().UnixNano()
	sanitizedFilename := sanitizeFilename(req.Filename)
	s3Key := fmt.Sprintf("chat-media/%d/%d-%s", userID, timestamp, sanitizedFilename)

	presignedPutURL, permanentObjectURL, err := createChatPresignedURL(s3Client, s3Bucket, s3Key, req.Type)
	if err != nil {
		log.Printf("[%s] Failed to generate presigned URL for user %d, key '%s': %v", operation, userID, s3Key, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to prepare file upload URL", operation)
		return
	}

	log.Printf("[%s] Generated chat media URLs for user %d: Presigned=%s..., Permanent=%s", operation, userID, presignedPutURL[:min(50, len(presignedPutURL))], permanentObjectURL)

	utils.RespondWithJSON(w, http.StatusOK, ChatMediaUploadResponse{
		Success:      true,
		PresignedURL: presignedPutURL,
		ObjectURL:    permanentObjectURL,
		Filename:     req.Filename,
		Type:         req.Type,
	})
}

func createChatPresignedURL(s3Client *s3.S3, bucket, key, fileType string) (string, string, error) {
	req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
		// Consider ACL if needed, or handle permissions via bucket policy
		// ACL: aws.String("private"),
	})

	presignDuration := 15 * time.Minute
	presignedURL, err := req.Presign(presignDuration)
	if err != nil {
		return "", "", fmt.Errorf("failed to presign chat media request for key '%s': %w", key, err)
	}

	region := aws.StringValue(s3Client.Config.Region)
	permanentURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		bucket,
		region,
		key)

	return presignedURL, permanentURL, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
