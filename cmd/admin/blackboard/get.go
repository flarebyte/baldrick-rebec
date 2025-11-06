package blackboard

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
    flagBBGetID string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a blackboard by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagBBGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        b, err := pgdao.GetBlackboardByID(ctx, db, flagBBGetID)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "blackboard id=%s role=%q store=%s\n", b.ID, b.RoleName, b.StoreID)
        out := map[string]any{"id":b.ID,"role":b.RoleName,"store_id":b.StoreID}
        if b.ConversationID.Valid { out["conversation_id"] = b.ConversationID.String }
        if b.ProjectName.Valid { out["project"] = b.ProjectName.String }
        if b.TaskID.Valid { out["task_id"] = b.TaskID.String }
        if b.Background.Valid { out["background"] = b.Background.String }
        if b.Guidelines.Valid { out["guidelines"] = b.Guidelines.String }
        if b.Created.Valid { out["created"] = b.Created.Time.Format(time.RFC3339Nano) }
        if b.Updated.Valid { out["updated"] = b.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    BlackboardCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagBBGetID, "id", "", "Blackboard UUID (required)")
}

