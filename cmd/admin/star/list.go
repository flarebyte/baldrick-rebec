package star

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagStarListRole    string
    flagStarListVariant string
    flagStarListLimit   int
    flagStarListOffset  int
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List starred tasks (optionally filter by role or variant)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        items, err := pgdao.ListStarredTasks(ctx, db, flagStarListRole, flagStarListVariant, flagStarListLimit, flagStarListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "starred tasks: %d\n", len(items))
        arr := make([]map[string]any, 0, len(items))
        for _, st := range items {
            m := map[string]any{
                "id": st.ID,
                "role": st.Role,
                "variant": st.Variant,
                "version": st.Version,
                "task_id": st.TaskID,
            }
            if st.Created.Valid { m["created"] = st.Created.Time.Format(time.RFC3339Nano) }
            if st.Updated.Valid { m["updated"] = st.Updated.Time.Format(time.RFC3339Nano) }
            arr = append(arr, m)
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(arr)
    },
}

func init() {
    StarCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagStarListRole, "role", "", "Filter by role")
    listCmd.Flags().StringVar(&flagStarListVariant, "variant", "", "Filter by variant")
    listCmd.Flags().IntVar(&flagStarListLimit, "limit", 100, "Max rows")
    listCmd.Flags().IntVar(&flagStarListOffset, "offset", 0, "Offset for pagination")
}
