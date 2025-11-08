package postgres

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

const graphName = "rbc_graph"

func escapeCypherString(s string) string { return strings.ReplaceAll(s, "'", "''") }

// EnsureStickieGraphSchema creates required vertex/edge labels if missing.
func EnsureStickieGraphSchema(ctx context.Context, db *pgxpool.Pool) {
    // Best-effort: ignore errors if AGE unavailable or labels already exist
    stmts := []string{
        fmt.Sprintf("SELECT ag_catalog.create_vlabel('%s','Stickie')", graphName),
        fmt.Sprintf("SELECT ag_catalog.create_elabel('%s','INCLUDES')", graphName),
        fmt.Sprintf("SELECT ag_catalog.create_elabel('%s','CAUSES')", graphName),
        fmt.Sprintf("SELECT ag_catalog.create_elabel('%s','USES')", graphName),
        fmt.Sprintf("SELECT ag_catalog.create_elabel('%s','REPRESENTS')", graphName),
        fmt.Sprintf("SELECT ag_catalog.create_elabel('%s','CONTRASTS_WITH')", graphName),
    }
    for _, q := range stmts {
        if _, err := db.Exec(ctx, q); err != nil {
            _ = err // ignore
        }
    }
}

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

// Stickie graph helpers

func EnsureStickieVertex(ctx context.Context, db *pgxpool.Pool, id string) error {
    if strings.TrimSpace(id) == "" { return nil }
    q := fmt.Sprintf(`SELECT * FROM ag_catalog.cypher('%s', $$
        MERGE (s:Stickie {id: '%s'})
    $$) as (v ag_catalog.agtype)`, graphName, escapeCypherString(id))
    _, err := db.Exec(ctx, q)
    if err != nil && (strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied")) {
        return nil
    }
    return err
}

func normalizeStickieRelType(t string) string {
    switch strings.ToLower(strings.TrimSpace(t)) {
    case "includes": return "INCLUDES"
    case "causes": return "CAUSES"
    case "uses": return "USES"
    case "represents": return "REPRESENTS"
    case "contrasts_with", "contrasts-with", "contrastswith": return "CONTRASTS_WITH"
    default: return ""
    }
}

// CreateStickieEdge creates or overwrites labels on an edge of the given type between two stickies.
func CreateStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string, labels []string) error {
    EnsureStickieGraphSchema(ctx, db)
    rt := normalizeStickieRelType(relType)
    if rt == "" { return fmt.Errorf("invalid relation type: %s", relType) }
    if strings.TrimSpace(fromID) == "" || strings.TrimSpace(toID) == "" { return fmt.Errorf("from/to ids required") }
    // build labels array literal
    arr := "[]"
    if len(labels) > 0 {
        parts := make([]string, 0, len(labels))
        for _, v := range labels { parts = append(parts, fmt.Sprintf("'%s'", escapeCypherString(v))) }
        arr = "[" + strings.Join(parts, ",") + "]"
    }
    q := fmt.Sprintf(`SELECT * FROM ag_catalog.cypher('%s', $$
        MERGE (a:Stickie {id: '%s'})
        MERGE (b:Stickie {id: '%s'})
        MERGE (a)-[r:%s]->(b)
        SET r.labels = %s
    $$) as (v ag_catalog.agtype)`, graphName, escapeCypherString(fromID), escapeCypherString(toID), rt, arr)
    _, err := db.Exec(ctx, q)
    if err != nil && (strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied")) {
        return nil
    }
    return err
}

type StickieEdge struct {
    FromID string
    ToID   string
    Type   string
    Labels []string
}

// ListStickieEdges lists edges touching a node id with optional direction and type filter.
// dir: out|in|both
func ListStickieEdges(ctx context.Context, db *pgxpool.Pool, id, dir string, relTypes []string) ([]StickieEdge, error) {
    EnsureStickieGraphSchema(ctx, db)
    if strings.TrimSpace(id) == "" { return nil, nil }
    rtFilter := ""
    if len(relTypes) > 0 {
        ups := make([]string, 0, len(relTypes))
        for _, t := range relTypes { if v := normalizeStickieRelType(t); v != "" { ups = append(ups, "'"+v+"'") } }
        if len(ups) > 0 { rtFilter = " WHERE type(r) IN (" + strings.Join(ups, ",") + ")" }
    }
    pattern := "(a:Stickie {id: '%s'})-[r]->(b:Stickie)"
    if d := strings.ToLower(strings.TrimSpace(dir)); d == "in" {
        pattern = "(a:Stickie)-[r]->(b:Stickie {id: '%s'})"
    } else if d == "both" {
        pattern = "(a:Stickie)-[r]-(b:Stickie {id: '%s'})"
    }
    q := fmt.Sprintf(`SELECT fr::text, typ::text, lab::text, toid::text FROM ag_catalog.cypher('%s', $$
        MATCH %s
        %s
        RETURN a.id, type(r), r.labels, b.id
    $$) as (fr ag_catalog.agtype, typ ag_catalog.agtype, lab ag_catalog.agtype, toid ag_catalog.agtype)`, graphName, fmt.Sprintf(pattern, escapeCypherString(id)), rtFilter)
    rows, err := db.Query(ctx, q)
    if err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return nil, nil
        }
        return nil, err
    }
    defer rows.Close()
    var out []StickieEdge
    for rows.Next() {
        var fr, typ, lab, to string
        if err := rows.Scan(&fr, &typ, &lab, &to); err != nil { return nil, err }
        fr = strings.Trim(fr, `"`)
        typ = strings.Trim(typ, `"`)
        to = strings.Trim(to, `"`)
        // lab may be a JSON-like string; try to parse into []string
        labels := []string{}
        l := strings.TrimSpace(lab)
        if len(l) > 0 {
            // trim outer quotes if present
            if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") { l = strings.Trim(l, "\"") }
            // basic safety: must start with [
            if strings.HasPrefix(l, "[") {
                var arr []string
                if err := json.Unmarshal([]byte(l), &arr); err == nil { labels = arr }
            }
        }
        // direction normalization: the query returns a,b matched as written; adjust From/To for 'in'
        if strings.ToLower(strings.TrimSpace(dir)) == "in" {
            out = append(out, StickieEdge{FromID: to, ToID: fr, Type: typ, Labels: labels})
        } else {
            out = append(out, StickieEdge{FromID: fr, ToID: to, Type: typ, Labels: labels})
        }
    }
    return out, rows.Err()
}

// GetStickieEdge returns a single relation between exact from/to/type.
func GetStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (*StickieEdge, error) {
    EnsureStickieGraphSchema(ctx, db)
    rt := normalizeStickieRelType(relType)
    if rt == "" { return nil, fmt.Errorf("invalid relation type: %s", relType) }
    q := fmt.Sprintf(`SELECT fr::text, lab::text FROM ag_catalog.cypher('%s', $$
        MATCH (a:Stickie {id: '%s'})-[r:%s]->(b:Stickie {id: '%s'})
        RETURN a.id, r.labels
        LIMIT 1
    $$) as (fr ag_catalog.agtype, lab ag_catalog.agtype)`, graphName, escapeCypherString(fromID), rt, escapeCypherString(toID))
    var fr, lab string
    if err := db.QueryRow(ctx, q).Scan(&fr, &lab); err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return nil, nil
        }
        return nil, err
    }
    labels := []string{}
    l := strings.TrimSpace(lab)
    if len(l) > 0 {
        if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") { l = strings.Trim(l, "\"") }
        if strings.HasPrefix(l, "[") { _ = json.Unmarshal([]byte(l), &labels) }
    }
    return &StickieEdge{FromID: fromID, ToID: toID, Type: rt, Labels: labels}, nil
}

// DeleteStickieEdge deletes edges of a type between two stickies and returns affected count.
func DeleteStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (int64, error) {
    EnsureStickieGraphSchema(ctx, db)
    rt := normalizeStickieRelType(relType)
    if rt == "" { return 0, fmt.Errorf("invalid relation type: %s", relType) }
    q := fmt.Sprintf(`SELECT cnt::text FROM ag_catalog.cypher('%s', $$
        MATCH (a:Stickie {id: '%s'})-[r:%s]->(b:Stickie {id: '%s'})
        WITH r
        DELETE r
        RETURN 1
    $$) as (cnt ag_catalog.agtype)`, graphName, escapeCypherString(fromID), rt, escapeCypherString(toID))
    var ag string
    if err := db.QueryRow(ctx, q).Scan(&ag); err != nil {
        if strings.Contains(err.Error(), "ag_catalog") || strings.Contains(err.Error(), "cypher") || strings.Contains(err.Error(), "permission denied") {
            return 0, nil
        }
        return 0, err
    }
    return 1, nil
}
