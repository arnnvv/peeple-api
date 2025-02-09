package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/arnnvv/peeple-api/pkg/envloader"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pre-allocated responses
var (
	helloResponse   = []byte("hi, phone: ")
	errClaimsFailed = []byte("Failed to retrieve token claims")
)

func main() {
	// Utilize all available CPUs
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Load environment variables
	if err := envloader.LoadEnv(".env"); err != nil {
		log.Fatalf("Error loading .env: %v", err)
	}

	// Configure database connection pool
	dbConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	// Connection pool settings
	dbConfig.MaxConns = 50
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Configure HTTP server with timeouts
	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Configure routes
	http.HandleFunc("/token", token.GenerateTokenHandler)
	http.HandleFunc("/", token.AuthMiddleware(protectedHandler))

	log.Println("Server is running on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func protectedHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(errClaimsFailed)
		return
	}

	// Optimized response writing
	w.Header().Set("Content-Type", "text/plain")
	w.Write(helloResponse)
	w.Write([]byte(claims.PhoneNumber))
}
