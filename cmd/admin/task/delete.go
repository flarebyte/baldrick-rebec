package task

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
    flagTaskDelID   string
    flagTaskDelWF   string
    flagTaskDelCmd  string
    flagTaskDelVar  string
    flagTaskDelVer  string
    flagTaskDelForce bool
    flagTaskDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a task by id or key (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Identify target
        var ident string
        var byID bool
        if strings.TrimSpace(flagTaskDelID) != "" {
            ident = fmt.Sprintf("id=%s", flagTaskDelID)
            byID = true
        } else {
            if strings.TrimSpace(flagTaskDelWF)=="" || strings.TrimSpace(flagTaskDelCmd)=="" || strings.TrimSpace(flagTaskDelVer)=="" {
                return errors.New("provide --id or all of --workflow, --command, --version (and optionally --variant)")
            }
            ident = fmt.Sprintf("workflow=%s command=%s variant=%s version=%s", flagTaskDelWF, flagTaskDelCmd, flagTaskDelVar, flagTaskDelVer)
        }
        if !flagTaskDelForce {
            fmt.Fprintf(os.Stderr, "About to delete task (%s).\n", ident)
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
            affected, err = pgdao.DeleteTaskByID(ctx, db, flagTaskDelID)
        } else {
            selector := flagTaskDelVar
            if strings.TrimSpace(selector) == "" { selector = flagTaskDelCmd }
            affected, err = pgdao.DeleteTaskByKey(ctx, db, selector, flagTaskDelVer)
        }
        if err != nil { return err }
        if affected == 0 {
            if flagTaskDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "task (%s) not found; ignoring\n", ident)
                out := map[string]any{"status":"not_found_ignored","id": flagTaskDelID, "workflow": flagTaskDelWF, "command": flagTaskDelCmd, "variant": flagTaskDelVar, "version": flagTaskDelVer}
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(out)
            }
            return fmt.Errorf("task (%s) not found", ident)
        }
        fmt.Fprintf(os.Stderr, "task deleted (%s)\n", ident)
        out := map[string]any{"status":"deleted","deleted":true}
        if byID { out["id"] = flagTaskDelID } else { out["workflow"]=flagTaskDelWF; out["command"]=flagTaskDelCmd; out["variant"]=flagTaskDelVar; out["version"]=flagTaskDelVer }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TaskCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagTaskDelID, "id", "", "Task UUID")
    deleteCmd.Flags().StringVar(&flagTaskDelWF, "workflow", "", "Workflow name (with --command and --version)")
    deleteCmd.Flags().StringVar(&flagTaskDelCmd, "command", "", "Task command (with --workflow and --version)")
    deleteCmd.Flags().StringVar(&flagTaskDelVar, "variant", "", "Task variant (optional)")
    deleteCmd.Flags().StringVar(&flagTaskDelVer, "version", "", "Task version (with --workflow and --command)")
    deleteCmd.Flags().BoolVar(&flagTaskDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagTaskDelIgnoreMissing, "ignore-missing", false, "Do not error if task does not exist")
}
