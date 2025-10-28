package role

import (
    "bufio"
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
    flagRoleDelName   string
    flagRoleDelForce  bool
    flagRoleDelIgnore bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a role by name",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRoleDelName) == "" {
            return errors.New("--name is required")
        }
        if !flagRoleDelForce {
            fmt.Fprintf(os.Stderr, "About to delete role name=%q.\n", flagRoleDelName)
            fmt.Fprint(os.Stderr, "Type 'yes' to confirm: ")
            reader := bufio.NewReader(os.Stdin)
            line, _ := reader.ReadString('\n')
            if strings.TrimSpace(strings.ToLower(line)) != "yes" {
                return errors.New("confirmation not 'yes'; aborting")
            }
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        affected, err := pgdao.DeleteRole(ctx, db, flagRoleDelName)
        if err != nil { return err }
        if affected == 0 {
            if flagRoleDelIgnore {
                fmt.Fprintf(os.Stderr, "role %q not found; ignoring\n", flagRoleDelName)
                out := map[string]any{"status":"not_found_ignored"}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("role %q not found", flagRoleDelName)
        }
        fmt.Fprintf(os.Stderr, "role deleted name=%q\n", flagRoleDelName)
        out := map[string]any{"status":"deleted","deleted":true}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    RoleCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagRoleDelName, "name", "", "Role name (required)")
    deleteCmd.Flags().BoolVar(&flagRoleDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagRoleDelIgnore, "ignore-missing", false, "Do not error if role not found")
}

