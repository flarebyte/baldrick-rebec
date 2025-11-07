package workspace

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"
)

var (
    flagWSListLimit  int
    flagWSListOffset int
    flagWSListOutput string
    flagWSListRole   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List workspaces for a role (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagWSListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        ws, err := pgdao.ListWorkspaces(ctx, db, flagWSListRole, flagWSListLimit, flagWSListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "workspaces: %d\n", len(ws))
        out := strings.ToLower(strings.TrimSpace(flagWSListOutput))
        if out == "json" {
            arr := make([]map[string]any, 0, len(ws))
            for _, w := range ws {
                item := map[string]any{"id": w.ID, "role": w.RoleName}
                if w.Created.Valid { item["created"] = w.Created.Time.Format(time.RFC3339Nano) }
                if w.Updated.Valid { item["updated"] = w.Updated.Time.Format(time.RFC3339Nano) }
                if w.Description.Valid && w.Description.String != "" { item["description"] = w.Description.String }
                if w.ProjectName.Valid && w.ProjectName.String != "" { item["project"] = w.ProjectName.String }
                if w.BuildScriptID.Valid { item["build_script_id"] = w.BuildScriptID.String }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        // table default
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "PROJECT", "UPDATED"})
        for _, w := range ws {
            updated := ""; if w.Updated.Valid { updated = w.Updated.Time.Format(time.RFC3339) }
            proj := ""; if w.ProjectName.Valid { proj = w.ProjectName.String }
            table.Append([]string{w.ID, proj, updated})
        }
        table.Render()
        return nil
    },
}

func init() {
    WorkspaceCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagWSListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagWSListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagWSListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagWSListRole, "role", "", "Role name (required)")
}
