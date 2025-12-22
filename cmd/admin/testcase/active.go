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

        var role string
        if err := db.QueryRow(ctx, `SELECT role_name FROM conversations WHERE id=$1::uuid`, flagTCActiveConversation).Scan(&role); err != nil {
            return err
        }

        // Dummy interactive behavior: just print a mock JSON payload
        fmt.Fprintf(os.Stderr, "testcase active: interactive mode (mock)\n")
        out := map[string]any{
            "status":       "ok",
            "mode":         "interactive-mock",
            "role":         role,
            "conversation": flagTCActiveConversation,
            "items": []map[string]any{
                {"id": "tc-mock-1", "title": "Sample Testcase", "status": "OK"},
                {"id": "tc-mock-2", "title": "Another Testcase", "status": "KO"},
            },
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
