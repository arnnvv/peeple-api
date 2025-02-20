package handlers

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func GeneratePresignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	awsRegion := os.Getenv("AWS_REGION")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")

	// Validate configuration
	if awsRegion == "" || awsAccessKey == "" || awsSecretKey == "" || s3Bucket == "" {
		http.Error(w, "Somethin Missing", http.StatusInternalServerError)
		return
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	}))

	svc := s3.New(sess)

	query := r.URL.Query()
	filename := query.Get("filename")
	fileType := query.Get("type")

	if filename == "" || fileType == "" {
		http.Error(w, "filename and type parameters required", http.StatusBadRequest)
		return
	}

	key := fmt.Sprintf("uploads/%s/%s", time.Now().Format("2006-01-02"), filename)

	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"url":"%s"}`, url)
}
