package pkg

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
    flagPkgRoleName string
    flagPkgVariant  string
    flagPkgVersion  string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Set (or update) a package for a role by variant and version",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagPkgRoleName) == "" || strings.TrimSpace(flagPkgVariant) == "" || strings.TrimSpace(flagPkgVersion) == "" {
            return errors.New("--role and --variant and --version are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        p, err := pgdao.UpsertPackage(ctx, db, flagPkgRoleName, flagPkgVariant, flagPkgVersion)
        if err != nil { return err }
        // Human-readable
        fmt.Fprintf(os.Stderr, "package set role_name=%q variant=%q version=%q task_id=%s id=%s\n", flagPkgRoleName, flagPkgVariant, flagPkgVersion, p.TaskID, p.ID)
        // JSON
        out := map[string]any{
            "status":  "upserted",
            "id":      p.ID,
            "role_name": p.RoleName,
            "variant": p.Variant,
            "version": p.Version,
            "task_id": p.TaskID,
        }
        if p.Created.Valid { out["created"] = p.Created.Time.Format(time.RFC3339Nano) }
        if p.Updated.Valid { out["updated"] = p.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    PackageCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagPkgRoleName, "role", "", "Role name (e.g., user, admin) (required)")
    setCmd.Flags().StringVar(&flagPkgVariant, "variant", "", "Task selector variant (e.g., unit/go) (required)")
    setCmd.Flags().StringVar(&flagPkgVersion, "version", "", "Task semver version (required)")
}

