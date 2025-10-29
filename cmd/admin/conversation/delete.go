package conversation

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
    flagConvDelID           string
    flagConvDelForce        bool
    flagConvDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a conversation by id (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagConvDelID) == "" { return errors.New("--id is required") }
        if !flagConvDelForce {
            fmt.Fprintf(os.Stderr, "About to delete conversation id=%d.\n", flagConvDelID)
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
        affected, err := pgdao.DeleteConversation(ctx, db, flagConvDelID)
        if err != nil { return err }
        if affected == 0 {
            if flagConvDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "conversation id=%d not found; ignoring\n", flagConvDelID)
                out := map[string]any{"status":"not_found_ignored","id":flagConvDelID}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("conversation id=%d not found", flagConvDelID)
        }
        fmt.Fprintf(os.Stderr, "conversation deleted id=%d\n", flagConvDelID)
        out := map[string]any{"status":"deleted","deleted":true,"id":flagConvDelID}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    ConversationCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagConvDelID, "id", "", "Conversation UUID (required)")
    deleteCmd.Flags().BoolVar(&flagConvDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagConvDelIgnoreMissing, "ignore-missing", false, "Do not error if conversation does not exist")
}
