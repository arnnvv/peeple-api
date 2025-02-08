package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	//	db "github.com/arnnvv/peeple-api/db/sqlc"
	"github.com/arnnvv/peeple-api/pkg/envloader"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/jackc/pgx/v5"
)

func main() {
	err := envloader.LoadEnv(".env")
	if err != nil {
		fmt.Printf("Error loading .env: %v\n", err)
		os.Exit(1)
	}

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	//	q := db.New(conn)

	// Token generation endpoint
	http.HandleFunc("/token", token.GenerateTokenHandler)

	// Protected endpoint with auth middleware
	http.HandleFunc("/", token.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the claims from the context using the string key "claims".
		claims, ok := r.Context().Value("claims").(*token.Claims)
		if !ok || claims == nil {
			http.Error(w, "Failed to retrieve token claims", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "hi, phone: %s", claims.PhoneNumber)
	}))

	fmt.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
