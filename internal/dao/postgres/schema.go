package postgres

import (
    "context"
    "database/sql"
)

// EnsureSchema creates the required tables if they do not exist.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
    stmts := []string{
        `CREATE TABLE IF NOT EXISTS messages_events (
            id BIGSERIAL PRIMARY KEY,
            content_id TEXT NOT NULL,
            conversation_id TEXT NOT NULL,
            attempt_id TEXT NOT NULL,
            profile_name TEXT NOT NULL,
            title TEXT,
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            sender_id TEXT,
            recipients TEXT[] DEFAULT '{}',
            description TEXT,
            goal TEXT,
            timeout INTERVAL,
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
        `CREATE TABLE IF NOT EXISTS message_profiles (
            id BIGSERIAL PRIMARY KEY,
            name TEXT UNIQUE NOT NULL,
            description TEXT,
            goal TEXT,
            tags TEXT[] DEFAULT '{}',
            is_vector BOOLEAN DEFAULT FALSE,
            timeout INTERVAL,
            sensitive BOOLEAN DEFAULT FALSE,
            title TEXT,
            level TEXT CHECK (level IN ('h1','h2','h3','h4','h5','h6') OR level IS NULL),
            sender_id TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        // Optional trigger to maintain updated_at; simple for now
        `CREATE OR REPLACE FUNCTION set_updated_at()
         RETURNS TRIGGER AS $$
         BEGIN
            NEW.updated_at = now();
            RETURN NEW;
         END;
         $$ LANGUAGE plpgsql;`,
        `DO $$ BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_trigger WHERE tgname = 'message_profiles_set_updated_at'
            ) THEN
                CREATE TRIGGER message_profiles_set_updated_at
                BEFORE UPDATE ON message_profiles
                FOR EACH ROW
                EXECUTE PROCEDURE set_updated_at();
            END IF;
        END $$;`,
    }
    for _, s := range stmts {
        if _, err := db.ExecContext(ctx, s); err != nil {
            return err
        }
    }
    return nil
}

