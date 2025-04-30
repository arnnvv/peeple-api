// FILE: pkg/handlers/chatMediaHandler.go
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils" // Assuming utils package exists for RespondWithJSON/ErrorResponse
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// --- Define allowed MIME types SPECIFICALLY for chat ---
// Allowing only Images and Audio as requested.
var allowedChatMimeTypes = map[string]bool{
	// Images
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/jpg":  true, // Alias for jpeg often used

	// Audio (Common types including the ones from the error log)
	"audio/mpeg":  true, // MP3
	"audio/ogg":   true, // OGG Vorbis/Opus
	"audio/wav":   true, // WAV
	"audio/aac":   true, // AAC
	"audio/opus":  true, // Opus
	"audio/webm":  true, // WebM Audio
	"audio/mp4":   true, // M4A often maps to this
	"audio/x-m4a": true, // Explicit M4A
	"audio/m4a":   true, // Another common M4A variant
	// Add "audio/flac" if needed
}

// --- Create a specific check function for chat ---
func isAllowedChatFileType(mimeType string) bool {
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))
	_, ok := allowedChatMimeTypes[normalizedMime]
	log.Printf("[isAllowedChatFileType] Checking '%s': Allowed=%t", normalizedMime, ok) // Added logging
	return ok
}

// --- FileRequest struct (assuming it's defined like this) ---
// If FileRequest is defined in imagehandler.go and imported, you don't need this here.
// If not, define it:
// type FileRequest struct {
// 	Filename string `json:"filename"`
// 	Type     string `json:"type"`
// }

// --- Keep ChatMediaUploadResponse struct ---
type ChatMediaUploadResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	PresignedURL string `json:"presigned_url,omitempty"`
	ObjectURL    string `json:"object_url,omitempty"`
	Filename     string `json:"filename,omitempty"`
	Type         string `json:"type,omitempty"`
}

// --- Main Handler ---
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

	var req FileRequest // Using the FileRequest struct (ensure it's accessible)
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

	// --- *** Use the NEW chat-specific validation function *** ---
	if !isAllowedChatFileType(req.Type) {
		log.Printf("[%s] Unsupported file type '%s' rejected for user %d", operation, req.Type, userID)
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported file type for chat: '%s'. Allowed types are images and audio.", req.Type), operation)
		return
	}
	// --- *** END CHANGE *** ---

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
	sanitizedFilename := sanitizeFilename(req.Filename) // Use the helper
	s3Key := fmt.Sprintf("chat-media/%d/%d-%s", userID, timestamp, sanitizedFilename)

	presignedPutURL, permanentObjectURL, err := createChatPresignedURL(s3Client, s3Bucket, s3Key, req.Type) // Use the helper
	if err != nil {
		log.Printf("[%s] Failed to generate presigned URL for user %d, key '%s': %v", operation, userID, s3Key, err)
		respondWithError(w, http.StatusInternalServerError, "Failed to prepare file upload URL", operation)
		return
	}

	log.Printf("[%s] Generated chat media URLs for user %d: Permanent=%s", operation, userID, permanentObjectURL)

	utils.RespondWithJSON(w, http.StatusOK, ChatMediaUploadResponse{
		Success:      true,
		PresignedURL: presignedPutURL,
		ObjectURL:    permanentObjectURL,
		Filename:     req.Filename, // Return original filename for reference
		Type:         req.Type,
	})
}

// --- Helper Functions (Copied/Adapted from previous context) ---

// Creates the presigned PUT URL and the final object URL
func createChatPresignedURL(s3Client *s3.S3, bucket, key, fileType string) (string, string, error) {
	req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
		// You might want to control ACL here depending on your bucket policy
		// ACL: aws.String("private"), // Example: if files should not be public by default
	})

	presignDuration := 15 * time.Minute // Standard duration for upload URLs
	presignedURL, err := req.Presign(presignDuration)
	if err != nil {
		return "", "", fmt.Errorf("failed to presign chat media request for key '%s': %w", key, err)
	}

	// Construct the final URL assuming standard S3 path-style or virtual-hosted style access
	// Adjust this if your bucket access pattern is different (e.g., using CloudFront)
	region := aws.StringValue(s3Client.Config.Region)
	// Example format, adjust if needed (e.g., using FIPS endpoints, different domain)
	permanentURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		bucket,
		region,
		key)

	return presignedURL, permanentURL, nil
}

// Helper to sanitize filename (basic example)

// Helper for min function (if not available elsewhere)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
