package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSchema creates the required tables if they do not exist.
func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
    stmts := []string{
        // Enable pgcrypto for gen_random_uuid()
        `CREATE EXTENSION IF NOT EXISTS pgcrypto`,
        // Roles table (name as unique identifier)
        `CREATE TABLE IF NOT EXISTS roles (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags JSONB DEFAULT '{}'::jsonb
        )`,
        // Workflows table (name as unique identifier) with created/updated timestamps and notes (markdown)
        `CREATE TABLE IF NOT EXISTS workflows (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT
        )`,
        // Tags table (name as unique identifier) similar to workflows
        `CREATE TABLE IF NOT EXISTS tags (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT
        )`,
        // Projects table (name scoped by role) with tags
        `CREATE TABLE IF NOT EXISTS projects (
            name TEXT NOT NULL,
            role_name TEXT NOT NULL DEFAULT 'user',
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            PRIMARY KEY (name, role_name)
        )`,
        // Conversations table (id UUID) with created/updated timestamps, notes and tags
        `CREATE TABLE IF NOT EXISTS conversations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title TEXT NOT NULL,
            description TEXT,
            project TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            tags JSONB DEFAULT '{}'::jsonb,
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
                SELECT 1 FROM pg_trigger WHERE tgname = 'tags_set_updated'
            ) THEN
                CREATE TRIGGER tags_set_updated
                BEFORE UPDATE ON tags
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'projects_set_updated'
            ) THEN
                CREATE TRIGGER projects_set_updated
                BEFORE UPDATE ON projects
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_projects_role_name ON projects(role_name)`,
        // Workspaces table: directories associated to a role (and optional project)
        `CREATE TABLE IF NOT EXISTS workspaces (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            description TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            project_name TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            tags JSONB DEFAULT '{}'::jsonb,
            directory TEXT NOT NULL,
            FOREIGN KEY (project_name, role_name) REFERENCES projects(name, role_name) ON DELETE SET NULL
        )`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'workspaces_set_updated'
            ) THEN
                CREATE TRIGGER workspaces_set_updated
                BEFORE UPDATE ON workspaces
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_workspaces_role_name ON workspaces(role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_workspaces_project_name ON workspaces(project_name)`,
        // Scripts content: store script body once keyed by SHA-256 (bytea) and role
        `CREATE TABLE IF NOT EXISTS scripts_content (
            id BYTEA PRIMARY KEY,
            script_content TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            role_name TEXT NOT NULL DEFAULT 'user'
        )`,
        // Scripts: metadata referencing content by hash
        `CREATE TABLE IF NOT EXISTS scripts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title TEXT NOT NULL,
            description TEXT,
            motivation TEXT,
            notes TEXT,
            script_content_id BYTEA,
            role_name TEXT NOT NULL DEFAULT 'user',
            tags JSONB DEFAULT '{}'::jsonb,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'scripts_set_updated'
            ) THEN
                CREATE TRIGGER scripts_set_updated
                BEFORE UPDATE ON scripts
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_scripts_role_name ON scripts(role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_tags_role_name ON tags(role_name)`,
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
        // Experiments table (UUID) linked to conversations
        `CREATE TABLE IF NOT EXISTS experiments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
            created TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        `CREATE INDEX IF NOT EXISTS idx_experiments_conversation ON experiments(conversation_id)`,
        // Task variants registry: one workflow per selector (variant)
        `CREATE TABLE IF NOT EXISTS task_variants (
            variant TEXT PRIMARY KEY,
            workflow_id TEXT NOT NULL REFERENCES workflows(name) ON DELETE CASCADE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_task_variants_workflow ON task_variants(workflow_id)`,
        // Tasks table: versioned execution units identified by (variant, version); UUID id
        `CREATE TABLE IF NOT EXISTS tasks (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            command TEXT NOT NULL,
            variant TEXT NOT NULL,
            title TEXT,
            description TEXT,
            motivation TEXT,
            version TEXT NOT NULL CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z\.-]+)?(\+[0-9A-Za-z\.-]+)?$'),
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            shell TEXT,
            run TEXT,
            timeout INTERVAL,
            tags JSONB DEFAULT '{}'::jsonb,
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            UNIQUE (variant, version),
            FOREIGN KEY (variant) REFERENCES task_variants(variant) ON DELETE CASCADE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_tasks_variant ON tasks(variant)`,
        // Content table for message bodies (text + optional parsed JSON); UUID id
        `CREATE TABLE IF NOT EXISTS messages_content (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            text_content TEXT NOT NULL,
            json_content JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Messages table: references tasks.id (optional), experiments.id (optional), content id
        `CREATE TABLE IF NOT EXISTS messages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            content_id UUID NOT NULL REFERENCES messages_content(id) ON DELETE CASCADE,
            task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
            experiment_id UUID REFERENCES experiments(id) ON DELETE SET NULL,
            role_name TEXT NOT NULL DEFAULT 'user',
            executor TEXT,
            received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            processed_at TIMESTAMPTZ,
            status TEXT NOT NULL DEFAULT 'ingested',
            error_message TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            UNIQUE (content_id, status, received_at)
        )`,
        // Attempt to migrate existing tags columns to JSONB if they exist as arrays
        `DO $$ BEGIN
            IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='roles' AND column_name='tags' AND data_type<>'jsonb') THEN
                ALTER TABLE roles ALTER COLUMN tags TYPE jsonb USING COALESCE(tags::jsonb,'{}'::jsonb);
            END IF;
            IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='conversations' AND column_name='tags' AND data_type<>'jsonb') THEN
                ALTER TABLE conversations ALTER COLUMN tags TYPE jsonb USING COALESCE(tags::jsonb,'{}'::jsonb);
            END IF;
            IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tasks' AND column_name='tags' AND data_type<>'jsonb') THEN
                ALTER TABLE tasks ALTER COLUMN tags TYPE jsonb USING COALESCE(tags::jsonb,'{}'::jsonb);
            END IF;
            IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='tags' AND data_type<>'jsonb') THEN
                ALTER TABLE messages ALTER COLUMN tags TYPE jsonb USING COALESCE(tags::jsonb,'{}'::jsonb);
            END IF;
        END $$;`,
        // Packages per role_name: bind a role (e.g., user, admin) to a specific
        // variant and version, referencing the task row. Unique per (role_name, variant).
        `CREATE TABLE IF NOT EXISTS packages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            role_name TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
            variant TEXT NOT NULL,
            version TEXT NOT NULL,
            task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (role_name, variant)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_packages_role_name ON packages(role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_packages_variant ON packages(variant)`,
        // Optional migration: rename old starred_tasks table/column if present.
        `DO $$ BEGIN
            IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='starred_tasks')
               AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='packages') THEN
                ALTER TABLE starred_tasks RENAME TO packages;
            END IF;
            IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='packages' AND column_name='role')
               AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='packages' AND column_name='role_name') THEN
                ALTER TABLE packages RENAME COLUMN role TO role_name;
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'packages_set_updated'
            ) THEN
                CREATE TRIGGER packages_set_updated
                BEFORE UPDATE ON packages
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
