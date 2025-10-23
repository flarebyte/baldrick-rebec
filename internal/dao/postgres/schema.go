package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSchema creates the required tables if they do not exist.
func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
    stmts := []string{
        `CREATE TABLE IF NOT EXISTS messages_events (
            id BIGSERIAL PRIMARY KEY,
            content_id TEXT NOT NULL,
            conversation_id TEXT NOT NULL,
            attempt_id TEXT NOT NULL,
            task_id BIGINT REFERENCES tasks(id),
            sender_id TEXT,
            recipients TEXT[] DEFAULT '{}',
            source TEXT NOT NULL,
            received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            processed_at TIMESTAMPTZ,
            status TEXT NOT NULL DEFAULT 'ingested',
            error_message TEXT,
            tags TEXT[] DEFAULT '{}',
            meta JSONB DEFAULT '{}',
            attempt INT DEFAULT 1,
            UNIQUE (content_id, source, received_at)
        )`,
        // Backfill: drop deprecated columns if they exist
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS profile_name`,
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS title`,
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS level`,
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS description`,
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS goal`,
        `ALTER TABLE messages_events DROP COLUMN IF EXISTS timeout`,
        `ALTER TABLE messages_events ADD COLUMN IF NOT EXISTS task_id BIGINT`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM information_schema.constraint_column_usage 
                WHERE table_name='messages_events' AND constraint_name='messages_events_task_id_fkey'
            ) THEN
                ALTER TABLE messages_events ADD CONSTRAINT messages_events_task_id_fkey 
                FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;
            END IF;
        END $$;`,
        // Workflows table (name as unique identifier) with created/updated timestamps and notes (markdown)
        `CREATE TABLE IF NOT EXISTS workflows (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT
        )`,
        // Trigger function to maintain 'updated' column on workflows
        `CREATE OR REPLACE FUNCTION set_updated()
         RETURNS TRIGGER AS $$
         BEGIN
            NEW.updated = now();
            RETURN NEW;
         END;
         $$ LANGUAGE plpgsql;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'workflows_set_updated'
            ) THEN
                CREATE TRIGGER workflows_set_updated
                BEFORE UPDATE ON workflows
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        // Tasks table: versioned execution units under a workflow
        `CREATE TABLE IF NOT EXISTS tasks (
            id BIGSERIAL UNIQUE NOT NULL,
            workflow_id TEXT NOT NULL REFERENCES workflows(name) ON DELETE CASCADE,
            name TEXT NOT NULL,
            title TEXT,
            description TEXT,
            motivation TEXT,
            version TEXT NOT NULL CHECK (version ~ '^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z\\.-]+)?(\\+[0-9A-Za-z\\.-]+)?$'),
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            shell TEXT,
            run TEXT,
            timeout INTERVAL,
            tags TEXT[] DEFAULT '{}',
            level TEXT CHECK (level IN ('h1','h2','h3') OR level IS NULL),
            PRIMARY KEY (workflow_id, name, version)
        )`,
        // Backfill columns if table existed prior to adding new fields
        `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS id BIGSERIAL`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint WHERE conname = 'tasks_id_unique'
            ) THEN
                ALTER TABLE tasks ADD CONSTRAINT tasks_id_unique UNIQUE (id);
            END IF;
        END $$;`,
        `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS timeout INTERVAL`,
        `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}'`,
        `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS level TEXT`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint WHERE conname = 'tasks_level_check'
            ) THEN
                ALTER TABLE tasks ADD CONSTRAINT tasks_level_check CHECK (level IN ('h1','h2','h3') OR level IS NULL);
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_tasks_workflow ON tasks(workflow_id)`,
    }
    for _, s := range stmts {
        if _, err := db.Exec(ctx, s); err != nil {
            return err
        }
    }
    return nil
}
