package db

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool    *pgxpool.Pool
	poolMu  sync.Mutex
	queries *migrations.Queries
)

func InitDB() error {
	poolMu.Lock()
	defer poolMu.Unlock()

	if pool != nil {
		return nil // Already initialized
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set") // Fatal error if not set
	}

	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Printf("Error parsing DATABASE_URL: %v", err) // Log error
		return err
	}

	// Consider making these configurable via env vars as well
	dbConfig.MaxConns = 50
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute
	dbConfig.HealthCheckPeriod = 1 * time.Minute // Add health check

	log.Println("Attempting to connect to database...")
	newPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Printf("Unable to create connection pool: %v", err) // Log error
		return err
	}

	// Ping the database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := newPool.Ping(ctx); err != nil {
		newPool.Close()                             // Close the pool if ping fails
		log.Printf("Database ping failed: %v", err) // Log error
		return err
	}

	pool = newPool
	queries = migrations.New(pool) // Initialize queries *after* successful connection

	log.Println("Database connection pool initialized successfully")
	return nil
}

// GetDB returns the sqlc Queries object, initializing the pool if necessary.
// Returns nil if initialization or ping fails.
func GetDB() *migrations.Queries {
	poolMu.Lock() // Ensure thread safety during check/init
	if pool == nil {
		poolMu.Unlock() // Unlock before calling InitDB to avoid deadlock
		if err := InitDB(); err != nil {
			log.Printf("Failed to initialize database connection: %v", err)
			return nil // Return nil explicitly on error
		}
		poolMu.Lock() // Lock again after potential initialization
	}

	// Check connection health before returning (optional but good practice)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		log.Printf("Database connection check failed: %v. Attempting to reconnect...", err)
		// Close existing potentially broken pool
		if pool != nil {
			pool.Close()
			pool = nil
			queries = nil
		}
		poolMu.Unlock() // Unlock before calling InitDB again

		// Attempt to re-initialize
		if err := InitDB(); err != nil {
			log.Printf("Failed to reconnect to database: %v", err)
			return nil
		}
		log.Println("Successfully reconnected to the database.")
		poolMu.Lock() // Lock again after successful re-init
	}

	currentQueries := queries
	poolMu.Unlock() // Unlock before returning
	return currentQueries
}

// GetPool returns the raw pgxpool.Pool, initializing if necessary.
// Useful for operations requiring direct pool access (like transactions).
// Returns nil if initialization fails.
func GetPool() *pgxpool.Pool {
	// Use GetDB to ensure initialization and health check
	if GetDB() == nil {
		return nil // If GetDB failed, pool is not usable
	}
	poolMu.Lock()
	defer poolMu.Unlock()
	return pool
}

func CloseDB() {
	poolMu.Lock()
	defer poolMu.Unlock()

	if pool != nil {
		pool.Close()
		pool = nil
		queries = nil
		log.Println("Database connection pool closed")
	}
}
