# Database Migrations

## How It Works

The backend embeds all SQL migration files from `backend/migrations/` into the binary using Go's `//go:embed`. On startup, before the HTTP server or scheduler begin, the migrator:

1. Acquires a PostgreSQL advisory lock (`pg_advisory_lock(424242)`) to prevent concurrent runs
2. Creates the `schema_migrations` tracking table if it doesn't exist
3. Detects pre-existing databases (bootstrapping) by checking for table/column footprints
4. Applies any pending migrations in order, each wrapped in its own transaction
5. Releases the advisory lock

### Tracking Table

```sql
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    filename   VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Bootstrap (Pre-existing Databases)

If `schema_migrations` is empty but the `users` table exists, the migrator assumes the database was created by `database/schema.sql` and checks for each migration's footprint:

| Version | Footprint Check |
|---------|----------------|
| 001 | `users` table exists |
| 002 | `proxy_stats.total_requests` column exists |
| 003 | `client_acl_rules` table exists |

Detected migrations are recorded without re-executing their SQL.

## Creating a New Migration

1. Create a new SQL file in `backend/migrations/` following the naming convention:

   ```
   NNN_description.sql
   ```

   Where `NNN` is a zero-padded sequential number (e.g., `004_add_tags.sql`).

2. Write idempotent SQL using `IF NOT EXISTS` / `IF EXISTS` where possible:

   ```sql
   -- Migration 004: Add tags to configs
   ALTER TABLE configs ADD COLUMN IF NOT EXISTS tags TEXT[];
   CREATE INDEX IF NOT EXISTS idx_configs_tags ON configs USING GIN(tags);
   ```

3. If the migration adds a new table or column that needs bootstrap detection, add a footprint entry in `backend/internal/migrate/migrate.go` in the `bootstrapExistingDB` function.

4. Update `database/schema.sql` to reflect the new state (this file is used for fresh databases via Docker init).

5. Rebuild and restart:

   ```bash
   docker compose up --build backend
   ```

## Scenarios

| Scenario | What Happens |
|----------|-------------|
| **Fresh DB** (`docker compose down -v && up`) | `schema.sql` creates everything; migrator bootstraps all versions as already applied |
| **Existing DB, missing migration** | Migrator bootstraps known versions; applies only the missing one(s) |
| **Normal restart** | Migrator finds all versions applied; logs "database is up to date" |
| **New migration added** | Migrator applies only the new version; logs "applied 1 migration(s)" |

## Inspecting Migration State

```bash
# List applied migrations
docker compose exec postgres psql -U proxymanager -c \
  "SELECT * FROM schema_migrations ORDER BY version;"

# Check current schema version
docker compose exec postgres psql -U proxymanager -c \
  "SELECT MAX(version) as current_version FROM schema_migrations;"
```

## Troubleshooting

**Migration failed mid-apply**: Each migration runs in its own transaction. If one fails, the SQL is rolled back but the version is NOT recorded. Fix the SQL and restart — it will retry.

**Advisory lock stuck**: If the backend crashes while holding the lock, the lock is automatically released when the connection closes. No manual intervention needed.

**Re-running bootstrap**: Bootstrap only runs when `schema_migrations` is empty. To re-bootstrap, truncate the table:

```sql
TRUNCATE schema_migrations;
```

Then restart the backend.
