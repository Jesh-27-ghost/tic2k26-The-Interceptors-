package audit

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// LogEvent represents a single request passing through the ShieldProxy firewall
type LogEvent struct {
	Timestamp      string      `json:"timestamp"`
	ClientIP       string      `json:"client_ip"`
	APIKey         string      `json:"api_key"`
	Verdict        string      `json:"verdict"`  // "BLOCK" or "PASS"
	Category       string      `json:"category"`
	Confidence     float64     `json:"confidence"`
	LatencyMs      int64       `json:"latency_ms"`
	PIIStats       interface{} `json:"pii_stats,omitempty"`
	UpstreamStatus string      `json:"upstream_status,omitempty"`
}

var (
	logger *log.Logger
	mu     sync.Mutex
)

// InitLogger initializes the JSON file audit logger.
func InitLogger(filepath string) error {
	mu.Lock()
	defer mu.Unlock()

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	logger = log.New(file, "", 0) // No prefix, no default timestamp (we provide it in JSON)
	return nil
}

// RecordEvent serializes and safely writes a LogEvent to the audit log.
func RecordEvent(event LogEvent) {
	if logger == nil {
		log.Println("[Warning] Audit logger not initialized. Skipping event recording.")
		return
	}

	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Error] Failed to marshal audit log event: %v\n", err)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	logger.Println(string(data))
}
