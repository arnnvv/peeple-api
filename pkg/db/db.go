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

	dbConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}

	dbConfig.MaxConns = 50
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute

	newPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := newPool.Ping(ctx); err != nil {
		newPool.Close()
		return err
	}

	pool = newPool
	queries = migrations.New(pool)

	log.Println("Database connection pool initialized successfully")
	return nil
}

func GetDB() *migrations.Queries {
	if pool == nil {
		if err := InitDB(); err != nil {
			log.Printf("Failed to initialize database: %v", err)
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		log.Printf("Database connection lost, attempting to reconnect: %v", err)
		if pool != nil {
			pool.Close()
			pool = nil
		}

		if err := InitDB(); err != nil {
			log.Printf("Failed to reconnect to database: %v", err)
			return nil
		}

		queries = migrations.New(pool)
	}

	return queries
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
