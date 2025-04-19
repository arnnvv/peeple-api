package db

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool    *pgxpool.Pool
	once    sync.Once
	initErr error
	queries *migrations.Queries
)

func InitDB(dbURL string) error {
	once.Do(func() {
		cfg, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			initErr = err
			return
		}

		cfg.MaxConns = 50
		cfg.MinConns = 5
		cfg.MaxConnLifetime = time.Hour
		cfg.MaxConnIdleTime = 5 * time.Minute
		cfg.ConnConfig.ConnectTimeout = 5 * time.Second

		pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			initErr = err
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			initErr = err
			return
		}

		queries = migrations.New(pool)
		log.Println("Database pool initialized successfully")
	})

	return initErr
}

func GetDB() (*migrations.Queries, error) {
	if pool == nil {
		return nil, errors.New("database not initialized")
	}
	return queries, nil
}

func GetPool() (*pgxpool.Pool, error) {
	if pool == nil {
		return nil, errors.New("database pool not initialized")
	}
	return pool, nil
}

func CloseDB() {
	if pool != nil {
		pool.Close()
		pool = nil
		queries = nil
		log.Println("Database pool closed")
	}
}
