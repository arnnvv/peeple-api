package main

import (
	"context"
	"fmt"
	"log"
	"os"

	db "github.com/arnnvv/peeple-api/db/sqlc"
	"github.com/arnnvv/peeple-api/pkg/envloader"
	"github.com/jackc/pgx/v5"
)

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

	// Create user
	newUser, err := q.CreateUser(context.Background(), "8580965219")
	if err != nil {
		log.Fatal("Error creating user:", err)
	}
	fmt.Printf("Created user: %+v\n", newUser)

	// Get user by phone number
	user, err := q.GetUserByPhoneNumber(context.Background(), "8580965219")
	if err != nil {
		log.Fatal("Error fetching user:", err)
	}
	fmt.Printf("Found user: %+v\n", user)
}
