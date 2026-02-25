package db

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

const maxDBSizeBytes = 30 * 1024 * 1024 * 1024 // 30GB

func (d *DB) StartRetentionLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.enforceRetention(ctx); err != nil {
				log.Printf("Retention error: %v", err)
			}
		}
	}
}

func (d *DB) enforceRetention(ctx context.Context) error {
	var totalSize int64
	err := d.Pool.QueryRow(ctx,
		"SELECT COALESCE(pg_total_relation_size('audit_events'), 0)").Scan(&totalSize)
	if err != nil {
		return fmt.Errorf("get size: %w", err)
	}

	log.Printf("Retention check: DB size = %d MB / %d MB",
		totalSize/(1024*1024), maxDBSizeBytes/(1024*1024))

	partitions, err := d.listPartitions(ctx)
	if err != nil {
		return err
	}

	sort.Strings(partitions)

	// Drop partitions older than 90 days
	cutoff := time.Now().UTC().AddDate(0, 0, -90).Format("2006_01_02")
	for _, p := range partitions {
		dateStr := strings.TrimPrefix(p, "audit_events_")
		if dateStr < cutoff {
			log.Printf("Dropping aged-out partition: %s", p)
			if _, err := d.Pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", p)); err != nil {
				log.Printf("Failed to drop %s: %v", p, err)
			}
		}
	}

	// Recheck size after age-based drops
	err = d.Pool.QueryRow(ctx,
		"SELECT COALESCE(pg_total_relation_size('audit_events'), 0)").Scan(&totalSize)
	if err != nil {
		return err
	}

	// Drop oldest partitions until under limit (keep at least one)
	partitions, err = d.listPartitions(ctx)
	if err != nil {
		return err
	}
	sort.Strings(partitions)

	for totalSize > maxDBSizeBytes && len(partitions) > 1 {
		oldest := partitions[0]
		log.Printf("Dropping partition for size: %s (current: %d MB)", oldest, totalSize/(1024*1024))
		if _, err := d.Pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", oldest)); err != nil {
			return fmt.Errorf("drop %s: %w", oldest, err)
		}
		partitions = partitions[1:]

		err = d.Pool.QueryRow(ctx,
			"SELECT COALESCE(pg_total_relation_size('audit_events'), 0)").Scan(&totalSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) listPartitions(ctx context.Context) ([]string, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT inhrelid::regclass::text
		 FROM pg_inherits
		 WHERE inhparent = 'audit_events'::regclass
		 ORDER BY inhrelid::regclass::text`)
	if err != nil {
		return nil, fmt.Errorf("list partitions: %w", err)
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		partitions = append(partitions, name)
	}
	return partitions, nil
}
