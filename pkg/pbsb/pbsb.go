package pbsb

import (
	"context"
	"crypto/tls"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() (*redis.Client, error) {
	host := os.Getenv("PUB_SUB_HOST")
	password := os.Getenv("PUB_SUB_PASSWORD")

	if host == "" || password == "" {
		log.Fatal("FATAL: Missing PUB_SUB_HOST or PUB_SUB_PASSWORD environment variables.")
	}

	opts := &redis.Options{
		Addr:     host,
		Password: password,
		DB:       0,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// ServerName: strings.Split(host, ":")[0], Helps with certificate validation if needed
		},
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("ERROR: Failed to connect to Redis at %s: %v", host, err)
		return nil, err
	}

	log.Println("Successfully connected to Redis Pub/Sub.")
	return rdb, nil
}
