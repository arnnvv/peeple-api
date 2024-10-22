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
	"time"

	"github.com/arnnvv/peeple-api/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type CheckEmailResponse struct {
	Exists bool `json:"exists"`
}

var dummyUsers = make(map[string]string)

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

var jwtSecret []byte

func generateOTP(email string) string {
	otp := fmt.Sprintf("%06d", 100000+rand.Intn(900000))
	dummyUsers[email] = otp
	return otp
}

func generateJWT(email string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET environment variable is required")
	}
}

func main() {
	ticker := time.NewTicker(300 * time.Second)

	go func() {
		for range ticker.C {
			fmt.Println("Clearing OTP memory")
			dummyUsers = make(map[string]string)
		}
	}()

	dbUrl := os.Getenv("DATABASE_URL")
	gmail := os.Getenv("GMAIL")
	gmail_pass := os.Getenv("GMAIL_PASS")
	port := os.Getenv("PORT")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL is not set in the environment")
	}

	// Establish a connection to the PostgreSQL database
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to the database: %v", err)
	}
	defer conn.Close(ctx)

	queries := db.New(conn)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		if email == "" {
			http.Error(w, "Email is required", http.StatusBadRequest)
			return
		}

		log.Printf("Checking if email exists: %s", email)
		user, err := queries.GetUserByEmail(ctx, email)
		if err != nil {
			println("err not nil")
			if err.Error() == "no rows in result set" {
				log.Printf("No user with email %s, creating new user", email)
				_, err := queries.CreateUser(ctx, email)
				println("error may exist")
				if err != nil {
					println("error exist")
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					log.Printf("Error creating user: %v", err)
					return
				}
				println("error free")
				// Return response indicating user does not exist
				response := CheckEmailResponse{Exists: false}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			log.Printf("Error fetching user: %v", err)
			return
		}

		log.Println(user.Email)
		response := CheckEmailResponse{Exists: true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/send-otp", func(w http.ResponseWriter, r *http.Request) {
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
		if email == "" {
			http.Error(w, "Email is required", http.StatusBadRequest)
			return
		}

		log.Printf("Checking if email exists: %s", email)
		user, err := queries.GetUserByEmail(ctx, email)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		log.Printf("User found: %v", user.Email)

		auth := smtp.PlainAuth(
			"",
			gmail,
			gmail_pass,
			"smtp.gmail.com",
		)

		otp := generateOTP(user.Email)
		msg := fmt.Sprintf("Subject: Welcome to peeple\nYour OTP is %s", otp)

		err = smtp.SendMail(
			"smtp.gmail.com:587",
			auth,
			gmail,
			[]string{user.Email},
			[]byte(msg),
		)
		if err != nil {
			log.Printf("Failed to send OTP: %v", err)
			http.Error(w, "Failed to send OTP. Please try again later.", http.StatusInternalServerError)
			return
		}

		log.Printf("OTP sent successfully to %s", user.Email)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{"message": "OTP sent successfully"}
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/verify-otp", func(w http.ResponseWriter, r *http.Request) {
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
		if email == "" || otp == "" {
			http.Error(w, "Email and OTP are required", http.StatusBadRequest)
			return
		}

		log.Printf("Verifying OTP for %s", email)

		if dummyUsers[email] == otp {
			log.Printf("OTP verified for %s", email)

			token, err := generateJWT(email)
			if err != nil {
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
				log.Printf("Error generating token: %v", err)
				return
			}

			log.Printf("Token generated for %s: %s", email, token)

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"token": token})
		} else {
			log.Printf("Invalid OTP for %s. Provided OTP: %s, Expected OTP: %s", email, otp, dummyUsers[email])
			http.Error(w, "Invalid OTP", http.StatusUnauthorized)
		}
	})

	log.Println("Server is running on port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
