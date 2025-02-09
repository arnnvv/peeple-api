package dbpackage

import (
	"context"
	"log"
	"os"
	"time"

	db "github.com/arnnvv/peeple-api/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetDB() *db.Queries {
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
	return db.New(pool)
}
