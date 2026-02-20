package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockID = 424242

type migration struct {
	Version  int
	Filename string
	SQL      string
}

// Run executes all pending database migrations from the given embed.FS.
// It uses an advisory lock to prevent concurrent execution, creates a
// schema_migrations tracking table, and bootstraps pre-existing databases
// by detecting already-applied migrations via table/column checks.
func Run(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	// Advisory lock to prevent concurrent migration runs
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("migrate: advisory lock: %w", err)
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockID) //nolint:errcheck

	// Create tracking table
	if err := createTrackingTable(ctx, conn, pool); err != nil {
		return err
	}

	// Load migrations from embedded FS
	migrations, err := loadMigrations(migrationFS)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		log.Println("migrate: no migration files found")
		return nil
	}

	// Get already-applied versions
	applied, err := getAppliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	// Bootstrap: if tracking table is empty but DB has tables, detect pre-existing state
	if len(applied) == 0 {
		exists, err := tableExists(ctx, pool, "users")
		if err != nil {
			return err
		}
		if exists {
			log.Println("migrate: detected pre-existing database, bootstrapping...")
			if err := bootstrapExistingDB(ctx, pool, migrations); err != nil {
				return err
			}
			// Re-read applied versions after bootstrap
			applied, err = getAppliedVersions(ctx, pool)
			if err != nil {
				return err
			}
		}
	}

	// Apply pending migrations
	var count int
	var lastVersion int
	for _, m := range migrations {
		if applied[m.Version] {
			lastVersion = m.Version
			continue
		}

		log.Printf("migrate: applying %s ...", m.Filename)

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", m.Filename, err)
		}

		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return fmt.Errorf("migrate: exec %s: %w", m.Filename, err)
		}

		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, filename) VALUES ($1, $2)",
			m.Version, m.Filename,
		); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return fmt.Errorf("migrate: record %s: %w", m.Filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", m.Filename, err)
		}

		count++
		lastVersion = m.Version
	}

	if count == 0 {
		log.Println("migrate: database is up to date")
	} else {
		log.Printf("migrate: applied %d migration(s), now at version %d", count, lastVersion)
	}

	return nil
}

func createTrackingTable(ctx context.Context, conn *pgxpool.Conn, pool *pgxpool.Pool) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			filename   VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create tracking table: %w", err)
	}
	return nil
}

var migrationFileRe = regexp.MustCompile(`^(\d+)_.+\.sql$`)

func loadMigrations(migrationFS fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: read dir: %w", err)
	}

	var migrations []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := migrationFileRe.FindStringSubmatch(e.Name())
		if matches == nil {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		data, err := fs.ReadFile(migrationFS, e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrate: read %s: %w", e.Name(), err)
		}

		migrations = append(migrations, migration{
			Version:  version,
			Filename: e.Name(),
			SQL:      string(data),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func getAppliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: query applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)",
		name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("migrate: check table %s: %w", name, err)
	}
	return exists, nil
}

func columnExists(ctx context.Context, pool *pgxpool.Pool, table, column string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2)",
		table, column,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("migrate: check column %s.%s: %w", table, column, err)
	}
	return exists, nil
}

// bootstrapExistingDB detects which migrations have already been applied
// by checking for their footprint (specific tables/columns) and records them.
func bootstrapExistingDB(ctx context.Context, pool *pgxpool.Pool, migrations []migration) error {
	// Define footprints: version → check function
	type footprint struct {
		version int
		check   func() (bool, error)
	}

	footprints := []footprint{
		{1, func() (bool, error) { return tableExists(ctx, pool, "users") }},
		{2, func() (bool, error) { return columnExists(ctx, pool, "proxy_stats", "total_requests") }},
		{3, func() (bool, error) { return tableExists(ctx, pool, "client_acl_rules") }},
		{4, func() (bool, error) { return columnExists(ctx, pool, "proxies", "registered_ip") }},
	}

	// Build a filename lookup from loaded migrations
	filenames := make(map[int]string)
	for _, m := range migrations {
		filenames[m.Version] = m.Filename
	}

	for _, fp := range footprints {
		filename, ok := filenames[fp.version]
		if !ok {
			continue // migration file not present, skip
		}

		exists, err := fp.check()
		if err != nil {
			return err
		}
		if !exists {
			continue // not yet applied
		}

		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version, filename) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			fp.version, filename,
		); err != nil {
			return fmt.Errorf("migrate: bootstrap version %d: %w", fp.version, err)
		}
		log.Printf("migrate: bootstrapped version %d (%s) — already applied", fp.version, filename)
	}

	return nil
}
