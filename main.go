package main

import (
	"context"
	"log"
	"os"

	"github.com/arnnvv/peeple-api/db"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	dbUrl := os.Getenv("DATABASE_URL")
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

	users, err := queries.ListAllUsers(ctx)
	if err != nil {
		log.Fatalf("Error fetching users: %v", err)
	}
	log.Printf("Found %d users", len(users))
	for _, user := range users {
		log.Printf("User: ID=%d, Name=%s, Email=%s", user.ID, user.Name, user.Email)
	}
}
