package postgres

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

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
    cy, p := cypherArgs(`
        MERGE (t:Task {id: $id})
        SET t.variant = $variant, t.command = $command
    `, map[string]any{"id": id, "variant": variant, "command": command})
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2,$3) as (v agtype)", graphName, cy, string(p))
    if err != nil {
        sum := strings.Join([]string{
            dbutil.ParamSummary("id", id),
            dbutil.ParamSummary("variant", variant),
            dbutil.ParamSummary("command", command),
        }, ",")
        // Include cypher text (parameterized) but not values
        return fmt.Errorf("AGE ensure Task vertex failed: %w; cypher=MERGE (t:Task {id: $id}) SET t.variant=$variant, t.command=$command; %s", err, sum)
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
    cy, p := cypherArgs(`
        MERGE (n:Task {id: $new})
        MERGE (o:Task {id: $old})
        CREATE (n)-[:REPLACES {level: $level, comment: $comment, created: $created}]->(o)
    `, map[string]any{"new": newTaskID, "old": oldTaskID, "level": lvl, "comment": comment, "created": createdISO})
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2,$3) as (v agtype)", graphName, cy, string(p))
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
    cy, p := cypherArgs(`
        MATCH (t:Task {variant: $variant})
        WHERE NOT (:Task)-[:REPLACES]->(t)
        RETURN t.id
        ORDER BY t.id
        LIMIT 1
    `, map[string]any{"variant": variant})
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2,$3) as (id agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy, string(p)).Scan(&ag); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// FindNextByLevel returns the next Task ID that directly REPLACES current with given level.
func FindNextByLevel(ctx context.Context, db *pgxpool.Pool, currentID, level string) (string, error) {
    lvl := strings.ToLower(strings.TrimSpace(level))
    if lvl != "patch" && lvl != "minor" && lvl != "major" { return "", fmt.Errorf("invalid level: %s", level) }
    cy, p := cypherArgs(`
        MATCH (n:Task)-[r:REPLACES]->(o:Task {id: $id})
        WHERE r.level = $level
        RETURN n.id, r.created
        ORDER BY COALESCE(r.created,'') DESC
        LIMIT 1
    `, map[string]any{"id": currentID, "level": lvl})
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2,$3) as (id agtype, created agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy, string(p)).Scan(&ag, new(string)); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// FindLatestFrom returns the latest Task ID reachable by REPLACES edges from current.
func FindLatestFrom(ctx context.Context, db *pgxpool.Pool, currentID string) (string, error) {
    cy, p := cypherArgs(`
        MATCH (n:Task), (c:Task {id: $id}), p=(n)-[:REPLACES*]->(c)
        WHERE NOT (:Task)-[:REPLACES]->(n)
        RETURN n.id
        LIMIT 1
    `, map[string]any{"id": currentID})
    q := "SELECT id::text FROM ag_catalog.cypher($1,$2,$3) as (id agtype)"
    var ag string
    if err := db.QueryRow(ctx, q, graphName, cy, string(p)).Scan(&ag); err != nil {
        return "", fmt.Errorf("AGE cypher failed: %v\ncypher=%s", err, cy)
    }
    return strings.Trim(ag, `"`), nil
}

// Stickie graph helpers

func EnsureStickieVertex(ctx context.Context, db *pgxpool.Pool, id string) error {
    if strings.TrimSpace(id) == "" { return nil }
    cy, p := cypherArgs(`MERGE (s:Stickie {id: $id})`, map[string]any{"id": id})
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2,$3) as (v agtype)", graphName, cy, string(p))
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
    var cyStr string
    switch rt {
    case "INCLUDES":
        cyStr = `MERGE (a:Stickie {id: $from}) MERGE (b:Stickie {id: $to}) MERGE (a)-[r:INCLUDES]->(b) SET r.labels = $labels`
    case "CAUSES":
        cyStr = `MERGE (a:Stickie {id: $from}) MERGE (b:Stickie {id: $to}) MERGE (a)-[r:CAUSES]->(b) SET r.labels = $labels`
    case "USES":
        cyStr = `MERGE (a:Stickie {id: $from}) MERGE (b:Stickie {id: $to}) MERGE (a)-[r:USES]->(b) SET r.labels = $labels`
    case "REPRESENTS":
        cyStr = `MERGE (a:Stickie {id: $from}) MERGE (b:Stickie {id: $to}) MERGE (a)-[r:REPRESENTS]->(b) SET r.labels = $labels`
    case "CONTRASTS_WITH":
        cyStr = `MERGE (a:Stickie {id: $from}) MERGE (b:Stickie {id: $to}) MERGE (a)-[r:CONTRASTS_WITH]->(b) SET r.labels = $labels`
    }
    cy, p := cypherArgs(cyStr, map[string]any{"from": fromID, "to": toID, "labels": labels})
    _, err := db.Exec(ctx, "SELECT * FROM ag_catalog.cypher($1,$2,$3) as (v agtype)", graphName, cy, p)
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
    // Build cypher returning full vertices; filter client-side on properties.id to avoid operator mismatches
    d := strings.ToLower(strings.TrimSpace(dir))
    cy := `MATCH (a:Stickie)-[r]->(b:Stickie)
           WHERE ($dir = 'out'  AND a.id = $id)
              OR ($dir = 'in'   AND b.id = $id)
              OR ($dir = 'both' AND (a.id = $id OR b.id = $id))
           AND ($types = [] OR type(r) IN $types)
           RETURN a, type(r), r.labels, b`
    params := map[string]any{"id": id, "dir": d, "types": relTypes}
    cy, p := cypherArgs(cy, params)
    rows, err := db.Query(ctx, "SELECT av::text, typ::text, lab::text, bv::text FROM ag_catalog.cypher($1,$2,$3) as (av agtype, typ agtype, lab agtype, bv agtype)", graphName, cy, string(p))
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
    cy, p := cypherArgs(`MATCH (a:Stickie)-[r]->(b:Stickie) WHERE type(r)=$rtype RETURN a, r.labels, b LIMIT 200`, map[string]any{"rtype": rt})
    rows, err := db.Query(ctx, "SELECT av::text, lab::text, bv::text FROM ag_catalog.cypher($1,$2,$3) as (av agtype, lab agtype, bv agtype)", graphName, cy, string(p))
    if err != nil { return nil, err }
    defer rows.Close()
    for rows.Next() {
        var av, lab, bv string
        if err := rows.Scan(&av, &lab, &bv); err != nil { return nil, err }
        var aV, bV vertexJSON
        if err := json.Unmarshal([]byte(av), &aV); err != nil { continue }
        if err := json.Unmarshal([]byte(bv), &bV); err != nil { continue }
        var fromProp, toProp string
        if v, ok := aV.Properties["id"].(string); ok { fromProp = v }
        if v, ok := bV.Properties["id"].(string); ok { toProp = v }
        if fromProp == fromID && toProp == toID {
            // parse labels
            labels := []string{}
            l := strings.TrimSpace(lab)
            if len(l) > 0 {
                if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") { l = strings.Trim(l, "\"") }
                if strings.HasPrefix(l, "[") { _ = json.Unmarshal([]byte(l), &labels) }
            }
            return &StickieEdge{FromID: fromID, ToID: toID, Type: rt, Labels: labels}, nil
        }
    }
    return nil, nil
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
