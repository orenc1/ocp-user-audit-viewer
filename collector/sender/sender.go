package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type AuditEventPayload struct {
	AuditID        string            `json:"audit_id"`
	Timestamp      time.Time         `json:"timestamp"`
	Username       string            `json:"username"`
	UserGroups     []string          `json:"user_groups"`
	Verb           string            `json:"verb"`
	Resource       string            `json:"resource"`
	Subresource    string            `json:"subresource"`
	Namespace      string            `json:"namespace"`
	Name           string            `json:"name"`
	APIGroup       string            `json:"api_group"`
	APIVersion     string            `json:"api_version"`
	RequestURI     string            `json:"request_uri"`
	SourceIPs      []string          `json:"source_ips"`
	UserAgent      string            `json:"user_agent"`
	ResponseCode   int32             `json:"response_code"`
	RequestObject  json.RawMessage   `json:"request_object,omitempty"`
	ResponseObject json.RawMessage   `json:"response_object,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
}

type BatchSender struct {
	backendURL string
	batchSize  int
	flushEvery time.Duration
	client     *http.Client
	inCh       chan *AuditEventPayload
	token      string
}

func New(backendURL string, batchSize int, flushEvery time.Duration, inCh chan *AuditEventPayload) *BatchSender {
	token, _ := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return &BatchSender{
		backendURL: backendURL,
		batchSize:  batchSize,
		flushEvery: flushEvery,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		inCh:  inCh,
		token: string(token),
	}
}

func (s *BatchSender) Run() {
	batch := make([]*AuditEventPayload, 0, s.batchSize)
	ticker := time.NewTicker(s.flushEvery)
	defer ticker.Stop()

	for {
		select {
		case event := <-s.inCh:
			batch = append(batch, event)
			if len(batch) >= s.batchSize {
				s.flush(batch)
				batch = make([]*AuditEventPayload, 0, s.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = make([]*AuditEventPayload, 0, s.batchSize)
			}
		}
	}
}

func (s *BatchSender) flush(batch []*AuditEventPayload) {
	data, err := json.Marshal(batch)
	if err != nil {
		log.Printf("Failed to marshal batch: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/ingest", s.backendURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("Failed to send batch of %d events: %v", len(batch), err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Backend returned %d for batch of %d events", resp.StatusCode, len(batch))
		return
	}

	log.Printf("Successfully sent batch of %d events", len(batch))
}
