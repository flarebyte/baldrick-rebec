package workflow

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
    flagWFDelName         string
    flagWFDelForce        bool
    flagWFDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a workflow by name (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        name := strings.TrimSpace(flagWFDelName)
        if name == "" {
            return errors.New("--name is required")
        }
        if !flagWFDelForce {
            fmt.Fprintf(os.Stderr, "About to delete workflow %q (and any dependent tasks).\n", name)
            fmt.Fprint(os.Stderr, "Type the workflow name to confirm: ")
            reader := bufio.NewReader(os.Stdin)
            line, _ := reader.ReadString('\n')
            if strings.TrimSpace(line) != name {
                return errors.New("confirmation did not match; aborting")
            }
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        affected, err := pgdao.DeleteWorkflow(ctx, db, name)
        if err != nil { return err }
        if affected == 0 {
            if flagWFDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "workflow %q not found; ignoring\n", name)
                out := map[string]any{"status":"not_found_ignored","name":name}
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(out)
            }
            return fmt.Errorf("workflow %q not found", name)
        }
        // Human-readable
        fmt.Fprintf(os.Stderr, "workflow deleted name=%q\n", name)
        // JSON
        out := map[string]any{"status":"deleted","name":name,"deleted":true}
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    WorkflowCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagWFDelName, "name", "", "Workflow unique name (required)")
    deleteCmd.Flags().BoolVar(&flagWFDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagWFDelIgnoreMissing, "ignore-missing", false, "Do not error if workflow does not exist")
}

