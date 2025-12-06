package tool

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
    flagToolDelName          string
    flagToolDelForce         bool
    flagToolDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a tool by name (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        name := strings.TrimSpace(flagToolDelName)
        if name == "" { return errors.New("--name is required") }
        if !flagToolDelForce {
            fmt.Fprintf(os.Stderr, "About to delete tool %q.\n", name)
            fmt.Fprint(os.Stderr, "Type the tool name to confirm: ")
            reader := bufio.NewReader(os.Stdin)
            line, _ := reader.ReadString('\n')
            if strings.TrimSpace(line) != name { return errors.New("confirmation did not match; aborting") }
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        affected, err := pgdao.DeleteTool(ctx, db, name)
        if err != nil { return err }
        if affected == 0 {
            if flagToolDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "tool %q not found; ignoring\n", name)
                out := map[string]any{"status":"not_found_ignored","name":name}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("tool %q not found", name)
        }
        fmt.Fprintf(os.Stderr, "tool deleted name=%q\n", name)
        out := map[string]any{"status":"deleted","name":name,"deleted":true}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    ToolCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagToolDelName, "name", "", "Tool unique name (required)")
    deleteCmd.Flags().BoolVar(&flagToolDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagToolDelIgnoreMissing, "ignore-missing", false, "Do not error if tool does not exist")
}

