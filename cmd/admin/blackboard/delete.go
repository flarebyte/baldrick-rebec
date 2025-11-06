package blackboard

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
    flagBBDelID            string
    flagBBDelForce         bool
    flagBBDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a blackboard by id (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        id := strings.TrimSpace(flagBBDelID)
        if id == "" { return errors.New("--id is required") }
        if !flagBBDelForce {
            fmt.Fprintf(os.Stderr, "About to delete blackboard %q.\n", id)
            fmt.Fprint(os.Stderr, "Type the blackboard id to confirm: ")
            reader := bufio.NewReader(os.Stdin)
            line, _ := reader.ReadString('\n')
            if strings.TrimSpace(line) != id {
                return errors.New("confirmation did not match; aborting")
            }
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        affected, err := pgdao.DeleteBlackboard(ctx, db, id)
        if err != nil { return err }
        if affected == 0 {
            if flagBBDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "blackboard %q not found; ignoring\n", id)
                out := map[string]any{"status":"not_found_ignored","id":id}
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(out)
            }
            return fmt.Errorf("blackboard %q not found", id)
        }
        fmt.Fprintf(os.Stderr, "blackboard deleted id=%q\n", id)
        out := map[string]any{"status":"deleted","id":id,"deleted":true}
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    BlackboardCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagBBDelID, "id", "", "Blackboard UUID (required)")
    deleteCmd.Flags().BoolVar(&flagBBDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagBBDelIgnoreMissing, "ignore-missing", false, "Do not error if blackboard does not exist")
}

