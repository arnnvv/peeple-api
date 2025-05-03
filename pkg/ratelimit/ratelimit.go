package ratelimit

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/go-redis/redis_rate/v10"
)

func RateLimiterMiddleware(limiter *redis_rate.Limiter, rps int, burst int) func(http.Handler) http.Handler {
	limit := redis_rate.PerSecond(rps)
	limit.Burst = burst

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			key := ""

			claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
			if ok && claims != nil && claims.UserID > 0 {
				key = fmt.Sprintf("http_user:%d", claims.UserID)
			} else {
				ip, err := getClientIP(r)
				if err != nil {
					log.Printf("WARN: RateLimiterMiddleware: Failed to get client IP: %v", err)
					key = "http_ip_unknown"
				} else {
					key = fmt.Sprintf("http_ip:%s", ip)
				}
			}

			res, err := limiter.Allow(ctx, key, limit)
			if err != nil {
				log.Printf("ERROR: RateLimiterMiddleware: Redis rate limiter check failed for key '%s': %v", key, err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit.Burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(res.ResetAfter).Unix(), 10))

			if res.Allowed == 0 {
				retryAfter := int(res.RetryAfter.Seconds())
				retryAfter = max(retryAfter, 1)
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				log.Printf("WARN: Rate limit exceeded for key '%s'", key)
				utils.RespondWithError(w, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) (string, error) {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		ip := net.ParseIP(clientIP)
		if ip != nil {
			return clientIP, nil
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		ip := net.ParseIP(xri)
		if ip != nil {
			return xri, nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		parsedIP := net.ParseIP(r.RemoteAddr)
		if parsedIP != nil {
			return r.RemoteAddr, nil
		}
		return "", fmt.Errorf("cannot parse remote address: %w", err)
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP format in remote address: %s", ip)
	}
	return ip, nil
}
