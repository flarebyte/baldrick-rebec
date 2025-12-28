package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagCountJSON    bool
	flagCountPerRole bool
)

var countCmd = &cobra.Command{
	Use:   "count",
	Short: "Count rows for each public table",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// List public base tables
		rows, err := db.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		// Count rows per table
		counts := map[string]int64{}
		identRe := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
		for _, t := range tables {
			if !identRe.MatchString(t) {
				continue
			}
			var n int64
			q := fmt.Sprintf("SELECT COUNT(*) FROM public.%s", t)
			if err := db.QueryRow(ctx, q).Scan(&n); err != nil {
				return err
			}
			counts[t] = n
		}

		// Include SQL-backed "graph" counts
		var n int64
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM task_replaces").Scan(&n); err == nil {
			counts["task_replaces"] = n
		}
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM stickie_relations").Scan(&n); err == nil {
			counts["stickie_relations"] = n
		}

		if flagCountPerRole && !flagCountJSON {
			// Build role list (first 10)
			roleRows, err := db.Query(ctx, `SELECT name FROM roles ORDER BY name ASC LIMIT 10`)
			if err != nil {
				return err
			}
			roles := []string{}
			for roleRows.Next() {
				var rn string
				_ = roleRows.Scan(&rn)
				roles = append(roles, rn)
			}
			roleRows.Close()
			// Determine which tables have role_name column
			hasRoleCol := map[string]bool{}
			for _, t := range tables {
				if !identRe.MatchString(t) {
					continue
				}
				var ok bool
				if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name='role_name')`, t).Scan(&ok); err != nil {
					return err
				}
				hasRoleCol[t] = ok
			}
			// Render table
			tw := tablewriter.NewWriter(os.Stdout)
			header := append([]string{"TABLE", "TOTAL"}, roles...)
			tw.SetHeader(header)
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, tbl := range keys {
				row := []string{tbl, fmt.Sprintf("%d", counts[tbl])}
				if hasRoleCol[tbl] && len(roles) > 0 {
					for _, rn := range roles {
						var c int64
						q := fmt.Sprintf("SELECT COUNT(*) FROM public.%s WHERE role_name=$1", tbl)
						if err := db.QueryRow(ctx, q, rn).Scan(&c); err != nil {
							c = 0
						}
						row = append(row, fmt.Sprintf("%d", c))
					}
				} else {
					// no direct role column: try deriving via joins for known tables
					for _, rn := range roles {
						var c int64
						var q string
						switch tbl {
						case "experiments":
							q = `SELECT COUNT(*) FROM experiments e JOIN conversations c ON c.id=e.conversation_id WHERE c.role_name=$1`
						case "messages_content":
							q = `SELECT COUNT(*) FROM messages_content mc WHERE EXISTS (SELECT 1 FROM messages m WHERE m.content_id=mc.id AND m.role_name=$1)`
						case "scripts_content":
							q = `SELECT COUNT(*) FROM scripts_content sc WHERE EXISTS (SELECT 1 FROM scripts s WHERE s.script_content_id=sc.id AND s.role_name=$1)`
						case "stickies":
							q = `SELECT COUNT(*) FROM stickies s JOIN blackboards b ON b.id=s.blackboard_id WHERE b.role_name=$1`
						case "stickie_relations":
							q = `SELECT COUNT(*) FROM stickie_relations r JOIN stickies s ON s.id=r.from_id JOIN blackboards b ON b.id=s.blackboard_id WHERE b.role_name=$1`
						case "task_variants":
							q = `SELECT COUNT(*) FROM task_variants tv JOIN workflows w ON w.name=tv.workflow_id WHERE w.role_name=$1`
						case "task_replaces":
							q = `SELECT COUNT(*) FROM task_replaces tr JOIN tasks t ON t.id=tr.new_task_id WHERE t.role_name=$1`
						case "queues":
							q = `SELECT COUNT(*)
                                 FROM queues q
                                 LEFT JOIN tasks t ON t.id = q.task_id
                                 LEFT JOIN messages m ON m.id = q.inbound_message
                                 LEFT JOIN workspaces w ON w.id = q.target_workspace_id
                                 WHERE COALESCE(t.role_name, m.role_name, w.role_name) = $1`
						default:
							q = ""
						}
						if q != "" {
							if err := db.QueryRow(ctx, q, rn).Scan(&c); err != nil {
								c = 0
							}
							row = append(row, fmt.Sprintf("%d", c))
						} else {
							row = append(row, "0")
						}
					}
				}
				tw.Append(row)
			}
			tw.Render()
			return nil
		}

		// Human-readable to stderr
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "%s\t%d\n", k, counts[k])
		}

		// JSON to stdout
		enc := json.NewEncoder(os.Stdout)
		if flagCountJSON {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(counts)
	},
}

func init() {
	DBCmd.AddCommand(countCmd)
	countCmd.Flags().BoolVar(&flagCountJSON, "json", false, "Pretty-print JSON output")
	countCmd.Flags().BoolVar(&flagCountPerRole, "per-role", false, "Display table counts per role (first 10 roles)")
}
