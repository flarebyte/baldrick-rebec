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
    q := fmt.Sprintf(`SELECT * FROM ag_catalog.cypher('%s', $$
        MERGE (t:Task {id: '%s'})
        SET t.variant = '%s', t.command = '%s'
    $$) as (v ag_catalog.agtype)`, graphName, escapeCypherString(id), variant, command)
    _, err := db.Exec(ctx, q)
    if err != nil && (strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied")) {
        return nil
    }
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
    q := fmt.Sprintf(`SELECT * FROM ag_catalog.cypher('%s', $$
        MERGE (n:Task {id: '%s'})
        MERGE (o:Task {id: '%s'})
        CREATE (n)-[:REPLACES {%s}]->(o)
    $$) as (v ag_catalog.agtype)`, graphName, escapeCypherString(newTaskID), escapeCypherString(oldTaskID), props)
    _, err := db.Exec(ctx, q)
    if err != nil && (strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied")) {
        return nil
    }
    return err
}

// FindLatestTaskIDByVariant returns the Task ID with no incoming REPLACES for a variant.
func FindLatestTaskIDByVariant(ctx context.Context, db *pgxpool.Pool, variant string) (string, error) {
    v := escapeCypherString(variant)
    q := fmt.Sprintf(`SELECT id::text FROM ag_catalog.cypher('%s', $$
        MATCH (t:Task {variant: '%s'})
        WHERE NOT (:Task)-[:REPLACES]->(t)
        RETURN t.id
        ORDER BY t.id
        LIMIT 1
    $$) as (id ag_catalog.agtype)`, graphName, v)
    var ag string
    if err := db.QueryRow(ctx, q).Scan(&ag); err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return "", nil
        }
        return "", err
    }
    return strings.Trim(ag, `"`), nil
}

// FindNextByLevel returns the next Task ID that directly REPLACES current with given level.
func FindNextByLevel(ctx context.Context, db *pgxpool.Pool, currentID, level string) (string, error) {
    cur := escapeCypherString(currentID)
    lvl := strings.ToLower(strings.TrimSpace(level))
    if lvl != "patch" && lvl != "minor" && lvl != "major" { return "", fmt.Errorf("invalid level: %s", level) }
    q := fmt.Sprintf(`SELECT id::text FROM ag_catalog.cypher('%s', $$
        MATCH (n:Task)-[r:REPLACES]->(o:Task {id: '%s'})
        WHERE r.level = '%s'
        RETURN n.id, r.created
        ORDER BY COALESCE(r.created,'') DESC
        LIMIT 1
    $$) as (id ag_catalog.agtype, created ag_catalog.agtype)`, graphName, cur, lvl)
    var ag string
    if err := db.QueryRow(ctx, q).Scan(&ag, new(string)); err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return "", nil
        }
        return "", err
    }
    return strings.Trim(ag, `"`), nil
}

// FindLatestFrom returns the latest Task ID reachable by REPLACES edges from current.
func FindLatestFrom(ctx context.Context, db *pgxpool.Pool, currentID string) (string, error) {
    cur := escapeCypherString(currentID)
    q := fmt.Sprintf(`SELECT id::text FROM ag_catalog.cypher('%s', $$
        MATCH (n:Task), (c:Task {id: '%s'}), p=(n)-[:REPLACES*]->(c)
        WHERE NOT (:Task)-[:REPLACES]->(n)
        RETURN n.id
        LIMIT 1
    $$) as (id ag_catalog.agtype)`, graphName, cur)
    var ag string
    if err := db.QueryRow(ctx, q).Scan(&ag); err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return "", nil
        }
        return "", err
    }
    return strings.Trim(ag, `"`), nil
}
