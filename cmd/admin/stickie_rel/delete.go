package stickie_rel

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
    flagRelDelFrom string
    flagRelDelTo   string
    flagRelDelType string
    flagRelDelForce bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a stickie relationship (from,to,type)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRelDelFrom) == "" || strings.TrimSpace(flagRelDelTo) == "" || strings.TrimSpace(flagRelDelType) == "" {
            return errors.New("--from, --to and --type are required")
        }
        if !flagRelDelForce {
            fmt.Fprintf(os.Stderr, "About to delete relation %s -[%s]-> %s\n", flagRelDelFrom, flagRelDelType, flagRelDelTo)
            fmt.Fprint(os.Stderr, "Type YES to confirm: ")
            rd := bufio.NewReader(os.Stdin)
            line, _ := rd.ReadString('\n')
            if strings.TrimSpace(strings.ToUpper(line)) != "YES" { return errors.New("confirmation failed") }
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        n, err := pgdao.DeleteStickieEdge(ctx, db, flagRelDelFrom, flagRelDelTo, flagRelDelType)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "relations deleted: %d\n", n)
        out := map[string]any{"status":"deleted","from":flagRelDelFrom,"to":flagRelDelTo,"type":flagRelDelType,"deleted":n}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    StickieRelCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagRelDelFrom, "from", "", "From stickie UUID (required)")
    deleteCmd.Flags().StringVar(&flagRelDelTo, "to", "", "To stickie UUID (required)")
    deleteCmd.Flags().StringVar(&flagRelDelType, "type", "", "Relation type: includes|causes|uses|represents|contrasts_with")
    deleteCmd.Flags().BoolVar(&flagRelDelForce, "force", false, "Skip confirmation prompt")
}

