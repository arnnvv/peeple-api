package main

import (
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/arnnvv/peeple-api/pkg/envloader"
	"github.com/arnnvv/peeple-api/pkg/handlers"
	"github.com/arnnvv/peeple-api/pkg/token"
)

func main() {
	// Utilize all available CPUs
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Load environment variables
	if err := envloader.LoadEnv(".env"); err != nil {
		log.Fatalf("Error loading .env: %v", err)
	}

	// Configure HTTP server with timeouts
	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Configure routes
	http.HandleFunc("/", token.AuthMiddleware(handlers.ProtectedHandler))
	http.HandleFunc("/token", token.GenerateTokenHandler)
	http.HandleFunc("/new", handlers.CreateNewUser)

	log.Println("Server is running on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
