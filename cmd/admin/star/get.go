package star

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
    "github.com/spf13/cobra"
)

var (
    flagStarGetID      int64
    flagStarGetRole    string
    flagStarGetVariant string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a starred task by id or by (role, variant)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        var st *pgdao.StarredTask
        if flagStarGetID > 0 {
            st, err = pgdao.GetStarredTaskByID(ctx, db, flagStarGetID)
        } else {
            if strings.TrimSpace(flagStarGetRole) == "" || strings.TrimSpace(flagStarGetVariant) == "" {
                return errors.New("provide --id or both --role and --variant")
            }
            st, err = pgdao.GetStarredTaskByKey(ctx, db, flagStarGetRole, flagStarGetVariant)
        }
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "star id=%d role=%q variant=%q version=%q task_id=%d\n", st.ID, st.Role, st.Variant, st.Version, st.TaskID)
        // JSON
        out := map[string]any{ "id": st.ID, "role": st.Role, "variant": st.Variant, "version": st.Version, "task_id": st.TaskID }
        if st.Created.Valid { out["created"] = st.Created.Time.Format(time.RFC3339Nano) }
        if st.Updated.Valid { out["updated"] = st.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    StarCmd.AddCommand(getCmd)
    getCmd.Flags().Int64Var(&flagStarGetID, "id", 0, "Starred task id")
    getCmd.Flags().StringVar(&flagStarGetRole, "role", "", "Role (with --variant)")
    getCmd.Flags().StringVar(&flagStarGetVariant, "variant", "", "Variant (with --role)")
}
