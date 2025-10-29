package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSchema creates the required tables if they do not exist.
func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
    stmts := []string{
        // Roles table for user roles/modes (name as unique identifier)
        `CREATE TABLE IF NOT EXISTS roles (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags TEXT[] DEFAULT '{}'
        )`,
        // Workflows table (name as unique identifier) with created/updated timestamps and notes (markdown)
        `CREATE TABLE IF NOT EXISTS workflows (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT
        )`,
        // Conversations table (id autoincrement) with created/updated timestamps, notes and tags
        `CREATE TABLE IF NOT EXISTS conversations (
            id BIGSERIAL PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            project TEXT,
            tags TEXT[] DEFAULT '{}',
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
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'roles_set_updated'
            ) THEN
                CREATE TRIGGER roles_set_updated
                BEFORE UPDATE ON roles
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'conversations_set_updated'
            ) THEN
                CREATE TRIGGER conversations_set_updated
                BEFORE UPDATE ON conversations
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_conversations_project ON conversations(project)`,
        // Experiments table (auto id) linked to conversations
        `CREATE TABLE IF NOT EXISTS experiments (
            id BIGSERIAL PRIMARY KEY,
            conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
            created TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        `CREATE INDEX IF NOT EXISTS idx_experiments_conversation ON experiments(conversation_id)`,
        // Task variants registry: one workflow per selector (variant)
        `CREATE TABLE IF NOT EXISTS task_variants (
            variant TEXT PRIMARY KEY,
            workflow_id TEXT NOT NULL REFERENCES workflows(name) ON DELETE CASCADE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_task_variants_workflow ON task_variants(workflow_id)`,
        // Tasks table: versioned execution units identified by (variant, version)
        `CREATE TABLE IF NOT EXISTS tasks (
            id BIGSERIAL PRIMARY KEY,
            command TEXT NOT NULL,
            variant TEXT NOT NULL,
            title TEXT,
            description TEXT,
            motivation TEXT,
            version TEXT NOT NULL CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z\.-]+)?(\+[0-9A-Za-z\.-]+)?$'),
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            shell TEXT,
            run TEXT,
            timeout INTERVAL,
            tags TEXT[] DEFAULT '{}',
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            UNIQUE (variant, version),
            FOREIGN KEY (variant) REFERENCES task_variants(variant) ON DELETE CASCADE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_tasks_variant ON tasks(variant)`,
        // Content table for message bodies (text + optional parsed JSON)
        `CREATE TABLE IF NOT EXISTS messages_content (
            id BIGSERIAL PRIMARY KEY,
            text_content TEXT NOT NULL,
            json_content JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Messages table: references tasks.id (optional), experiments.id (optional), content id
        `CREATE TABLE IF NOT EXISTS messages (
            id BIGSERIAL PRIMARY KEY,
            content_id BIGINT NOT NULL REFERENCES messages_content(id) ON DELETE CASCADE,
            task_id BIGINT REFERENCES tasks(id) ON DELETE SET NULL,
            experiment_id BIGINT REFERENCES experiments(id) ON DELETE SET NULL,
            executor TEXT,
            received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            processed_at TIMESTAMPTZ,
            status TEXT NOT NULL DEFAULT 'ingested',
            error_message TEXT,
            tags TEXT[] DEFAULT '{}',
            meta JSONB DEFAULT '{}',
            UNIQUE (content_id, status, received_at)
        )`,
        // Starred tasks per role: binds a role (e.g., user, admin) to a specific
        // variant and version, referencing the task row. Unique per (role, variant).
        `CREATE TABLE IF NOT EXISTS starred_tasks (
            id BIGSERIAL PRIMARY KEY,
            role TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
            variant TEXT NOT NULL,
            version TEXT NOT NULL,
            task_id BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (role, variant)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_starred_tasks_role ON starred_tasks(role)`,
        `CREATE INDEX IF NOT EXISTS idx_starred_tasks_variant ON starred_tasks(variant)`,
        // Ensure FK exists if table predated the FK addition
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint WHERE conname='starred_tasks_role_fkey'
            ) THEN
                ALTER TABLE starred_tasks
                    ADD CONSTRAINT starred_tasks_role_fkey
                    FOREIGN KEY (role) REFERENCES roles(name) ON DELETE CASCADE;
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'starred_tasks_set_updated'
            ) THEN
                CREATE TRIGGER starred_tasks_set_updated
                BEFORE UPDATE ON starred_tasks
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
    }
    for _, s := range stmts {
        if _, err := db.Exec(ctx, s); err != nil {
            return err
        }
    }
    return nil
}
