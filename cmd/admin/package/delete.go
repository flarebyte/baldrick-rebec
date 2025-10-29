package pkg

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
    flagPkgDelID      string
    flagPkgDelRole    string
    flagPkgDelVariant string
    flagPkgDelForce   bool
    flagPkgDelIgnore  bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a package by id or by (role, variant)",
    RunE: func(cmd *cobra.Command, args []string) error {
        var ident string
        byID := false
        if strings.TrimSpace(flagPkgDelID) != "" {
            ident = fmt.Sprintf("id=%s", flagPkgDelID)
            byID = true
        } else {
            if strings.TrimSpace(flagPkgDelRole) == "" || strings.TrimSpace(flagPkgDelVariant) == "" {
                return errors.New("provide --id or both --role and --variant")
            }
            ident = fmt.Sprintf("role=%s variant=%s", flagPkgDelRole, flagPkgDelVariant)
        }
        if !flagPkgDelForce {
            fmt.Fprintf(os.Stderr, "About to delete package (%s).\n", ident)
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
            affected, err = pgdao.DeletePackageByID(ctx, db, flagPkgDelID)
        } else {
            affected, err = pgdao.DeletePackageByKey(ctx, db, flagPkgDelRole, flagPkgDelVariant)
        }
        if err != nil { return err }
        if affected == 0 {
            if flagPkgDelIgnore {
                fmt.Fprintf(os.Stderr, "package (%s) not found; ignoring\n", ident)
                out := map[string]any{"status":"not_found_ignored"}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("package (%s) not found", ident)
        }
        fmt.Fprintf(os.Stderr, "package deleted (%s)\n", ident)
        out := map[string]any{"status":"deleted","deleted":true}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    PackageCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagPkgDelID, "id", "", "Package UUID")
    deleteCmd.Flags().StringVar(&flagPkgDelRole, "role", "", "Role name (with --variant)")
    deleteCmd.Flags().StringVar(&flagPkgDelVariant, "variant", "", "Variant (with --role)")
    deleteCmd.Flags().BoolVar(&flagPkgDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagPkgDelIgnore, "ignore-missing", false, "Do not error if not found")
}

