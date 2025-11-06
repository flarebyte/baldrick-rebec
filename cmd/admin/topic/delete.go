package topic

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
    flagTopicDelName          string
    flagTopicDelRole          string
    flagTopicDelForce         bool
    flagTopicDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete a topic by name and role (asks for confirmation unless --force)",
    RunE: func(cmd *cobra.Command, args []string) error {
        name := strings.TrimSpace(flagTopicDelName)
        role := strings.TrimSpace(flagTopicDelRole)
        if name == "" { return errors.New("--name is required") }
        if role == "" { return errors.New("--role is required") }
        if !flagTopicDelForce {
            fmt.Fprintf(os.Stderr, "About to delete topic %q for role %q.\n", name, role)
            fmt.Fprint(os.Stderr, "Type the topic name to confirm: ")
            reader := bufio.NewReader(os.Stdin)
            line, _ := reader.ReadString('\n')
            if strings.TrimSpace(line) != name {
                return errors.New("confirmation did not match; aborting")
            }
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        affected, err := pgdao.DeleteTopic(ctx, db, name, role)
        if err != nil { return err }
        if affected == 0 {
            if flagTopicDelIgnoreMissing {
                fmt.Fprintf(os.Stderr, "topic %q (role=%q) not found; ignoring\n", name, role)
                out := map[string]any{"status":"not_found_ignored","name":name,"role":role}
                enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
            }
            return fmt.Errorf("topic %q (role=%q) not found", name, role)
        }
        fmt.Fprintf(os.Stderr, "topic deleted name=%q role=%q\n", name, role)
        out := map[string]any{"status":"deleted","name":name,"role":role,"deleted":true}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    TopicCmd.AddCommand(deleteCmd)
    deleteCmd.Flags().StringVar(&flagTopicDelName, "name", "", "Topic unique name within role (required)")
    deleteCmd.Flags().StringVar(&flagTopicDelRole, "role", "", "Role name (required)")
    deleteCmd.Flags().BoolVar(&flagTopicDelForce, "force", false, "Do not prompt for confirmation")
    deleteCmd.Flags().BoolVar(&flagTopicDelIgnoreMissing, "ignore-missing", false, "Do not error if topic does not exist")
}

