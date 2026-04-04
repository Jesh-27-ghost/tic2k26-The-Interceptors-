package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
	"shieldproxy/internal/ratelimit"
)

func main() {
	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	
	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	// Token bucket: 60 requests/minute per API key (capacity 60, refill rate 1.0 tokens/sec)
	rateLimiter := ratelimit.NewRateLimiter(redisClient, 60, 1.0)

	// Basic Handler mapping Phase 2 logic scaffolding
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Auth: extract ShieldProxy API key
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			http.Error(w, `{"error":"Missing API Key"}`, http.StatusUnauthorized)
			return
		}
		
		clientIP := r.RemoteAddr
		
		// 2. Rate limit check (Redis)
		allowed, err := rateLimiter.Allow(ctx, apiKey, clientIP)
		if err != nil {
			log.Printf("Rate limiter error: %v", err)
			http.Error(w, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)
			return
		}

		if !allowed {
			http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Success path — scaffolded proxy target
		fmt.Fprintf(w, `{"status": "PASS", "message": "Request passed rate limiter successfully"}`)
	})

	log.Println("Starting ShieldProxy proxy on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
