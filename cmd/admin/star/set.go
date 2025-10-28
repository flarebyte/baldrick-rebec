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
    flagStarRole    string
    flagStarVariant string
    flagStarVersion string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Set (or update) a starred task for a role by variant and version",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagStarRole) == "" || strings.TrimSpace(flagStarVariant) == "" || strings.TrimSpace(flagStarVersion) == "" {
            return errors.New("--role, --variant, and --version are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        st, err := pgdao.UpsertStarredTask(ctx, db, flagStarRole, flagStarVariant, flagStarVersion)
        if err != nil { return err }
        // Human-readable
        fmt.Fprintf(os.Stderr, "star set role=%q variant=%q version=%q task_id=%d id=%d\n", flagStarRole, flagStarVariant, flagStarVersion, st.TaskID, st.ID)
        // JSON output
        out := map[string]any{
            "status":  "upserted",
            "id":      st.ID,
            "role":    st.Role,
            "variant": st.Variant,
            "version": st.Version,
            "task_id": st.TaskID,
        }
        if st.Created.Valid { out["created"] = st.Created.Time.Format(time.RFC3339Nano) }
        if st.Updated.Valid { out["updated"] = st.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    StarCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagStarRole, "role", "", "Role name (e.g., user, admin) (required)")
    setCmd.Flags().StringVar(&flagStarVariant, "variant", "", "Task selector variant (e.g., unit/go) (required)")
    setCmd.Flags().StringVar(&flagStarVersion, "version", "", "Task semver version (required)")
}
