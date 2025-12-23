package testcase

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
    flagTCActiveConversation string
)

// activeCmd is an interactive variant of list (dummy implementation).
// For now, it validates required flags and returns a mock payload.
var activeCmd = &cobra.Command{
    Use:   "active",
    Short: "Interactive list of active testcases (mock)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTCActiveConversation) == "" {
            return errors.New("--conversation is required")
        }

        // Load config and resolve role_name from the conversation
        cfg, err := cfgpkg.Load()
        if err != nil {
            return err
        }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil {
            return err
        }
        defer db.Close()

        // Resolve conversation and role via DAO
        conv, err := pgdao.GetConversationByID(ctx, db, flagTCActiveConversation)
        if err != nil {
            return err
        }
        role := strings.TrimSpace(conv.RoleName)
        if role == "" {
            return errors.New("conversation has no role_name")
        }

        // Fetch last experiment for the conversation
        exps, err := pgdao.ListExperiments(ctx, db, conv.ID, 1, 0)
        if err != nil {
            return err
        }
        if len(exps) == 0 {
            return fmt.Errorf("no experiment found for conversation %s", conv.ID)
        }
        exp := exps[0]

        // Fetch testcases for that experiment and role
        tcs, err := pgdao.ListTestcases(ctx, db, role, exp.ID, "", 100, 0)
        if err != nil {
            return err
        }

        // Output
        fmt.Fprintf(os.Stderr, "testcase active: conversation=%s role=%s experiment=%s count=%d\n", conv.ID, role, exp.ID, len(tcs))

        // Minimal JSON payload with items
        arr := make([]map[string]any, 0, len(tcs))
        for _, t := range tcs {
            m := map[string]any{"id": t.ID, "title": t.Title, "status": t.Status}
            if t.Created.Valid {
                m["created"] = t.Created.Time.Format(time.RFC3339Nano)
            }
            if t.Name.Valid {
                m["name"] = t.Name.String
            }
            if t.Package.Valid {
                m["package"] = t.Package.String
            }
            if t.Classname.Valid {
                m["classname"] = t.Classname.String
            }
            if t.File.Valid {
                m["file"] = t.File.String
            }
            if t.Line.Valid {
                m["line"] = t.Line.Int64
            }
            if t.ExecutionTime.Valid {
                m["execution_time"] = t.ExecutionTime.Float64
            }
            if t.ErrorMessage.Valid {
                m["error"] = t.ErrorMessage.String
            }
            if len(t.Tags) > 0 {
                m["tags"] = t.Tags
            }
            arr = append(arr, m)
        }
        out := map[string]any{
            "status":       "ok",
            "mode":         "interactive",
            "conversation": conv.ID,
            "role":         role,
            "experiment":   exp.ID,
            "items":        arr,
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TestcaseCmd.AddCommand(activeCmd)
    activeCmd.Flags().StringVar(&flagTCActiveConversation, "conversation", "", "Conversation ID (required)")
}
