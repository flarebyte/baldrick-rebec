package star

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
    flagStarDelID      string
    flagStarDelRole    string
    flagStarDelVariant string
    flagStarDelForce   bool
    flagStarDelIgnore  bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a starred task by id or by (role, variant)",
    RunE: func(cmd *cobra.Command, args []string) error {
        var ident string
        byID := false
        if strings.TrimSpace(flagStarDelID) != "" {
            ident = fmt.Sprintf("id=%s", flagStarDelID)
            byID = true
        } else {
            if strings.TrimSpace(flagStarDelRole) == "" || strings.TrimSpace(flagStarDelVariant) == "" {
                return errors.New("provide --id or both --role and --variant")
            }
            ident = fmt.Sprintf("role=%s variant=%s", flagStarDelRole, flagStarDelVariant)
        }
        if !flagStarDelForce {
            fmt.Fprintf(os.Stderr, "About to delete starred task (%s).\n", ident)
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
        var affected int64
        if byID {
            affected, err = pgdao.DeleteStarredTaskByID(ctx, db, flagStarDelID)
        } else {
            affected, err = pgdao.DeleteStarredTaskByKey(ctx, db, flagStarDelRole, flagStarDelVariant)
        }
        if err != nil { return err }
        if affected == 0 {
            if flagStarDelIgnore {
                fmt.Fprintf(os.Stderr, "starred task (%s) not found; ignoring\n", ident)
                out := map[string]any{"status":"not_found_ignored"}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("starred task (%s) not found", ident)
        }
        fmt.Fprintf(os.Stderr, "starred task deleted (%s)\n", ident)
        out := map[string]any{"status":"deleted","deleted":true}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    StarCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagStarDelID, "id", "", "Starred task UUID")
    deleteCmd.Flags().StringVar(&flagStarDelRole, "role", "", "Role (with --variant)")
    deleteCmd.Flags().StringVar(&flagStarDelVariant, "variant", "", "Variant (with --role)")
    deleteCmd.Flags().BoolVar(&flagStarDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagStarDelIgnore, "ignore-missing", false, "Do not error if not found")
}
