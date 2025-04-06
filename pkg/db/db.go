package db

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetDB() *migrations.Queries {
	dbConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	dbConfig.MaxConns = 50
	dbConfig.MinConns = 10
	dbConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	return migrations.New(pool)
}
