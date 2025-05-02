package pbsb

import (
	"crypto/tls"
	"os"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:      os.Getenv("PUB_SUB_HOST"),
		Username:  os.Getenv("PUB_SUB_USERNAME"),
		Password:  os.Getenv("PUB_SUB_PASSWORD"),
		TLSConfig: &tls.Config{},
	})
}
