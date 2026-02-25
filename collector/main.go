package main

import (
	"log"
	"os"
	"time"

	"github.com/ocohen/ocp-user-auditter/collector/filter"
	"github.com/ocohen/ocp-user-auditter/collector/sender"
	"github.com/ocohen/ocp-user-auditter/collector/tailer"

	audit "k8s.io/apiserver/pkg/apis/audit"
)

func main() {
	logDir := getEnv("AUDIT_LOG_DIR", "/var/log/kube-apiserver")
	offsetFile := getEnv("OFFSET_FILE", "/tmp/audit-offset")
	backendURL := getEnv("BACKEND_URL", "http://audit-backend.audit-system.svc.cluster.local:8080")
	batchSize := 100
	flushInterval := 5 * time.Second

	log.Printf("Starting audit log collector: logDir=%s backendURL=%s", logDir, backendURL)

	rawEventCh := make(chan *audit.Event, 1000)
	sendCh := make(chan *sender.AuditEventPayload, 1000)

	t := tailer.New(logDir, offsetFile, rawEventCh)
	s := sender.New(backendURL, batchSize, flushInterval, sendCh)

	go t.Run()
	go s.Run()

	for event := range rawEventCh {
		if !filter.ShouldKeepEvent(event) {
			continue
		}

		payload := &sender.AuditEventPayload{
			AuditID:    string(event.AuditID),
			Timestamp:  event.RequestReceivedTimestamp.Time,
			Username:   event.User.Username,
			UserGroups: event.User.Groups,
			Verb:       event.Verb,
			UserAgent:  event.UserAgent,
			RequestURI: event.RequestURI,
			SourceIPs:  event.SourceIPs,
		}

		if event.ObjectRef != nil {
			payload.Resource = event.ObjectRef.Resource
			payload.Subresource = event.ObjectRef.Subresource
			payload.Namespace = event.ObjectRef.Namespace
			payload.Name = event.ObjectRef.Name
			payload.APIGroup = event.ObjectRef.APIGroup
			payload.APIVersion = event.ObjectRef.APIVersion
		}

		if event.ResponseStatus != nil {
			payload.ResponseCode = event.ResponseStatus.Code
		}

		if event.RequestObject != nil {
			payload.RequestObject = event.RequestObject.Raw
		}
		if event.ResponseObject != nil {
			payload.ResponseObject = event.ResponseObject.Raw
		}

		if event.Annotations != nil {
			payload.Annotations = event.Annotations
		}

		sendCh <- payload
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
