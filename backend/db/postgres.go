package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(dsn string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	log.Println("Connected to PostgreSQL")

	db := &DB{Pool: pool}
	if err := db.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (d *DB) migrate(ctx context.Context) error {
	var exists bool
	err := d.Pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM pg_class WHERE relname = 'audit_events'
		)`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check table: %w", err)
	}

	if exists {
		log.Println("Database schema already exists, running incremental migrations")
		return d.migrateIncremental(ctx)
	}

	schema := `
	CREATE TABLE audit_events (
		id              UUID DEFAULT gen_random_uuid(),
		audit_id        VARCHAR(255) NOT NULL,
		timestamp       TIMESTAMPTZ NOT NULL,
		username        VARCHAR(255) NOT NULL,
		user_groups     TEXT[],
		verb            VARCHAR(50) NOT NULL,
		resource        VARCHAR(255),
		subresource     VARCHAR(255),
		namespace       VARCHAR(255),
		name            VARCHAR(512),
		api_group       VARCHAR(255),
		api_version     VARCHAR(50),
		request_uri     TEXT,
		source_ips      INET[],
		user_agent      TEXT,
		response_code   INTEGER,
		request_object  JSONB,
		response_object JSONB,
		annotations     JSONB,
		PRIMARY KEY (id, timestamp)
	) PARTITION BY RANGE (timestamp);

	CREATE INDEX idx_audit_timestamp ON audit_events (timestamp DESC);
	CREATE INDEX idx_audit_username ON audit_events (username);
	CREATE INDEX idx_audit_verb ON audit_events (verb);
	CREATE INDEX idx_audit_resource ON audit_events (resource);
	CREATE INDEX idx_audit_namespace ON audit_events (namespace);
	CREATE INDEX idx_audit_name ON audit_events (name);
	CREATE INDEX idx_audit_response_code ON audit_events (response_code);
	CREATE INDEX idx_audit_user_time ON audit_events (username, timestamp DESC);
	`

	_, err = d.Pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	log.Println("Database schema created")

	return d.migrateIncremental(ctx)
}

// migrateIncremental applies additive migrations that are safe to re-run.
func (d *DB) migrateIncremental(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	if err != nil {
		log.Printf("pg_trgm extension not available (non-fatal): %v", err)
	} else {
		trgmIndexes := []string{
			`CREATE INDEX IF NOT EXISTS idx_audit_username_trgm ON audit_events USING gin (username gin_trgm_ops)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_name_trgm ON audit_events USING gin (name gin_trgm_ops)`,
		}
		for _, ddl := range trgmIndexes {
			if _, err := d.Pool.Exec(ctx, ddl); err != nil {
				log.Printf("GIN index creation (non-fatal): %v", err)
			}
		}
		log.Println("pg_trgm GIN indexes ensured")
	}
	return nil
}

func (d *DB) Close() {
	d.Pool.Close()
}

func (d *DB) EnsurePartition(ctx context.Context, t time.Time) error {
	date := t.UTC().Truncate(24 * time.Hour)
	partName := fmt.Sprintf("audit_events_%s", date.Format("2006_01_02"))
	nextDay := date.Add(24 * time.Hour)

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_events FOR VALUES FROM ('%s') TO ('%s')`,
		partName,
		date.Format("2006-01-02"),
		nextDay.Format("2006-01-02"),
	)

	_, err := d.Pool.Exec(ctx, query)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P07" {
			return nil // partition already exists (race condition with another backend)
		}
		return fmt.Errorf("create partition %s: %w", partName, err)
	}
	return nil
}
