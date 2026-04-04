package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
	"shieldproxy/internal/classifier"
	"shieldproxy/internal/ratelimit"
)

// Simplified OpenAI-like chat request for extracting the prompt
type ChatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func main() {
	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	// Token bucket: 60 requests/minute per API key (capacity 60, refill rate 1.0 tokens/sec)
	rateLimiter := ratelimit.NewRateLimiter(redisClient, 60, 1.0)

	// Initialize Threat Classifier (Ollama Llama 3)
	threatClassifier := classifier.NewOllamaClient("http://localhost:11434", "llama3")

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// CORS Headers for React Frontend Simulator
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
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
			log.Printf("[Dev Warning] Rate limiter failing (%v) - Bypassing block globally to allow local LLM testing without Redis database.", err)
			allowed = true
		}

		if !allowed {
			http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// 3. Extract prompt from request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"Failed to read request body"}`, http.StatusBadRequest)
			return
		}

		var chatReq ChatRequest
		// If it parses as OpenAI schema, grab the last user message. Otherwise, use raw body.
		prompt := string(body)
		if err := json.Unmarshal(body, &chatReq); err == nil && len(chatReq.Messages) > 0 {
			prompt = chatReq.Messages[len(chatReq.Messages)-1].Content
		}

		if len(prompt) == 0 {
			http.Error(w, `{"error":"Empty prompt"}`, http.StatusBadRequest)
			return
		}

		// 4. Classify using Ollama Threat Classifier
		log.Printf("[Proxy] Forwarding prompt to classifier: %.50s...", prompt)
		result, err := threatClassifier.Classify(ctx, prompt)
		if err != nil {
			log.Printf("Classification failed: %v", err)
			// Decide to fail open or fail closed. For hackathon demo, fail closed or return error.
			http.Error(w, `{"error":"Threat classifier unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		// 5. Block or Forward
		w.Header().Set("Content-Type", "application/json")
		if result.Verdict == "BLOCK" {
			log.Printf("[Blocked] Category: %s, Confidence: %.2f", result.Category, result.Confidence)
			w.WriteHeader(http.StatusForbidden)
			
			blockResp := fmt.Sprintf(`{"status": 403, "blocked": true, "category": "%s", "confidence": %.2f}`, 
				result.Category, result.Confidence)
			w.Write([]byte(blockResp))
			return
		}

		log.Printf("[Passed Threat] Sending prompt to PII Scrubber...")
		cleanPrompt := prompt
		var scrubberStats interface{} = nil

		scrubReqBody, _ := json.Marshal(map[string]string{"text": prompt})
		scrubResp, err := http.Post("http://localhost:5001/scrub", "application/json", bytes.NewBuffer(scrubReqBody))
		
		if err == nil {
			defer scrubResp.Body.Close()
			if scrubResp.StatusCode == http.StatusOK {
				var scrubResult map[string]interface{}
				if err := json.NewDecoder(scrubResp.Body).Decode(&scrubResult); err == nil {
					if ct, ok := scrubResult["clean_text"].(string); ok {
						cleanPrompt = ct
					}
					scrubberStats = scrubResult["stats"]
				}
			} else {
				log.Printf("PII Scrubber returned %d. Using original prompt.", scrubResp.StatusCode)
			}
		} else {
			log.Printf("Failed to connect to Python PII Scrubber at :5001 - %v", err)
		}

		log.Printf("[Pipeline Complete] Final outcome generated for front-end")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "PASS",
			"blocked": false,
			"message": "Request passed rate limiter and threat classifier safely",
			"category": "SAFE",
			"confidence": result.Confidence,
			"scrubbed_prompt": cleanPrompt,
			"pii_stats": scrubberStats,
		})
	})

	log.Println("Starting ShieldProxy proxy on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
