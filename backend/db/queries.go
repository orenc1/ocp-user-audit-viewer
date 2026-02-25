package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ocohen/ocp-user-auditter/backend/models"
)

func (d *DB) InsertEvents(ctx context.Context, events []models.IngestPayload) error {
	partitionsCreated := make(map[string]bool)

	for _, e := range events {
		dateKey := e.Timestamp.UTC().Truncate(24 * time.Hour).Format("2006-01-02")
		if !partitionsCreated[dateKey] {
			if err := d.EnsurePartition(ctx, e.Timestamp); err != nil {
				return err
			}
			partitionsCreated[dateKey] = true
		}
	}

	batch := &pgx.Batch{}
	for _, e := range events {
		var annotationsJSON []byte
		if e.Annotations != nil {
			annotationsJSON, _ = json.Marshal(e.Annotations)
		}

		batch.Queue(
			`INSERT INTO audit_events
				(audit_id, timestamp, username, user_groups, verb, resource, subresource,
				 namespace, name, api_group, api_version, request_uri, source_ips,
				 user_agent, response_code, request_object, response_object, annotations)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
			e.AuditID, e.Timestamp, e.Username, e.UserGroups, e.Verb, e.Resource, e.Subresource,
			e.Namespace, e.Name, e.APIGroup, e.APIVersion, e.RequestURI, e.SourceIPs,
			e.UserAgent, e.ResponseCode, e.RequestObject, e.ResponseObject, annotationsJSON,
		)
	}

	br := d.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for range events {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch insert: %w", err)
		}
	}
	return nil
}

var allowedSortFields = map[string]bool{
	"timestamp":     true,
	"username":      true,
	"verb":          true,
	"resource":      true,
	"namespace":     true,
	"response_code": true,
}

func (d *DB) QueryEvents(ctx context.Context, q models.EventQuery) (*models.EventsResponse, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 50
	}

	where, args := buildWhereClause(q)

	countQuery := "SELECT COUNT(*) FROM audit_events" + where
	var total int64
	err := d.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	sortField := "timestamp"
	sortDir := "DESC"
	if q.Sort != "" {
		parts := strings.SplitN(q.Sort, " ", 2)
		if allowedSortFields[parts[0]] {
			sortField = parts[0]
		}
		if len(parts) > 1 && strings.EqualFold(parts[1], "ASC") {
			sortDir = "ASC"
		}
	}

	offset := (q.Page - 1) * q.PageSize
	dataQuery := fmt.Sprintf(
		`SELECT id, audit_id, timestamp, username, user_groups, verb, resource, subresource,
				namespace, name, api_group, api_version, request_uri, source_ips::TEXT[],
				user_agent, response_code, request_object, response_object, annotations
		 FROM audit_events%s ORDER BY %s %s LIMIT %d OFFSET %d`,
		where, sortField, sortDir, q.PageSize, offset,
	)

	rows, err := d.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var events []models.AuditEvent
	for rows.Next() {
		var e models.AuditEvent
		err := rows.Scan(
			&e.ID, &e.AuditID, &e.Timestamp, &e.Username, &e.UserGroups,
			&e.Verb, &e.Resource, &e.Subresource, &e.Namespace, &e.Name,
			&e.APIGroup, &e.APIVersion, &e.RequestURI, &e.SourceIPs,
			&e.UserAgent, &e.ResponseCode, &e.RequestObject, &e.ResponseObject, &e.Annotations,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		events = append(events, e)
	}

	if events == nil {
		events = []models.AuditEvent{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(q.PageSize)))

	return &models.EventsResponse{
		Events:     events,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (d *DB) GetEvent(ctx context.Context, id string) (*models.AuditEvent, error) {
	query := `SELECT id, audit_id, timestamp, username, user_groups, verb, resource, subresource,
				namespace, name, api_group, api_version, request_uri, source_ips::TEXT[],
				user_agent, response_code, request_object, response_object, annotations
			  FROM audit_events WHERE id = $1`

	var e models.AuditEvent
	err := d.Pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.AuditID, &e.Timestamp, &e.Username, &e.UserGroups,
		&e.Verb, &e.Resource, &e.Subresource, &e.Namespace, &e.Name,
		&e.APIGroup, &e.APIVersion, &e.RequestURI, &e.SourceIPs,
		&e.UserAgent, &e.ResponseCode, &e.RequestObject, &e.ResponseObject, &e.Annotations,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (d *DB) GetStats(ctx context.Context) (*models.StatsResponse, error) {
	stats := &models.StatsResponse{}

	err := d.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&stats.TotalEvents)
	if err != nil {
		return nil, err
	}

	err = d.Pool.QueryRow(ctx,
		"SELECT COALESCE(pg_total_relation_size('audit_events'), 0)").Scan(&stats.DBSizeBytes)
	if err != nil {
		return nil, err
	}

	stats.TopUsers, err = d.queryStatEntries(ctx,
		"SELECT username, COUNT(*) as cnt FROM audit_events GROUP BY username ORDER BY cnt DESC LIMIT 10")
	if err != nil {
		return nil, err
	}

	stats.TopResources, err = d.queryStatEntries(ctx,
		"SELECT resource, COUNT(*) as cnt FROM audit_events WHERE resource != '' GROUP BY resource ORDER BY cnt DESC LIMIT 10")
	if err != nil {
		return nil, err
	}

	stats.VerbBreakdown, err = d.queryStatEntries(ctx,
		"SELECT verb, COUNT(*) as cnt FROM audit_events GROUP BY verb ORDER BY cnt DESC")
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (d *DB) queryStatEntries(ctx context.Context, query string) ([]models.StatEntry, error) {
	rows, err := d.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.StatEntry
	for rows.Next() {
		var e models.StatEntry
		if err := rows.Scan(&e.Label, &e.Count); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []models.StatEntry{}
	}
	return entries, nil
}

func buildWhereClause(q models.EventQuery) (string, []any) {
	var conditions []string
	var args []any
	argIdx := 1

	if q.Username != "" {
		conditions = append(conditions, fmt.Sprintf("username ILIKE $%d", argIdx))
		args = append(args, "%"+q.Username+"%")
		argIdx++
	}
	if q.Verb != "" {
		verbs := strings.Split(q.Verb, ",")
		placeholders := make([]string, len(verbs))
		for i, v := range verbs {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, strings.TrimSpace(v))
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf("verb IN (%s)", strings.Join(placeholders, ",")))
	}
	if q.Resource != "" {
		conditions = append(conditions, fmt.Sprintf("resource = $%d", argIdx))
		args = append(args, q.Resource)
		argIdx++
	}
	if q.Namespace != "" {
		conditions = append(conditions, fmt.Sprintf("namespace = $%d", argIdx))
		args = append(args, q.Namespace)
		argIdx++
	}
	if q.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+q.Name+"%")
		argIdx++
	}
	if q.From != "" {
		t, err := time.Parse(time.RFC3339, q.From)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if q.To != "" {
		t, err := time.Parse(time.RFC3339, q.To)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if q.ResponseCode > 0 {
		conditions = append(conditions, fmt.Sprintf("response_code = $%d", argIdx))
		args = append(args, q.ResponseCode)
		argIdx++
	}
	if q.ExcludeResource != "" {
		resources := strings.Split(q.ExcludeResource, ",")
		placeholders := make([]string, len(resources))
		for i, r := range resources {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, strings.TrimSpace(r))
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf("resource NOT IN (%s)", strings.Join(placeholders, ",")))
	}
	if q.Client != "" {
		clients := strings.Split(q.Client, ",")
		clientConds := make([]string, len(clients))
		for i, c := range clients {
			clientConds[i] = fmt.Sprintf("user_agent ILIKE $%d", argIdx)
			args = append(args, "%"+strings.TrimSpace(c)+"%")
			argIdx++
		}
		conditions = append(conditions, "("+strings.Join(clientConds, " OR ")+")")
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
