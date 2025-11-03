package postgres

import (
    "context"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

const graphName = "rbc_graph"

func escapeCypherString(s string) string { return strings.ReplaceAll(s, "'", "''") }

// EnsureTaskVertex creates or merges a Task vertex in the AGE graph with minimal properties.
func EnsureTaskVertex(ctx context.Context, db *pgxpool.Pool, id, variant, command string) error {
    if strings.TrimSpace(id) == "" { return nil }
    variant = escapeCypherString(variant)
    command = escapeCypherString(command)
    q := fmt.Sprintf(`SELECT * FROM cypher('%s', $$
        MERGE (t:Task {id: '%s'})
        SET t.variant = '%s', t.command = '%s'
    $$) as (v agtype)`, graphName, escapeCypherString(id), variant, command)
    _, err := db.Exec(ctx, q)
    return err
}

// CreateTaskReplacesEdge creates a REPLACES edge from newTaskID to oldTaskID with properties.
// level should be one of: patch, minor, major. comment and createdISO are optional.
func CreateTaskReplacesEdge(ctx context.Context, db *pgxpool.Pool, newTaskID, oldTaskID, level, comment, createdISO string) error {
    if strings.TrimSpace(newTaskID) == "" || strings.TrimSpace(oldTaskID) == "" { return fmt.Errorf("task ids required") }
    lvl := strings.ToLower(strings.TrimSpace(level))
    switch lvl {
    case "patch", "minor", "major":
    default:
        lvl = "minor"
    }
    props := fmt.Sprintf("level: '%s'", escapeCypherString(lvl))
    if strings.TrimSpace(comment) != "" { props += ", comment: '" + escapeCypherString(comment) + "'" }
    if strings.TrimSpace(createdISO) != "" { props += ", created: '" + escapeCypherString(createdISO) + "'" }
    q := fmt.Sprintf(`SELECT * FROM cypher('%s', $$
        MERGE (n:Task {id: '%s'})
        MERGE (o:Task {id: '%s'})
        CREATE (n)-[:REPLACES {%s}]->(o)
    $$) as (v agtype)`, graphName, escapeCypherString(newTaskID), escapeCypherString(oldTaskID), props)
    _, err := db.Exec(ctx, q)
    return err
}

