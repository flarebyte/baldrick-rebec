package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagShowOutput  string
	flagShowSchema  string
	flagShowConcise bool
)

type columnInfo struct {
	Name     string
	DataType string
	Nullable string
	Default  string
	PK       bool
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show database table schemas",
	RunE: func(cmd *cobra.Command, args []string) error {
		outFmt := strings.ToLower(strings.TrimSpace(flagShowOutput))
		if outFmt == "" {
			outFmt = "tables"
		}
		if outFmt != "tables" && outFmt != "md" && outFmt != "json" {
			return errors.New("--output must be 'tables', 'md' or 'json'")
		}
		schema := flagShowSchema
		if strings.TrimSpace(schema) == "" {
			schema = "public"
		}

		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// Fetch tables in the target schema
		tblRows, err := db.Query(ctx, `SELECT table_name FROM information_schema.tables
            WHERE table_schema=$1 AND table_type='BASE TABLE'
            ORDER BY table_name`, schema)
		if err != nil {
			return err
		}
		defer tblRows.Close()
		tables := []string{}
		for tblRows.Next() {
			var t string
			if err := tblRows.Scan(&t); err != nil {
				return err
			}
			tables = append(tables, t)
		}
		if err := tblRows.Err(); err != nil {
			return err
		}

		// stable order, and prefer our core tables first if present
		priority := map[string]int{
			"roles": 0, "workflows": 1, "tags": 2, "projects": 3, "stores": 4,
			"conversations": 5, "experiments": 6, "task_variants": 7, "tasks": 8,
			"scripts_content": 9, "scripts": 10, "messages_content": 11, "messages": 12,
			"workspaces": 13, "packages": 14, "queues": 15, "testcases": 16,
		}
		sort.Slice(tables, func(i, j int) bool {
			pi, pj := 1000, 1000
			if v, ok := priority[tables[i]]; ok {
				pi = v
			}
			if v, ok := priority[tables[j]]; ok {
				pj = v
			}
			if pi != pj {
				return pi < pj
			}
			return tables[i] < tables[j]
		})

		// Output
		switch outFmt {
		case "tables":
			return showAsTables(ctx, db, schema, tables, flagShowConcise)
		case "md":
			return showAsMarkdown(ctx, db, schema, tables, flagShowConcise)
		case "json":
			return showAsJSON(ctx, db, schema, tables, flagShowConcise)
		default:
			return nil
		}
	},
}

func init() {
	DBCmd.AddCommand(showCmd)
	showCmd.Flags().StringVar(&flagShowOutput, "output", "tables", "Output format: tables or md")
	showCmd.Flags().StringVar(&flagShowSchema, "schema", "public", "Schema to inspect (default public)")
	showCmd.Flags().BoolVar(&flagShowConcise, "concise", false, "Concise view (columns and types only)")
}

func fetchColumns(ctx context.Context, db *pgxpool.Pool, schema, table string) ([]columnInfo, error) {
	// Primary key columns
	pkset := map[string]bool{}
	pkRows, err := db.Query(ctx, `SELECT kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
          ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
        WHERE tc.table_schema=$1 AND tc.table_name=$2 AND tc.constraint_type='PRIMARY KEY'`, schema, table)
	if err == nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var col string
			if err := pkRows.Scan(&col); err == nil {
				pkset[col] = true
			}
		}
		_ = pkRows.Err()
	}

	// Columns meta
	rows, err := db.Query(ctx, `SELECT column_name, data_type, is_nullable, COALESCE(column_default,'')
        FROM information_schema.columns
        WHERE table_schema=$1 AND table_name=$2
        ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := []columnInfo{}
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.Name, &c.DataType, &c.Nullable, &c.Default); err != nil {
			return nil, err
		}
		c.PK = pkset[c.Name]
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func showAsTables(ctx context.Context, db *pgxpool.Pool, schema string, tables []string, concise bool) error {
	for i, t := range tables {
		cols, err := fetchColumns(ctx, db, schema, t)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "TABLE: %s\n", t)
		table := tablewriter.NewWriter(os.Stdout)
		if concise {
			table.SetHeader([]string{"COLUMN", "TYPE"})
			for _, c := range cols {
				table.Append([]string{c.Name, c.DataType})
			}
		} else {
			table.SetHeader([]string{"COLUMN", "TYPE", "NULL", "DEFAULT", "PK"})
			for _, c := range cols {
				pk := ""
				if c.PK {
					pk = "yes"
				}
				table.Append([]string{c.Name, c.DataType, strings.ToLower(c.Nullable), c.Default, pk})
			}
		}
		table.Render()
		if i < len(tables)-1 {
			fmt.Fprintln(os.Stdout)
		}
	}
	// Relationships summary table
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "RELATIONSHIPS:")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"FROM", "RELATION", "TO", "NATURE"})
	for _, r := range relationships() {
		table.Append([]string{r.From, r.Rel, r.To, r.Nature})
	}
	table.Render()

	// Access summaries
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "ACCESS: ROLES")
	acc, _ := computeAccess(ctx, db)
	rt := tablewriter.NewWriter(os.Stdout)
	rt.SetHeader([]string{"ROLE", "EXISTS", "LOGIN", "SUPERUSER"})
	for _, r := range acc.Roles {
		rt.Append([]string{r.Name, boolStr(r.Exists), boolStr(r.CanLogin), boolStr(r.Superuser)})
	}
	rt.Render()

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "ACCESS: DATABASE")
	dt := tablewriter.NewWriter(os.Stdout)
	dt.SetHeader([]string{"ROLE", "CONNECT", "CREATE"})
	for _, d := range acc.DB {
		dt.Append([]string{d.Role, boolStr(d.CanConnect), boolStr(d.CanCreateDB)})
	}
	dt.Render()

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "ACCESS: SCHEMAS")
	st := tablewriter.NewWriter(os.Stdout)
	st.SetHeader([]string{"SCHEMA", "ROLE", "USAGE", "CREATE", "DML_OK"})
	for _, s := range acc.Schemas {
		st.Append([]string{s.Schema, s.Role, boolStr(s.Usage), boolStr(s.Create), boolStr(s.DMLOK)})
	}
	st.Render()
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func showAsMarkdown(ctx context.Context, db *pgxpool.Pool, schema string, tables []string, concise bool) error {
	for _, t := range tables {
		fmt.Fprintf(os.Stdout, "## %s\n", t)
		if concise {
			fmt.Fprintln(os.Stdout, "| Column | Type |")
			fmt.Fprintln(os.Stdout, "|---|---|")
		} else {
			fmt.Fprintln(os.Stdout, "| Column | Type | Nullable | Default | PK |")
			fmt.Fprintln(os.Stdout, "|---|---|---|---|---|")
		}
		cols, err := fetchColumns(ctx, db, schema, t)
		if err != nil {
			return err
		}
		for _, c := range cols {
			if concise {
				fmt.Fprintf(os.Stdout, "| %s | %s |\n", c.Name, c.DataType)
			} else {
				pk := ""
				if c.PK {
					pk = "yes"
				}
				fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n", c.Name, c.DataType, strings.ToLower(c.Nullable), c.Default, pk)
			}
		}
		fmt.Fprintln(os.Stdout)
	}
	// Relationships summary
	fmt.Fprintln(os.Stdout, "## Relationships")
	fmt.Fprintln(os.Stdout, "| From | Relation | To | Nature |")
	fmt.Fprintln(os.Stdout, "|---|---|---|---|")
	for _, r := range relationships() {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n", r.From, r.Rel, r.To, r.Nature)
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

type relRow struct{ From, Rel, To, Nature string }

// relationships returns a curated list of relational FKs and graph edges.
func relationships() []relRow {
	return []relRow{
		// Relational FKs
		{"experiments.conversation_id", "->", "conversations.id", "rel"},
		{"messages.content_id", "->", "messages_content.id", "rel"},
		{"messages.from_task_id", "->", "tasks.id", "rel"},
		{"messages.experiment_id", "->", "experiments.id", "rel"},
		{"packages.role_name", "->", "roles.name", "rel"},
		{"packages.task_id", "->", "tasks.id", "rel"},
		{"queues.task_id", "->", "tasks.id", "rel"},
		{"queues.inbound_message", "->", "messages.id", "rel"},
		{"queues.target_workspace_id", "->", "workspaces.id", "rel"},
		// removed: tasks.run_script_id (legacy)
		{"tasks.tool_workspace_id", "->", "workspaces.id", "rel"},
		{"testcases.experiment_id", "->", "experiments.id", "rel"},
		{"workspaces.build_script_id", "->", "scripts.id", "rel"},
		{"workspaces.project_name,role_name", "->", "projects.name,role_name", "rel"},
		{"blackboards.conversation_id", "->", "conversations.id", "rel"},
		{"blackboards.task_id", "->", "tasks.id", "rel"},
		{"blackboards.project_name,role_name", "->", "projects.name,role_name", "rel"},
		{"stickies.blackboard_id", "->", "blackboards.id", "rel"},
		{"stickies.created_by_task_id", "->", "tasks.id", "rel"},
		{"stickies.topic_name,topic_role_name", "->", "topics.name,role_name", "rel"},
		{"stickie_relations.from_id,to_id", "->", "stickies.id", "rel (graph-sql)"},
		{"task_replaces.new_task_id,old_task_id", "->", "tasks.id", "rel (graph-sql)"},
	}
}

// JSON output
type jsonColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable,omitempty"`
	Default  string `json:"default,omitempty"`
	PK       bool   `json:"pk,omitempty"`
}

type jsonTable struct {
	Name    string       `json:"name"`
	Columns []jsonColumn `json:"columns"`
}

type jsonOut struct {
	Schema        string      `json:"schema"`
	Tables        []jsonTable `json:"tables"`
	Relationships []relRow    `json:"relationships"`
	Access        jsonAccess  `json:"access"`
}

func showAsJSON(ctx context.Context, db *pgxpool.Pool, schema string, tables []string, concise bool) error {
	out := jsonOut{Schema: schema}
	for _, t := range tables {
		cols, err := fetchColumns(ctx, db, schema, t)
		if err != nil {
			return err
		}
		jt := jsonTable{Name: t}
		for _, c := range cols {
			jc := jsonColumn{Name: c.Name, Type: c.DataType}
			if !concise {
				jc.Nullable = strings.ToLower(c.Nullable)
				jc.Default = c.Default
				jc.PK = c.PK
			}
			jt.Columns = append(jt.Columns, jc)
		}
		out.Tables = append(out.Tables, jt)
	}
	out.Relationships = relationships()
	// Access
	acc, _ := computeAccess(ctx, db)
	out.Access = acc
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Access reporting
type jsonRole struct {
	Name      string `json:"name"`
	Exists    bool   `json:"exists"`
	CanLogin  bool   `json:"can_login"`
	Superuser bool   `json:"superuser"`
}

type jsonDBPriv struct {
	Role        string `json:"role"`
	CanConnect  bool   `json:"can_connect"`
	CanCreateDB bool   `json:"can_create_db"`
}

type jsonSchemaPriv struct {
	Schema string `json:"schema"`
	Role   string `json:"role"`
	Usage  bool   `json:"usage"`
	Create bool   `json:"create"`
	DMLOK  bool   `json:"dml_ok"`
}

type jsonAccess struct {
	Roles   []jsonRole       `json:"roles"`
	DB      []jsonDBPriv     `json:"database"`
	Schemas []jsonSchemaPriv `json:"schemas"`
}

func computeAccess(ctx context.Context, db *pgxpool.Pool) (jsonAccess, error) {
	// We report for configured roles if present in config: admin, app, backup
	cfg, _ := cfgpkg.Load()
	roles := []string{}
	if cfg.Postgres.Admin.User != "" {
		roles = append(roles, cfg.Postgres.Admin.User)
	}
	if cfg.Postgres.App.User != "" {
		roles = append(roles, cfg.Postgres.App.User)
	}
	if cfg.Postgres.Backup.User != "" {
		roles = append(roles, cfg.Postgres.Backup.User)
	}
	// De-dup while preserving order
	seen := map[string]struct{}{}
	var uniq []string
	for _, r := range roles {
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			uniq = append(uniq, r)
		}
	}
	roles = uniq

	acc := jsonAccess{}
	// Role info
	for _, r := range roles {
		var exists, super, login bool
		// Existence and flags
		_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, r).Scan(&exists)
		if exists {
			_ = db.QueryRow(ctx, `SELECT rolsuper, rolcanlogin FROM pg_roles WHERE rolname=$1`, r).Scan(&super, &login)
		}
		acc.Roles = append(acc.Roles, jsonRole{Name: r, Exists: exists, Superuser: super, CanLogin: login})
		// DB privs
		var canConn, canCreate bool
		_ = db.QueryRow(ctx, `SELECT has_database_privilege($1, current_database(), 'CONNECT')`, r).Scan(&canConn)
		_ = db.QueryRow(ctx, `SELECT has_database_privilege($1, current_database(), 'CREATE')`, r).Scan(&canCreate)
		acc.DB = append(acc.DB, jsonDBPriv{Role: r, CanConnect: canConn, CanCreateDB: canCreate})
	}
	// Schemas of interest: always report public and backup
	// (If 'backup' does not exist, privilege checks will return false, which is informative.)
	schemas := []string{"public", "backup"}
	for _, s := range schemas {
		for _, r := range roles {
			var usage, create bool
			_ = db.QueryRow(ctx, `SELECT has_schema_privilege($1, $2, 'USAGE')`, r, s).Scan(&usage)
			_ = db.QueryRow(ctx, `SELECT has_schema_privilege($1, $2, 'CREATE')`, r, s).Scan(&create)
			// Aggregate DML check: true if role has full DML across all tables in schema
			missingDML, _ := pgdao.MissingTableDML(ctx, db, r, s)
			dmlOK := usage && !missingDML
			acc.Schemas = append(acc.Schemas, jsonSchemaPriv{Schema: s, Role: r, Usage: usage, Create: create, DMLOK: dmlOK})
		}
	}
	return acc, nil
}
