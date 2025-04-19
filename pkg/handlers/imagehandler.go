package handlers

import (
	"encoding/json"
	"errors" // Import errors package
	"fmt"
	"log" // Import log package
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

// GeneratePresignedURLs handles the original /upload route.
// It requires 3-6 files and SAVES the resulting URLs to the user's profile.
// THIS FUNCTION REMAINS UNCHANGED.
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

	// Verify user exists (good practice)
	_, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Correct error check for pgx/v5
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to retrieve user: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Clear existing URLs before generating new ones for the main profile media
	err = queries.ClearUserMediaURLs(ctx, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear existing media URLs: %v", err), http.StatusInternalServerError)
		return
	}

	// AWS Configuration setup (same as before)
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		http.Error(w, "Missing AWS configuration", http.StatusInternalServerError)
		return
	}

	// Decode request body (same as before)
	var requestBody struct {
		Files []FileRequest `json:"files"`
	}
	err = json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// **Original File Count Validation (3-6)**
	fileCount := len(requestBody.Files)
	if fileCount < 3 {
		http.Error(w, "Requires minimum 3 files", http.StatusBadRequest)
		return
	}
	if fileCount > 6 {
		http.Error(w, "Requires maximum 6 files", http.StatusBadRequest)
		return
	}

	// S3 Session and URL Generation (same as before)
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))
	svc := s3.New(sess)
	var uploadURLs []UploadURL
	var permanentURLs []string // Used to store in DB
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

		// Sanitize filename before using it in the key
		sanitizedFilename := sanitizeFilename(file.Filename) // Call the function defined below
		key := fmt.Sprintf("uploads/%d/%s/%s",               // Include userID in the path
			userID,
			datePrefix,
			sanitizedFilename)

		req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
			Bucket:      aws.String(s3Bucket),
			Key:         aws.String(key),
			ContentType: aws.String(file.Type),
		})

		url, err := req.Presign(15 * time.Minute) // Presigned URL for upload
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate URL for %s: %v", file.Filename, err), http.StatusInternalServerError)
			return
		}

		uploadURLs = append(uploadURLs, UploadURL{
			Filename: file.Filename, // Return original filename
			Type:     file.Type,
			URL:      url,
		})

		// Construct the permanent public URL for DB storage
		publicURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Bucket, awsRegion, key)
		permanentURLs = append(permanentURLs, publicURL)
	}

	// **Database Update (Save Permanent URLs)**
	updateParams := migrations.UpdateUserMediaURLsParams{
		MediaUrls: permanentURLs,
		ID:        userID,
	}
	err = queries.UpdateUserMediaURLs(ctx, updateParams)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to store media URLs in database: %v", err), http.StatusInternalServerError)
		return
	}

	// Respond with upload URLs (same as before)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]UploadURL{
		"uploads": uploadURLs,
	})
}

// GenerateEditPresignedURLs handles the /api/edit-presigned-urls route.
// It allows 1-6 files and DOES NOT save the resulting URLs to the database.
// *** ADDED CHECK FOR EXISTING MEDIA ***
func GenerateEditPresignedURLs(w http.ResponseWriter, r *http.Request) {
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
	ctx := r.Context()    // Get context
	queries := db.GetDB() // Get DB queries instance
	if queries == nil {
		log.Println("GenerateEditPresignedURLs: Database connection not available")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// --- *** ADDED CHECK FOR EXISTING MEDIA *** ---
	log.Printf("GenerateEditPresignedURLs: Checking existing media for user %d", userID)
	user, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Defensive check
			log.Printf("GenerateEditPresignedURLs: User %d not found (unexpected after auth)", userID)
			http.Error(w, "User not found", http.StatusNotFound) // Or Internal Server Error? NotFound seems appropriate if user somehow disappeared
		} else {
			log.Printf("GenerateEditPresignedURLs: Error fetching user %d: %v", userID, err)
			http.Error(w, "Failed to retrieve user data", http.StatusInternalServerError)
		}
		return
	}

	if len(user.MediaUrls) < 3 {
		log.Printf("GenerateEditPresignedURLs: User %d has only %d media URLs, requires 3.", userID, len(user.MediaUrls))
		http.Error(w, "User must have at least 3 existing media items before requesting edit URLs", http.StatusBadRequest) // Use 400 Bad Request
		return
	}
	log.Printf("GenerateEditPresignedURLs: User %d has %d media URLs (>=3), check passed.", userID, len(user.MediaUrls))
	// --- *** END CHECK *** ---

	// AWS Configuration setup
	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		log.Println("GenerateEditPresignedURLs: Missing AWS configuration")
		http.Error(w, "Missing AWS configuration", http.StatusInternalServerError)
		return
	}

	// Decode request body
	var requestBody struct {
		Files []FileRequest `json:"files"`
	}
	err = json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// **MODIFIED File Count Validation (1-6)**
	fileCount := len(requestBody.Files)
	if fileCount < 1 { // Changed minimum to 1
		http.Error(w, "Requires minimum 1 file", http.StatusBadRequest)
		return
	}
	if fileCount > 6 { // Maximum remains 6
		http.Error(w, "Requires maximum 6 files", http.StatusBadRequest)
		return
	}

	// S3 Session and URL Generation
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))
	svc := s3.New(sess)
	var uploadURLs []UploadURL
	// No need for permanentURLs slice as we are not saving to DB
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

		// Use a different path prefix maybe? Or keep the same structure but just don't save.
		// Let's keep the structure consistent for now.
		sanitizedFilename := sanitizeFilename(file.Filename) // Call the function defined below
		// Use a distinct path prefix for these temporary edit uploads if desired, e.g., "temp-uploads"
		// key := fmt.Sprintf("temp-uploads/%d/%s/%s",
		key := fmt.Sprintf("uploads/%d/%s/%s", // Or keep same path as initial uploads
			userID,
			datePrefix,
			sanitizedFilename)

		req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
			Bucket:      aws.String(s3Bucket),
			Key:         aws.String(key),
			ContentType: aws.String(file.Type),
		})

		url, err := req.Presign(15 * time.Minute) // Presigned URL for upload
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate URL for %s: %v", file.Filename, err), http.StatusInternalServerError)
			return
		}

		uploadURLs = append(uploadURLs, UploadURL{
			Filename: file.Filename, // Return original filename
			Type:     file.Type,
			URL:      url,
		})

		// No construction or storage of the permanent URL needed here
	}

	// **NO Database Update Call Here**

	// Respond with upload URLs
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]UploadURL{
		"uploads": uploadURLs, // Use the same response structure as /upload
	})
}

// Helper function to sanitize filenames (Defined ONCE here)
