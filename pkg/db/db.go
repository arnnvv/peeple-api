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

// InitDB initializes the database connection pool.
func InitDB() error {
	poolMu.Lock()
	defer poolMu.Unlock()

	if pool != nil {
		log.Println("Database connection pool already initialized.")
		return nil // Already initialized
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	var err error
	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Printf("Error parsing DATABASE_URL: %v", err)
		return err
	}

	// Configure pool settings (adjust as needed)
	dbConfig.MaxConns = 50
	dbConfig.MinConns = 5 // Can start lower
	dbConfig.MaxConnLifetime = time.Hour
	dbConfig.MaxConnIdleTime = 5 * time.Minute // Reduce idle time
	dbConfig.HealthCheckPeriod = 1 * time.Minute
	dbConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	log.Println("Attempting to connect to database...")
	// Retry logic could be added here if desired
	newPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Printf("Unable to create connection pool: %v", err)
		return err
	}

	// Ping the database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = newPool.Ping(ctx); err != nil {
		newPool.Close() // Close the pool if ping fails
		log.Printf("Database ping failed after connection: %v", err)
		return err
	}

	pool = newPool
	queries = migrations.New(pool) // Create queries instance

	log.Println("Database connection pool initialized successfully")
	return nil
}

// GetDB returns the queries instance, ensuring the pool is initialized.
func GetDB() *migrations.Queries {
	poolMu.Lock()
	// If pool is nil, InitDB likely failed or hasn't been called.
	// It's better to let InitDB handle the initialization.
	if pool == nil {
		poolMu.Unlock()
		log.Println("GetDB called but pool is not initialized. Check InitDB call.")
		// Returning nil forces callers to handle the uninitialized state.
		// Alternatively, could attempt InitDB here, but it might hide startup issues.
		return nil
	}
	q := queries // Get the current queries instance
	poolMu.Unlock()
	return q
}

// GetPool returns the raw connection pool. Use with caution.
func GetPool() *pgxpool.Pool {
	poolMu.Lock()
	p := pool // Get the current pool instance
	poolMu.Unlock()
	// We check if 'p' is nil AFTER unlocking to avoid holding the lock
	// while potentially calling GetDB() which also locks.
	if p == nil {
		log.Println("GetPool called but pool is not initialized. Check InitDB call.")
		// Attempt to initialize or return nil? Returning nil is safer for now.
		return nil
	}
	return p
}

// CloseDB closes the database connection pool.
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
