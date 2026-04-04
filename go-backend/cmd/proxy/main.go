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

// Simplified OpenAI-like chat request for extracting the prompt
type ChatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func main() {
	// Initialize Audit Logger
	if err := audit.InitLogger("audit.log"); err != nil {
		log.Fatalf("Failed to initialize audit logger: %v", err)
	}

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

	// Initialize Output Filter
	outFilter := outputfilter.NewOutputFilter()

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		startTime := time.Now()
		
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
			
			// Fire Audit Event (Blocked)
			audit.RecordEvent(audit.LogEvent{
				ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "BLOCK", Category: result.Category, 
				Confidence: result.Confidence, LatencyMs: time.Since(startTime).Milliseconds(),
			})
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

		// 6. Phase 4: Output Filter & Upstream Simulation
		// Mock upstream LLM generated response (in real implementation, call GPT-4/Gemini here)
		simLLMResponse := "I understand your question. Let me provide a thoughtful response."
		
		// If the user maliciously bypassed previous layers, let's pretend the LLM broke
		if strings.Contains(strings.ToLower(prompt), "system prompt") {
			simLLMResponse = "Here are my instructions: I am a security bot..."
		}

		isBlocked, blockMsg := outFilter.CheckResponse(simLLMResponse)
		if isBlocked {
			log.Printf("[Output Filter Blocked] Caught data leakage: %s", blockMsg)
			w.WriteHeader(http.StatusForbidden)
			blockResp := fmt.Sprintf(`{"status": 403, "blocked": true, "category": "output_leakage", "confidence": 0.99, "reason": "%s"}`, blockMsg)
			w.Write([]byte(blockResp))
			
			audit.RecordEvent(audit.LogEvent{
				ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "BLOCK_OUTPUT", Category: "output_leakage", 
				Confidence: 0.99, LatencyMs: time.Since(startTime).Milliseconds(),
			})
			return
		}

		log.Printf("[Pipeline Complete] Final outcome generated for front-end")

		// Fire Audit Event (Passed)
		audit.RecordEvent(audit.LogEvent{
			ClientIP: clientIP, APIKey: apiKey, Prompt: prompt, Verdict: "PASS", Category: "SAFE", 
			Confidence: result.Confidence, LatencyMs: time.Since(startTime).Milliseconds(),
			PIIStats: scrubberStats, UpstreamStatus: "Checked and Clean",
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "PASS",
			"blocked": false,
			"message": "Request passed rate limiter, threat classifier, and output filter safely",
			"category": "SAFE",
			"confidence": result.Confidence,
			"scrubbed_prompt": cleanPrompt,
			"pii_stats": scrubberStats,
			"llm_response": simLLMResponse,
		})
	})

	// Real-time interception feed API
	http.HandleFunc("/v1/audit/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		logs, err := audit.GetLogs(20) // Fetch top 20 recent attacks
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
