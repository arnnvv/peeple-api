package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type CheckEmailResponse struct {
	Exists bool `json:"exists"`
}

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

var (
	dummyUsers = make(map[string]string)
	jwtSecret  []byte
	dbQueries  *db.Queries
	mu         sync.Mutex // Mutex for safe concurrent map access
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET environment variable is required")
	}
}

func main() {
	port := os.Getenv("PORT")
	initDB()
	initOTPCleaner()

	http.HandleFunc("/", handleEmailCheck)
	http.HandleFunc("/send-otp", handleSendOTP)
	http.HandleFunc("/verify-otp", handleVerifyOTP)
	http.Handle("/get-user", verifyToken(http.HandlerFunc(handleGetUser)))

	log.Println("Server is running on port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}

// initDB sets up the database connection and queries instance.
func initDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set in the environment")
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to the database: %v", err)
	}
	dbQueries = db.New(conn)
}

// initOTPCleaner starts a background goroutine to clear OTPs every 5 minutes.
func initOTPCleaner() {
	ticker := time.NewTicker(300 * time.Second)
	go func() {
		for range ticker.C {
			log.Println("Clearing OTP memory")
			mu.Lock()
			dummyUsers = make(map[string]string)
			mu.Unlock()
		}
	}()
}

func verifyToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") || len(strings.Split(authHeader, " ")) != 2 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenString := strings.Split(authHeader, " ")[1]

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "email", claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleEmailCheck checks if a user exists by email and creates one if not.
func handleEmailCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := requestBody.Email
	emailExists := emailExists(email)

	// Create the user if they don't exist, then send the response
	if !emailExists {
		if err := createUser(email); err != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			log.Printf("Error creating user: %v", err)
			return
		}
		emailExists = true
	}

	sendJSONResponse(w, CheckEmailResponse{Exists: emailExists})
}

// emailExists checks if a user exists by email.
func emailExists(email string) bool {
	_, err := dbQueries.GetUserByEmail(context.Background(), email)
	return err == nil
}

// createUser creates a new user with the given email.
func createUser(email string) error {
	_, err := dbQueries.CreateUser(context.Background(), email)
	return err
}

// handleSendOTP generates and sends an OTP to the user's email.
func handleSendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := requestBody.Email

	if !emailExists(email) {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	otp := generateOTP(email)

	go func() {
		if err := sendOTPByEmail(email, otp); err != nil {
			log.Printf("Failed to send OTP: %v", err)
		}
	}()

	sendJSONResponse(w, map[string]string{"message": "OTP sent successfully"})
}

// generateOTP generates a random 6-digit OTP.
func generateOTP(email string) string {
	otp := fmt.Sprintf("%06d", 100000+rand.Intn(900000))
	mu.Lock()
	dummyUsers[email] = otp
	mu.Unlock()
	return otp
}

// sendOTPByEmail sends the OTP to the given email using SMTP.
func sendOTPByEmail(email, otp string) error {
	auth := smtp.PlainAuth("", os.Getenv("GMAIL"), os.Getenv("GMAIL_PASS"), "smtp.gmail.com")
	msg := fmt.Sprintf("Subject: Your OTP\nYour OTP is %s", otp)
	return smtp.SendMail("smtp.gmail.com:587", auth, os.Getenv("GMAIL"), []string{email}, []byte(msg))
}

// handleVerifyOTP verifies the OTP and returns a JWT if correct.
func handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := requestBody.Email
	otp := requestBody.OTP

	if isValidOTP(email, otp) {
		token, err := generateJWT(email)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			log.Printf("Error generating token: %v", err)
			return
		}
		sendJSONResponse(w, map[string]string{"token": token})
	} else {
		http.Error(w, "Invalid OTP", http.StatusUnauthorized)
	}
}

// isValidOTP checks if the provided OTP matches the stored OTP for the email.
func isValidOTP(email, otp string) bool {
	mu.Lock()
	defer mu.Unlock()
	return dummyUsers[email] == otp
}

// generateJWT creates a new JWT for the given email.
func generateJWT(email string) (string, error) {
	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func handleGetUser(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value("email").(string)
	user, err := dbQueries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching user", http.StatusInternalServerError)
			log.Printf("Error fetching user: %v", err)
		}
		return
	}

	sendJSONResponse(w, user)
}

// sendJSONResponse writes a JSON response with the provided data.
func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
