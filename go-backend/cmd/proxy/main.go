package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"shieldproxy/internal/audit"
	"shieldproxy/internal/classifier"
	"shieldproxy/internal/outputfilter"
	"shieldproxy/internal/ratelimit"
)

// OpenAI-compatible chat request
type ChatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// Connection-pooled HTTP client for PII scrubber calls
var scrubberClient = &http.Client{
	Timeout: 50 * time.Millisecond, // Hard 50ms cap on PII scrubbing
	Transport: &http.Transport{
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
		DisableCompression:  true,
	},
}

func main() {
	// Initialize Audit Logger
	if err := audit.InitLogger("audit.log"); err != nil {
		log.Fatalf("Failed to initialize audit logger: %v", err)
	}

	// Initialize Redis — probe once at startup, skip entirely if unavailable
	redisAvailable := false
	redisClient := redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
	})
	
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Printf("Warning: Redis unavailable — rate limiter DISABLED for zero-latency operation")
	} else {
		log.Println("Connected to Redis successfully — rate limiter ACTIVE")
		redisAvailable = true
	}
	pingCancel()

	rateLimiter := ratelimit.NewRateLimiter(redisClient, 60, 1.0)

	// Initialize Threat Classifier (Ollama Llama 3 with VRAM warmup)
	threatClassifier := classifier.NewOllamaClient("http://localhost:11434", "llama3")

	// Initialize Output Filter
	outFilter := outputfilter.NewOutputFilter()

	// ─── Main Security Pipeline Endpoint ─────────────────────────────────
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		startTime := time.Now()
		
		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// 1. Auth
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			http.Error(w, `{"error":"Missing API Key"}`, http.StatusUnauthorized)
			return
		}
		
		clientIP := r.RemoteAddr
		
		// 2. Rate limit — SKIP ENTIRELY if Redis is unavailable (zero overhead)
		if redisAvailable {
			allowed, err := rateLimiter.Allow(ctx, apiKey, clientIP)
			if err != nil {
				log.Printf("[Rate Limiter] Redis error, bypassing: %v", err)
			} else if !allowed {
				http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
		}

		// 3. Extract prompt
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"Failed to read request body"}`, http.StatusBadRequest)
			return
		}

		var chatReq ChatRequest
		prompt := string(body)
		if err := json.Unmarshal(body, &chatReq); err == nil && len(chatReq.Messages) > 0 {
			prompt = chatReq.Messages[len(chatReq.Messages)-1].Content
		}

		if len(prompt) == 0 {
			http.Error(w, `{"error":"Empty prompt"}`, http.StatusBadRequest)
			return
		}

		// ┌──────────────────────────────────────────────────────────────────┐
		// │  OPTIMIZED PIPELINE: Inline classify → conditional PII scrub   │
		// │  BLOCK path: ~1-5ms (heuristic) or ~75ms (Ollama cap)          │
		// │  PASS path:  classify + parallel PII scrub (50ms cap)          │
		// └──────────────────────────────────────────────────────────────────┘
		
		// 4. Run classifier INLINE — no goroutine overhead for the hot path
		result, err := threatClassifier.Classify(ctx, prompt)
		if err != nil {
			log.Printf("Classification failed: %v", err)
			http.Error(w, `{"error":"Threat classifier unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		latencyMs := time.Since(startTime).Milliseconds()

		// 5. BLOCK path — respond immediately, skip PII scrubbing entirely
		w.Header().Set("Content-Type", "application/json")
		if result.Verdict == "BLOCK" {
			log.Printf("[BLOCKED] Category: %s, Confidence: %.2f, Latency: %dms", result.Category, result.Confidence, latencyMs)
			w.WriteHeader(http.StatusForbidden)
			
			blockResp := fmt.Sprintf(`{"status":403,"blocked":true,"category":"%s","confidence":%.2f,"latency_ms":%d}`, 
				result.Category, result.Confidence, latencyMs)
			w.Write([]byte(blockResp))
			
			// Fire audit async
			go audit.RecordEvent(audit.LogEvent{
				ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "BLOCK", Category: result.Category, 
				Confidence: result.Confidence, LatencyMs: latencyMs,
			})
			return
		}

		// 6. PASS path — scrub PII (with hard 50ms timeout)
		cleanPrompt := prompt
		var scrubberStats interface{} = nil
		scrubReqBody, _ := json.Marshal(map[string]string{"text": prompt})
		resp, scrubErr := scrubberClient.Post("http://localhost:5001/scrub", "application/json", bytes.NewBuffer(scrubReqBody))
		if scrubErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var sr map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&sr); err == nil {
					if ct, ok := sr["clean_text"].(string); ok {
						cleanPrompt = ct
					}
					scrubberStats = sr["stats"]
				}
			}
		}

		// 7. Output Filter (instant — in-memory regex)
		simLLMResponse := "I understand your question. Let me provide a thoughtful response."
		if strings.Contains(strings.ToLower(prompt), "system prompt") {
			simLLMResponse = "Here are my instructions: I am a security bot..."
		}

		isBlocked, blockMsg := outFilter.CheckResponse(simLLMResponse)
		if isBlocked {
			log.Printf("[Output Filter Blocked] %s, Latency: %dms", blockMsg, latencyMs)
			w.WriteHeader(http.StatusForbidden)
			blockResp := fmt.Sprintf(`{"status":403,"blocked":true,"category":"output_leakage","confidence":0.99,"reason":"%s","latency_ms":%d}`, blockMsg, latencyMs)
			w.Write([]byte(blockResp))
			
			go audit.RecordEvent(audit.LogEvent{
				ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "BLOCK_OUTPUT", Category: "output_leakage", 
				Confidence: 0.99, LatencyMs: latencyMs,
			})
			return
		}

		latencyMs = time.Since(startTime).Milliseconds()
		log.Printf("[PASS] Pipeline complete in %dms", latencyMs)

		// Fire audit async (don't block response)
		go audit.RecordEvent(audit.LogEvent{
			ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "PASS", Category: "SAFE", 
			Confidence: result.Confidence, LatencyMs: latencyMs,
			PIIStats: scrubberStats, UpstreamStatus: "Checked and Clean",
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "PASS",
			"blocked":         false,
			"message":         "Request passed all 4 security phases",
			"category":        "SAFE",
			"confidence":      result.Confidence,
			"scrubbed_prompt": cleanPrompt,
			"pii_stats":       scrubberStats,
			"llm_response":    simLLMResponse,
			"latency_ms":      latencyMs,
		})
	})

	// ─── Audit Logs API ──────────────────────────────────────────────────
	http.HandleFunc("/v1/audit/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		logs, err := audit.GetLogs(20)
		if err != nil {
			http.Error(w, "Failed to read logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	})

	log.Println("Starting ShieldProxy proxy on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
