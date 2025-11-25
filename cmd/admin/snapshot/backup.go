package snapshot

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    bkp "github.com/flarebyte/baldrick-rebec/internal/backup"
    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagBkpInclude     string
    flagBkpExclude     string
    flagBkpSchema      string
    flagBkpDescription string
    flagBkpTags        []string
    flagBkpWho         string
    flagBkpJSON        bool
)

func parseTags(kvs []string) map[string]any {
    if len(kvs) == 0 { return map[string]any{} }
    m := map[string]any{}
    for _, kv := range kvs {
        kv = strings.TrimSpace(kv)
        if kv == "" { continue }
        parts := strings.SplitN(kv, "=", 2)
        if len(parts) == 2 {
            m[parts[0]] = parts[1]
        } else {
            m[parts[0]] = true
        }
    }
    return m
}

var backupCmd = &cobra.Command{
    Use:   "backup",
    Short: "Create a new schema-aware logical backup",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute); defer cancel()
        // Prefer backup role if configured; fallback to admin
        db, err := pgdao.OpenBackup(ctx, cfg); if err != nil { return err }
        defer db.Close()

        include := splitCSV(flagBkpInclude)
        exclude := splitCSV(flagBkpExclude)
        opt := bkp.BackupOptions{
            Schema:      flagBkpSchema,
            Description: flagBkpDescription,
            Tags:        parseTags(flagBkpTags),
            InitiatedBy: flagBkpWho,
            Include:     include,
            Exclude:     exclude,
        }
        id, err := bkp.CreateBackup(ctx, db, bkp.DefaultEntities(), opt)
        if err != nil { return err }
        if flagBkpJSON {
            return json.NewEncoder(os.Stdout).Encode(map[string]any{"id": id})
        }
        fmt.Fprintln(os.Stdout, id)
        return nil
    },
}

func splitCSV(s string) []string {
    var out []string
    for _, p := range strings.Split(s, ",") {
        p = strings.TrimSpace(p)
        if p != "" { out = append(out, p) }
    }
    return out
}

func init() {
    SnapshotCmd.AddCommand(backupCmd)
    backupCmd.Flags().StringVar(&flagBkpSchema, "schema", "backup", "Backup schema name")
    backupCmd.Flags().StringVar(&flagBkpInclude, "include", "", "Comma-separated entity names to include (overrides defaults)")
    backupCmd.Flags().StringVar(&flagBkpExclude, "exclude", "", "Comma-separated entity names to exclude")
    backupCmd.Flags().StringVar(&flagBkpDescription, "description", "", "Optional description for the backup")
    backupCmd.Flags().StringSliceVar(&flagBkpTags, "tag", nil, "Tags as key=value (repeatable)")
    backupCmd.Flags().StringVar(&flagBkpWho, "who", "", "Initiated by (user/role)")
    backupCmd.Flags().BoolVar(&flagBkpJSON, "json", false, "Output JSON")
}
