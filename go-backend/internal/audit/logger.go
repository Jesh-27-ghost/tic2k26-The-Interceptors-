package audit

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogEvent represents a single request passing through the ShieldProxy firewall
type LogEvent struct {
	Timestamp      string      `json:"timestamp"`
	ClientIP       string      `json:"client_ip"`
	APIKey         string      `json:"api_key"`
	Prompt         string      `json:"prompt"`
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

// GetLogs returns the most recent N log events in reverse-chronological order
func GetLogs(limit int) ([]LogEvent, error) {
	mu.Lock()
	defer mu.Unlock()

	content, err := os.ReadFile("audit.log")
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEvent{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	var events []LogEvent

	// Parse backwards
	for i := len(lines) - 1; i >= 0 && len(events) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var evt LogEvent
		if err := json.Unmarshal([]byte(line), &evt); err == nil {
			events = append(events, evt)
		}
	}
	return events, nil
}
