package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	db "github.com/arnnvv/peeple-api/db/sqlc"
	"github.com/arnnvv/peeple-api/pkg/envloader"
	"github.com/jackc/pgx/v5"
)

func hiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "hi")
}

func main() {
	err := envloader.LoadEnv(".env")
	if err != nil {
		fmt.Printf("Error loading .env: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("DATABASE_URL", os.Getenv("DATABASE_URL"))

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	q := db.New(conn)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Ensure that only GET requests are processed
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		// Write "hi" to the response
		fmt.Fprint(w, "hi")
	})

	// Start the server on port 8080
	fmt.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
