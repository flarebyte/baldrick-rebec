package blackboard

import (
    "context"
    "database/sql"
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
    flagBBID         string
    flagBBRole       string
    flagBBStoreID    string
    flagBBConvID     string
    flagBBProject    string
    flagBBTaskID     string
    flagBBBackground string
    flagBBGuidelines string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a blackboard (by id)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagBBRole) == "" { return errors.New("--role is required") }
        if strings.TrimSpace(flagBBID) == "" && strings.TrimSpace(flagBBStoreID) == "" {
            return errors.New("--store-id is required when creating a blackboard")
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()

        b := &pgdao.Blackboard{ ID: strings.TrimSpace(flagBBID), RoleName: flagBBRole }
        if strings.TrimSpace(flagBBStoreID) != "" { b.StoreID = strings.TrimSpace(flagBBStoreID) }
        if strings.TrimSpace(flagBBConvID) != "" { b.ConversationID = sql.NullString{String: strings.TrimSpace(flagBBConvID), Valid: true} }
        if strings.TrimSpace(flagBBProject) != "" { b.ProjectName = sql.NullString{String: strings.TrimSpace(flagBBProject), Valid: true} }
        if strings.TrimSpace(flagBBTaskID) != "" { b.TaskID = sql.NullString{String: strings.TrimSpace(flagBBTaskID), Valid: true} }
        if strings.TrimSpace(flagBBBackground) != "" { b.Background = sql.NullString{String: flagBBBackground, Valid: true} }
        if strings.TrimSpace(flagBBGuidelines) != "" { b.Guidelines = sql.NullString{String: flagBBGuidelines, Valid: true} }

        if err := pgdao.UpsertBlackboard(ctx, db, b); err != nil { return err }

        fmt.Fprintf(os.Stderr, "blackboard upserted id=%s role=%q store=%s\n", b.ID, b.RoleName, b.StoreID)
        out := map[string]any{"status":"upserted","id":b.ID,"role":b.RoleName,"store_id":b.StoreID}
        if b.Created.Valid { out["created"] = b.Created.Time.Format(time.RFC3339Nano) }
        if b.Updated.Valid { out["updated"] = b.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    BlackboardCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagBBID, "id", "", "Blackboard UUID (optional; when omitted, a new id is generated)")
    setCmd.Flags().StringVar(&flagBBRole, "role", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagBBStoreID, "store-id", "", "Store UUID (required on create)")
    setCmd.Flags().StringVar(&flagBBConvID, "conversation", "", "Conversation UUID (optional)")
    setCmd.Flags().StringVar(&flagBBProject, "project", "", "Project name (optional; must exist for role)")
    setCmd.Flags().StringVar(&flagBBTaskID, "task", "", "Task UUID (optional)")
    setCmd.Flags().StringVar(&flagBBBackground, "background", "", "Background text")
    setCmd.Flags().StringVar(&flagBBGuidelines, "guidelines", "", "Guidelines text")
}

