package stickie

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
    flagStFindName      string
    flagStFindVariant   string
    flagStFindArchived  bool
    flagStFindBlackboard string
)

var findCmd = &cobra.Command{
    Use:   "find",
    Short: "Find a single stickie by complex name (name + variant)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagStFindName) == "" {
            return errors.New("--name is required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        var s *pgdao.Stickie
        if strings.TrimSpace(flagStFindBlackboard) != "" {
            s, err = pgdao.GetStickieByComplexNameInBlackboard(ctx, db, flagStFindName, flagStFindVariant, flagStFindArchived, flagStFindBlackboard)
        } else {
            s, err = pgdao.GetStickieByComplexName(ctx, db, flagStFindName, flagStFindVariant, flagStFindArchived)
        }
        if err != nil { return err }
        // stderr summary
        fmt.Fprintf(os.Stderr, "stickie id=%s board=%s name=%q variant=%q archived=%t\n", s.ID, s.BlackboardID, s.ComplexName.Name, s.ComplexName.Variant, s.Archived)
        // stdout JSON
        out := map[string]any{
            "id": s.ID,
            "blackboard_id": s.BlackboardID,
            "name": s.ComplexName.Name,
            "variant": s.ComplexName.Variant,
            "archived": s.Archived,
            "edit_count": s.EditCount,
        }
        if s.TopicName.Valid { out["topic_name"] = s.TopicName.String }
        if s.TopicRoleName.Valid { out["topic_role_name"] = s.TopicRoleName.String }
        if s.Score.Valid { out["score"] = s.Score.Float64 }
        if s.Updated.Valid { out["updated"] = s.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    StickieCmd.AddCommand(findCmd)
    findCmd.Flags().StringVar(&flagStFindName, "name", "", "Complex name: name (required)")
    findCmd.Flags().StringVar(&flagStFindVariant, "variant", "", "Complex name: variant (optional; default empty)")
    findCmd.Flags().BoolVar(&flagStFindArchived, "archived", false, "Search archived stickies instead of active ones")
    findCmd.Flags().StringVar(&flagStFindBlackboard, "blackboard", "", "Optional blackboard UUID scope filter")
}
