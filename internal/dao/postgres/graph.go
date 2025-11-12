package postgres

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

// IMPORTANT: AGE parameter placeholders ($param) are not supported via ag_catalog.cypher
// in our current deployment (AGE 1.6.0 reports "a name constant is expected").
// As a result, DO NOT use parameterized Cypher here. Always build Cypher with
// safely escaped literals using escapeCypherString and call the 2-argument form:
//   SELECT * FROM ag_catalog.cypher(graph, cypher)
// SQL-side parameters to Postgres are still allowed/encouraged, but the Cypher
// string itself must not contain $param placeholders.
const graphName = "rbc_graph"

func escapeCypherString(s string) string { return strings.ReplaceAll(s, "'", "''") }

func cypherArgs(query string, params map[string]any) (string, []byte) {
    b, _ := json.Marshal(params)
    return query, b
}

// EnsureStickieGraphSchema creates required vertex/edge labels if missing.
func EnsureStickieGraphSchema(ctx context.Context, db *pgxpool.Pool) {
    // Best-effort: ignore errors if AGE unavailable or labels already exist
    labels := []string{"Stickie"}
    for _, l := range labels { _, _ = db.Exec(ctx, "SELECT ag_catalog.create_vlabel($1,$2)", graphName, l) }
    edges := []string{"INCLUDES","CAUSES","USES","REPRESENTS","CONTRASTS_WITH"}
    for _, e := range edges { _, _ = db.Exec(ctx, "SELECT ag_catalog.create_elabel($1,$2)", graphName, e) }
}

// EnsureTaskVertex creates or merges a Task vertex in the AGE graph with minimal properties.
func EnsureTaskVertex(ctx context.Context, db *pgxpool.Pool, id, variant, command string) error {
    if strings.TrimSpace(id) == "" { return nil }
    // Best-effort ensure Task label exists to reduce cryptic errors on older AGE
    _, _ = db.Exec(ctx, "SELECT ag_catalog.create_vlabel($1,$2)", graphName, "Task")
    // Workaround: AGE 1.6 in this environment rejects parameter placeholders like $id.
    // Use safely escaped literals in the Cypher string.
    // Use SET with map update to avoid parser quirks with comma-separated assignments
    cy := fmt.Sprintf(
        "MERGE (t:Task {id: '%s'}) SET t += {variant: '%s', command: '%s'}",
        escapeCypherString(id), escapeCypherString(variant), escapeCypherString(command),
    )
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2) as (v agtype)", graphName, cy)
    if err != nil {
        sum := strings.Join([]string{
            dbutil.ParamSummary("id", id),
            dbutil.ParamSummary("variant", variant),
            dbutil.ParamSummary("command", command),
        }, ",")
        // Keep cypher text generic to avoid leaking values, but clarify it's literal form
        return fmt.Errorf("AGE ensure Task vertex failed: %w; cypher(literal)=MERGE (t:Task {id: '<redacted>'}) SET t += {variant:'<redacted>',command:'<redacted>'}; %s", err, sum)
    }
    return nil
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
    cy := fmt.Sprintf(
        "MERGE (n:Task {id: '%s'}) MERGE (o:Task {id: '%s'}) CREATE (n)-[:REPLACES {level: '%s', comment: '%s', created: '%s'}]->(o)",
        escapeCypherString(newTaskID), escapeCypherString(oldTaskID), escapeCypherString(lvl), escapeCypherString(comment), escapeCypherString(createdISO),
    )
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2) as (v agtype)", graphName, cy)
    if err != nil {
        sum := strings.Join([]string{
            dbutil.ParamSummary("new", newTaskID),
            dbutil.ParamSummary("old", oldTaskID),
            dbutil.ParamSummary("level", lvl),
            dbutil.ParamSummary("comment", comment),
            dbutil.ParamSummary("created", createdISO),
        }, ",")
        return fmt.Errorf("AGE create REPLACES failed: %w; cypher=MERGE (n:Task)... CREATE (n)-[:REPLACES {...}]->(o); %s", err, sum)
    }
    return nil
}

// FindLatestTaskIDByVariant returns the Task ID with no incoming REPLACES for a variant.
func FindLatestTaskIDByVariant(ctx context.Context, db *pgxpool.Pool, variant string) (string, error) {
    cy := fmt.Sprintf(
        "MATCH (t:Task {variant: '%s'}) WHERE NOT (:Task)-[:REPLACES]->(t) RETURN t.id ORDER BY t.id LIMIT 1",
        escapeCypherString(variant),
    )
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2) as (id agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy).Scan(&ag); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// FindNextByLevel returns the next Task ID that directly REPLACES current with given level.
func FindNextByLevel(ctx context.Context, db *pgxpool.Pool, currentID, level string) (string, error) {
    lvl := strings.ToLower(strings.TrimSpace(level))
    if lvl != "patch" && lvl != "minor" && lvl != "major" { return "", fmt.Errorf("invalid level: %s", level) }
    cy := fmt.Sprintf(
        "MATCH (n:Task)-[r:REPLACES]->(o:Task {id: '%s'}) WHERE r.level = '%s' RETURN n.id, r.created ORDER BY COALESCE(r.created,'') DESC LIMIT 1",
        escapeCypherString(currentID), escapeCypherString(lvl),
    )
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2) as (id agtype, created agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy).Scan(&ag, new(string)); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// FindLatestFrom returns the latest Task ID reachable by REPLACES edges from current.
func FindLatestFrom(ctx context.Context, db *pgxpool.Pool, currentID string) (string, error) {
    cy := fmt.Sprintf(
        "MATCH (n:Task), (c:Task {id: '%s'}), p=(n)-[:REPLACES*]->(c) WHERE NOT (:Task)-[:REPLACES]->(n) RETURN n.id LIMIT 1",
        escapeCypherString(currentID),
    )
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2) as (id agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy).Scan(&ag); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// Stickie graph helpers

func EnsureStickieVertex(ctx context.Context, db *pgxpool.Pool, id string) error {
    if strings.TrimSpace(id) == "" { return nil }
    cy := fmt.Sprintf("MERGE (s:Stickie {id: '%s'})", escapeCypherString(id))
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2) as (v agtype)", graphName, cy)
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
    // Build labels list literal: ['a','b']
    list := "[]"
    if len(labels) > 0 {
        esc := make([]string, 0, len(labels))
        for _, s := range labels { esc = append(esc, fmt.Sprintf("'%s'", escapeCypherString(s))) }
        list = "[" + strings.Join(esc, ",") + "]"
    }
    cy := fmt.Sprintf(
        "MERGE (a:Stickie {id: '%s'}) MERGE (b:Stickie {id: '%s'}) MERGE (a)-[r:%s]->(b) SET r.labels = %s",
        escapeCypherString(fromID), escapeCypherString(toID), rt, list,
    )
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2) as (v agtype)", graphName, cy)
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

type vertexJSON struct {
    ID        any               `json:"id"`
    Label     string            `json:"label"`
    Properties map[string]any   `json:"properties"`
}

// ListStickieEdges lists edges touching a node id with optional direction and type filter.
// dir: out|in|both
func ListStickieEdges(ctx context.Context, db *pgxpool.Pool, id, dir string, relTypes []string) ([]StickieEdge, error) {
    EnsureStickieGraphSchema(ctx, db)
    if strings.TrimSpace(id) == "" { return nil, nil }
    // Build cypher with escaped literals and optional filters
    escapedID := escapeCypherString(id)
    d := strings.ToLower(strings.TrimSpace(dir))
    var match string
    switch d {
    case "in":
        match = fmt.Sprintf("MATCH (a:Stickie)-[r]->(b:Stickie {id: '%s'})", escapedID)
    case "out":
        match = fmt.Sprintf("MATCH (a:Stickie {id: '%s'})-[r]->(b:Stickie)", escapedID)
    default: // both
        match = fmt.Sprintf("MATCH (a:Stickie)-[r]->(b:Stickie) WHERE a.id = '%s' OR b.id = '%s'", escapedID, escapedID)
    }
    // Normalize and include relationship type filter if provided
    var typeFilter string
    if len(relTypes) > 0 {
        allowed := []string{}
        for _, t := range relTypes {
            if nt := normalizeStickieRelType(t); nt != "" { allowed = append(allowed, nt) }
        }
        if len(allowed) > 0 {
            // type(r) returns a string; compare against string list
            lits := make([]string, 0, len(allowed))
            for _, t := range allowed { lits = append(lits, fmt.Sprintf("'%s'", t)) }
            typeFilter = " AND type(r) IN [" + strings.Join(lits, ",") + "]"
        }
    }
    cy := match + " RETURN a, type(r), r.labels, b"
    if strings.Contains(match, " WHERE ") {
        // WHERE already present (both)
        if typeFilter != "" { cy = strings.Replace(cy, " RETURN", typeFilter+" RETURN", 1) }
    } else {
        if typeFilter != "" { cy = strings.Replace(cy, " RETURN", " WHERE true"+typeFilter+" RETURN", 1) }
    }
    rows, err := db.Query(ctx, "SELECT av::text, typ::text, lab::text, bv::text FROM ag_catalog.cypher($1,$2) as (av agtype, typ agtype, lab agtype, bv agtype)", graphName, cy)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []StickieEdge
    for rows.Next() {
        var av, typ, lab, bv string
        if err := rows.Scan(&av, &typ, &lab, &bv); err != nil { return nil, err }
        typ = strings.Trim(typ, `"`)
        // Decode vertices to obtain properties.id
        var aV, bV vertexJSON
        if err := json.Unmarshal([]byte(av), &aV); err != nil { continue }
        if err := json.Unmarshal([]byte(bv), &bV); err != nil { continue }
        from := ""; to := ""
        if v, ok := aV.Properties["id"].(string); ok { from = v }
        if v, ok := bV.Properties["id"].(string); ok { to = v }
        if from == "" || to == "" { continue }
        // Directional filtering
        switch strings.ToLower(strings.TrimSpace(dir)) {
        case "in":
            if to != id { continue }
        case "both":
            if from != id && to != id { continue }
        default:
            if from != id { continue }
        }
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
        out = append(out, StickieEdge{FromID: from, ToID: to, Type: typ, Labels: labels})
    }
    return out, rows.Err()
}

// GetStickieEdge returns a single relation between exact from/to/type.
func GetStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (*StickieEdge, error) {
    EnsureStickieGraphSchema(ctx, db)
    rt := normalizeStickieRelType(relType)
    if rt == "" { return nil, fmt.Errorf("invalid relation type: %s", relType) }
    cy := fmt.Sprintf(
        "MATCH (a:Stickie {id: '%s'})-[r:%s]->(b:Stickie {id: '%s'}) RETURN a, r.labels, b LIMIT 1",
        escapeCypherString(fromID), rt, escapeCypherString(toID),
    )
    rows, err := db.Query(ctx, "SELECT av::text, lab::text, bv::text FROM ag_catalog.cypher($1,$2) as (av agtype, lab agtype, bv agtype)", graphName, cy)
    if err != nil { return nil, err }
    defer rows.Close()
    if rows.Next() {
        var av, lab, bv string
        if err := rows.Scan(&av, &lab, &bv); err != nil { return nil, err }
        // parse labels
        labels := []string{}
        l := strings.TrimSpace(lab)
        if len(l) > 0 {
            if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") { l = strings.Trim(l, "\"") }
            if strings.HasPrefix(l, "[") { _ = json.Unmarshal([]byte(l), &labels) }
        }
        return &StickieEdge{FromID: fromID, ToID: toID, Type: rt, Labels: labels}, nil
    }
    return nil, rows.Err()
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
        return 0, err
    }
    return 1, nil
}
