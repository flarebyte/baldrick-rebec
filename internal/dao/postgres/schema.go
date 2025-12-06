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
        // Enable pgvector (vector) extension for vector similarity (best-effort; ignore if not installed)
        `DO $$
        BEGIN
            EXECUTE 'CREATE EXTENSION IF NOT EXISTS vector';
        EXCEPTION WHEN others THEN
            NULL;
        END$$;`,
        // AGE extension intentionally not required; graph features use SQL tables now.
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
        // Tools table (name as unique identifier) with role scope, tags and settings
        `CREATE TABLE IF NOT EXISTS tools (
            name TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            description TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            settings JSONB DEFAULT '{}'::jsonb,
            tool_type TEXT
        )`,
        `CREATE INDEX IF NOT EXISTS idx_tools_role_name ON tools(role_name)`,
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
        // Tools trigger after set_updated() exists
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'tools_set_updated'
            ) THEN
                CREATE TRIGGER tools_set_updated
                BEFORE UPDATE ON tools
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
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
        // Scripts content: store script body once keyed by SHA-256 (bytea)
        `CREATE TABLE IF NOT EXISTS scripts_content (
            id BYTEA PRIMARY KEY,
            script_content TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Scripts: metadata referencing content by hash (with complex_name and archived)
        `CREATE TABLE IF NOT EXISTS scripts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title TEXT NOT NULL,
            description TEXT,
            motivation TEXT,
            notes TEXT,
            script_content_id BYTEA,
            role_name TEXT NOT NULL DEFAULT 'user',
            tags JSONB DEFAULT '{}'::jsonb,
            complex_name JSONB NOT NULL,
            archived BOOLEAN NOT NULL DEFAULT FALSE,
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
        `CREATE INDEX IF NOT EXISTS idx_scripts_complex_name ON scripts ((complex_name->>'name'), (complex_name->>'variant')) WHERE archived = FALSE`,
        `CREATE INDEX IF NOT EXISTS idx_scripts_updated ON scripts (updated DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_scripts_complex_name_gin ON scripts USING GIN (complex_name jsonb_path_ops)`,
        // Workspaces table associated to a role (and optional project)
        `CREATE TABLE IF NOT EXISTS workspaces (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            description TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            project_name TEXT,
            build_script_id UUID REFERENCES scripts(id) ON DELETE SET NULL,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            tags JSONB DEFAULT '{}'::jsonb,
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
        // Scripts content: store script body once keyed by SHA-256 (bytea)
        `CREATE TABLE IF NOT EXISTS scripts_content (
            id BYTEA PRIMARY KEY,
            script_content TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Scripts: metadata referencing content by hash (with complex_name and archived)
        `CREATE TABLE IF NOT EXISTS scripts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title TEXT NOT NULL,
            description TEXT,
            motivation TEXT,
            notes TEXT,
            script_content_id BYTEA,
            role_name TEXT NOT NULL DEFAULT 'user',
            tags JSONB DEFAULT '{}'::jsonb,
            complex_name JSONB NOT NULL,
            archived BOOLEAN NOT NULL DEFAULT FALSE,
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
        `CREATE INDEX IF NOT EXISTS idx_scripts_complex_name ON scripts ((complex_name->>'name'), (complex_name->>'variant')) WHERE archived = FALSE`,
        `CREATE INDEX IF NOT EXISTS idx_scripts_updated ON scripts (updated DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_scripts_complex_name_gin ON scripts USING GIN (complex_name jsonb_path_ops)`,
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
        // Tasks table: execution units identified by variant; UUID id
        `CREATE TABLE IF NOT EXISTS tasks (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            command TEXT NOT NULL,
            variant TEXT NOT NULL,
            title TEXT,
            description TEXT,
            motivation TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            shell TEXT,
            timeout INTERVAL,
            tool_workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL,
            tags JSONB DEFAULT '{}'::jsonb,
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            archived BOOLEAN NOT NULL DEFAULT FALSE,
            UNIQUE (variant),
            FOREIGN KEY (variant) REFERENCES task_variants(variant) ON DELETE CASCADE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_tasks_variant ON tasks(variant)`,
        // Remove legacy column if present
        `ALTER TABLE tasks DROP COLUMN IF EXISTS run_script_id`,
        // Ensure archived column exists for tasks
        `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT FALSE`,
        // Task-Script attachments: associate scripts to tasks under logical names and optional aliases
        `CREATE TABLE IF NOT EXISTS task_scripts (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
            name TEXT NOT NULL,
            alias TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        `CREATE INDEX IF NOT EXISTS idx_task_scripts_task_id ON task_scripts(task_id)`,
        `CREATE INDEX IF NOT EXISTS idx_task_scripts_script_id ON task_scripts(script_id)`,
        `CREATE INDEX IF NOT EXISTS idx_task_scripts_task_name ON task_scripts(task_id, name)`,
        `CREATE INDEX IF NOT EXISTS idx_task_scripts_task_alias ON task_scripts(task_id, alias)`,
        // Decisions: enforce unique (task_id, name) and unique (task_id, alias) when alias present
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint WHERE conname = 'task_scripts_task_name_uniq' AND conrelid = 'task_scripts'::regclass
            ) THEN
                ALTER TABLE task_scripts ADD CONSTRAINT task_scripts_task_name_uniq UNIQUE (task_id, name);
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'task_scripts_task_alias_uniq'
            ) THEN
                CREATE UNIQUE INDEX task_scripts_task_alias_uniq ON task_scripts(task_id, alias) WHERE alias IS NOT NULL;
            END IF;
        END $$;`,
        // Task replacement relations (SQL graph): new_task REPLACES old_task
        `CREATE TABLE IF NOT EXISTS task_replaces (
            new_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            old_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            level TEXT NOT NULL CHECK (level IN ('patch','minor','major')),
            comment TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY (new_task_id, old_task_id)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_task_replaces_old ON task_replaces(old_task_id)`,
        `CREATE INDEX IF NOT EXISTS idx_task_replaces_new ON task_replaces(new_task_id)`,
        // Content table for message bodies (text + optional parsed JSON); UUID id
        `CREATE TABLE IF NOT EXISTS messages_content (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            text_content TEXT NOT NULL,
            json_content JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Messages table: references tasks.id (optional) as from_task_id, experiments.id (optional), content id
        `CREATE TABLE IF NOT EXISTS messages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            content_id UUID NOT NULL REFERENCES messages_content(id) ON DELETE CASCADE,
            from_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
            experiment_id UUID REFERENCES experiments(id) ON DELETE SET NULL,
            role_name TEXT NOT NULL DEFAULT 'user',
            status TEXT NOT NULL DEFAULT 'ingested',
            error_message TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (content_id, status, created)
        )`,
        // Packages per role_name: bind a role (e.g., user, admin) to a specific
        // task (by id). Unique per (role_name, task_id).
        `CREATE TABLE IF NOT EXISTS packages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            role_name TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
            task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (role_name, task_id)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_packages_role_name ON packages(role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_packages_task ON packages(task_id)`,
        // Queue of work items
        `CREATE TABLE IF NOT EXISTS queues (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            description TEXT,
            inQueueSince TIMESTAMPTZ NOT NULL DEFAULT now(),
            status TEXT NOT NULL DEFAULT 'Waiting',
            why TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
            inbound_message UUID REFERENCES messages(id) ON DELETE SET NULL,
            target_workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL
        )`,
        `CREATE INDEX IF NOT EXISTS idx_queues_status ON queues(status)`,
        `CREATE INDEX IF NOT EXISTS idx_queues_inqueue_since ON queues(inQueueSince)`,
        // Test cases table
        `CREATE TABLE IF NOT EXISTS testcases (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name TEXT,
            package TEXT,
            classname TEXT,
            title TEXT NOT NULL,
            experiment_id UUID REFERENCES experiments(id) ON DELETE SET NULL,
            role_name TEXT NOT NULL DEFAULT 'user',
            status TEXT NOT NULL DEFAULT 'KO',
            error_message TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            file TEXT,
            line INT,
            execution_time DOUBLE PRECISION
        )`,
        `CREATE INDEX IF NOT EXISTS idx_testcases_role_name ON testcases(role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_testcases_experiment ON testcases(experiment_id)`,
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
        // Stores: generic storage for notes/ideas/etc scoped by role and name
        `CREATE TABLE IF NOT EXISTS stores (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name TEXT NOT NULL,
            title TEXT NOT NULL,
            description TEXT,
            motivation TEXT,
            security TEXT,
            privacy TEXT,
            role_name TEXT NOT NULL DEFAULT 'user',
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            store_type TEXT,
            scope TEXT CHECK (scope IN ('conversation','shared','project','task') OR scope IS NULL),
            lifecycle TEXT CHECK (lifecycle IN ('permanent','yearly','quarterly','monthly','weekly','daily') OR lifecycle IS NULL),
            UNIQUE (name, role_name)
        )`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'stores_set_updated'
            ) THEN
                CREATE TRIGGER stores_set_updated
                BEFORE UPDATE ON stores
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_stores_role_name ON stores(role_name)`,
        // Blackboards: notes tied to a store with optional links
        `CREATE TABLE IF NOT EXISTS blackboards (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
            role_name TEXT NOT NULL DEFAULT 'user',
            conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
            project_name TEXT,
            task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            background TEXT,
            guidelines TEXT,
            FOREIGN KEY (project_name, role_name) REFERENCES projects(name, role_name) ON DELETE SET NULL
        )`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'blackboards_set_updated'
            ) THEN
                CREATE TRIGGER blackboards_set_updated
                BEFORE UPDATE ON blackboards
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_blackboards_role_name ON blackboards(role_name)`,
        // Topics: role-scoped catalog of topics
        `CREATE TABLE IF NOT EXISTS topics (
            name TEXT NOT NULL,
            role_name TEXT NOT NULL DEFAULT 'user',
            title TEXT NOT NULL,
            description TEXT,
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            notes TEXT,
            tags JSONB DEFAULT '{}'::jsonb,
            PRIMARY KEY (name, role_name)
        )`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'topics_set_updated'
            ) THEN
                CREATE TRIGGER topics_set_updated
                BEFORE UPDATE ON topics
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_topics_role_name ON topics(role_name)`,
        // Stickies: notes attached to blackboards, optionally associated to topics
        `CREATE TABLE IF NOT EXISTS stickies (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            blackboard_id UUID NOT NULL REFERENCES blackboards(id) ON DELETE CASCADE,
            topic_name TEXT,
            topic_role_name TEXT,
            note TEXT,
            labels TEXT[],
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated TIMESTAMPTZ NOT NULL DEFAULT now(),
            created_by_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
            edit_count INT NOT NULL DEFAULT 0,
            priority_level TEXT CHECK (priority_level IN ('must','should','could','wont') OR priority_level IS NULL),
            score DOUBLE PRECISION,
            complex_name JSONB NOT NULL DEFAULT '{"name":"","variant":""}',
            archived BOOLEAN NOT NULL DEFAULT FALSE,
            FOREIGN KEY (topic_name, topic_role_name) REFERENCES topics(name, role_name) ON DELETE SET NULL
        )`,
        // Trigger to auto-increment edit_count on any update
        `CREATE OR REPLACE FUNCTION inc_edit_count()
         RETURNS TRIGGER AS $$
         BEGIN
            NEW.edit_count = COALESCE(OLD.edit_count,0) + 1;
            RETURN NEW;
         END;
         $$ LANGUAGE plpgsql;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'stickies_inc_edit_count'
            ) THEN
                CREATE TRIGGER stickies_inc_edit_count
                BEFORE UPDATE ON stickies
                FOR EACH ROW
                EXECUTE PROCEDURE inc_edit_count();
            END IF;
        END $$;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'stickies_set_updated'
            ) THEN
                CREATE TRIGGER stickies_set_updated
                BEFORE UPDATE ON stickies
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated();
            END IF;
        END $$;`,
        `CREATE INDEX IF NOT EXISTS idx_stickies_blackboard ON stickies(blackboard_id)`,
        `CREATE INDEX IF NOT EXISTS idx_stickies_topic ON stickies(topic_name, topic_role_name)`,
        `CREATE INDEX IF NOT EXISTS idx_stickies_complex_name ON stickies ((complex_name->>'name'), (complex_name->>'variant')) WHERE archived = FALSE`,
        `CREATE INDEX IF NOT EXISTS idx_stickies_updated ON stickies (updated DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_stickies_complex_name_gin ON stickies USING GIN (complex_name jsonb_path_ops)`,
        // Ensure new optional structured JSONB column exists for stickies
        `ALTER TABLE stickies ADD COLUMN IF NOT EXISTS structured JSONB`,
        // Ensure new optional score column exists for stickies
        `ALTER TABLE stickies ADD COLUMN IF NOT EXISTS score DOUBLE PRECISION`,
        // Fallback store for stickie relationships when AGE is unavailable
        `CREATE TABLE IF NOT EXISTS stickie_relations (
            from_id UUID NOT NULL REFERENCES stickies(id) ON DELETE CASCADE,
            to_id   UUID NOT NULL REFERENCES stickies(id) ON DELETE CASCADE,
            rel_type TEXT NOT NULL CHECK (rel_type IN ('INCLUDES','CAUSES','USES','REPRESENTS','CONTRASTS_WITH')),
            labels TEXT[] DEFAULT ARRAY[]::text[],
            created TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY (from_id, to_id, rel_type)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_stickie_relations_from ON stickie_relations(from_id)`,
        `CREATE INDEX IF NOT EXISTS idx_stickie_relations_to ON stickie_relations(to_id)`,
    }
    for _, s := range stmts {
        if _, err := db.Exec(ctx, s); err != nil {
            return err
        }
    }
    return nil
}
