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
		return nil
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Printf("Error parsing DATABASE_URL: %v", err)
		return err
	}

	dbConfig.MaxConns = 50
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute
	dbConfig.HealthCheckPeriod = 1 * time.Minute

	log.Println("Attempting to connect to database...")
	newPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Printf("Unable to create connection pool: %v", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := newPool.Ping(ctx); err != nil {
		newPool.Close()
		log.Printf("Database ping failed: %v", err)
		return err
	}

	pool = newPool
	queries = migrations.New(pool)

	log.Println("Database connection pool initialized successfully")
	return nil
}

func GetDB() *migrations.Queries {
	poolMu.Lock()
	if pool == nil {
		poolMu.Unlock()
		if err := InitDB(); err != nil {
			log.Printf("Failed to initialize database connection: %v", err)
			return nil
		}
		poolMu.Lock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		log.Printf("Database connection check failed: %v. Attempting to reconnect...", err)
		if pool != nil {
			pool.Close()
			pool = nil
			queries = nil
		}
		poolMu.Unlock()

		if err := InitDB(); err != nil {
			log.Printf("Failed to reconnect to database: %v", err)
			return nil
		}
		log.Println("Successfully reconnected to the database.")
		poolMu.Lock()
	}

	currentQueries := queries
	poolMu.Unlock()
	return currentQueries
}

func GetPool() *pgxpool.Pool {
	if GetDB() == nil {
		return nil
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
