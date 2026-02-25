package models

import (
	"encoding/json"
	"time"
)

type AuditEvent struct {
	ID             string          `json:"id" db:"id"`
	AuditID        string          `json:"audit_id" db:"audit_id"`
	Timestamp      time.Time       `json:"timestamp" db:"timestamp"`
	Username       string          `json:"username" db:"username"`
	UserGroups     []string        `json:"user_groups" db:"user_groups"`
	Verb           string          `json:"verb" db:"verb"`
	Resource       string          `json:"resource" db:"resource"`
	Subresource    string          `json:"subresource" db:"subresource"`
	Namespace      string          `json:"namespace" db:"namespace"`
	Name           string          `json:"name" db:"name"`
	APIGroup       string          `json:"api_group" db:"api_group"`
	APIVersion     string          `json:"api_version" db:"api_version"`
	RequestURI     string          `json:"request_uri" db:"request_uri"`
	SourceIPs      []string        `json:"source_ips" db:"source_ips"`
	UserAgent      string          `json:"user_agent" db:"user_agent"`
	ResponseCode   int32           `json:"response_code" db:"response_code"`
	RequestObject  json.RawMessage `json:"request_object,omitempty" db:"request_object"`
	ResponseObject json.RawMessage `json:"response_object,omitempty" db:"response_object"`
	Annotations    json.RawMessage `json:"annotations,omitempty" db:"annotations"`
}

type IngestPayload struct {
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

type EventQuery struct {
	Username     string `form:"username"`
	Verb         string `form:"verb"`
	Resource     string `form:"resource"`
	Namespace    string `form:"namespace"`
	Name         string `form:"name"`
	From         string `form:"from"`
	To           string `form:"to"`
	ResponseCode int    `form:"response_code"`
	Client          string `form:"client"`
	ExcludeResource string `form:"exclude_resource"`
	Page            int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Sort         string `form:"sort"`
}

type EventsResponse struct {
	Events     []AuditEvent `json:"events"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	TotalPages int          `json:"total_pages"`
}

type StatsResponse struct {
	TopUsers      []StatEntry `json:"top_users"`
	TopResources  []StatEntry `json:"top_resources"`
	VerbBreakdown []StatEntry `json:"verb_breakdown"`
	TotalEvents   int64       `json:"total_events"`
	DBSizeBytes   int64       `json:"db_size_bytes"`
}

type StatEntry struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}
