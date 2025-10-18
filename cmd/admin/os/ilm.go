package oscmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    osdao "github.com/flarebyte/baldrick-rebec/internal/dao/opensearch"
    "github.com/spf13/cobra"
)

var (
    flagILMName       string
    flagPolicyFile    string
    flagAttachIndexes []string
    flagForceILM      bool
    flagDryRunILM     bool
)

var ilmCmd = &cobra.Command{
    Use:   "ilm",
    Short: "Manage OpenSearch ILM policies",
}

var ensureCmd = &cobra.Command{
    Use:   "ensure",
    Short: "Ensure an ILM policy exists (optionally attach to indexes)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagILMName == "" { flagILMName = "messages-content-ilm" }

        // Build policy body
        var policy map[string]interface{}
        if flagPolicyFile != "" {
            b, err := os.ReadFile(flagPolicyFile)
            if err != nil { return err }
            if err := json.Unmarshal(b, &policy); err != nil {
                return fmt.Errorf("invalid policy json: %w", err)
            }
        } else {
            policy = defaultPolicy()
        }

        // Dry-run output
        if flagDryRunILM {
            fmt.Fprintf(os.Stderr, "ILM(dry-run): would ensure policy %q\n", flagILMName)
            if len(flagAttachIndexes) > 0 {
                fmt.Fprintf(os.Stderr, "ILM(dry-run): would attach policy %q to indexes: %v\n", flagILMName, flagAttachIndexes)
            }
            // Print policy to stdout for visibility
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            _ = enc.Encode(policy)
            return nil
        }

        // Real operation
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        client := osdao.NewClientFromConfigAdmin(cfg)
        if err := client.EnsureILMPolicy(ctx, flagILMName, policy, flagForceILM); err != nil {
            return err
        }
        fmt.Fprintf(os.Stderr, "ILM: policy %q ensured\n", flagILMName)

        for _, idx := range flagAttachIndexes {
            if err := client.AttachILMToIndex(ctx, idx, flagILMName); err != nil {
                return fmt.Errorf("attach policy to index %s: %w", idx, err)
            }
            fmt.Fprintf(os.Stderr, "ILM: policy %q attached to index %q\n", flagILMName, idx)
        }
        return nil
    },
}

func defaultPolicy() map[string]interface{} {
    return map[string]interface{}{
        "policy": map[string]interface{}{
            "phases": map[string]interface{}{
                "hot": map[string]interface{}{
                    "actions": map[string]interface{}{
                        "rollover": map[string]interface{}{
                            "max_primary_shard_size": "50gb",
                            "max_age":                 "30d",
                        },
                    },
                },
                "warm": map[string]interface{}{
                    "min_age": "60d",
                    "actions": map[string]interface{}{
                        "forcemerge": map[string]interface{}{
                            "max_num_segments": 1,
                        },
                    },
                },
                "delete": map[string]interface{}{
                    "min_age": "180d",
                    "actions": map[string]interface{}{
                        "delete": map[string]interface{}{},
                    },
                },
            },
        },
    }
}

func init() {
    ilmCmd.AddCommand(ensureCmd)

    ensureCmd.Flags().StringVar(&flagILMName, "name", "messages-content-ilm", "ILM policy name")
    ensureCmd.Flags().StringVar(&flagPolicyFile, "policy-file", "", "Path to ILM policy JSON file")
    ensureCmd.Flags().StringSliceVar(&flagAttachIndexes, "attach-to-index", nil, "Indexes to attach the ILM policy to")
    ensureCmd.Flags().BoolVar(&flagForceILM, "force", false, "Overwrite existing ILM policy if it exists")
    ensureCmd.Flags().BoolVar(&flagDryRunILM, "dry-run", false, "Show actions and policy without applying changes")

    ilmCmd.AddCommand(showCmd)
    ilmCmd.AddCommand(deleteCmd)
}

var showCmd = &cobra.Command{
    Use:   "show",
    Short: "Show the JSON of an ILM policy",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagILMName == "" { flagILMName = "messages-content-ilm" }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        raw, err := client.GetILMPolicy(ctx, flagILMName)
        if err != nil { return err }
        // Pretty-print
        var m map[string]interface{}
        if err := json.Unmarshal(raw, &m); err == nil {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(m)
        }
        // Fallback to raw
        os.Stdout.Write(raw)
        if len(raw) == 0 || raw[len(raw)-1] != '\n' { fmt.Fprintln(os.Stdout) }
        return nil
    },
}

var (
    flagYesDelete bool
)

var deleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete an ILM policy (requires --yes)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagILMName == "" { flagILMName = "messages-content-ilm" }
        if !flagYesDelete {
            return fmt.Errorf("refusing to delete ILM policy %q without --yes", flagILMName)
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        if err := client.DeleteILMPolicy(ctx, flagILMName); err != nil { return err }
        fmt.Fprintf(os.Stderr, "ILM: policy %q deleted\n", flagILMName)
        return nil
    },
}

func init() {
    showCmd.Flags().StringVar(&flagILMName, "name", "messages-content-ilm", "ILM policy name")
    deleteCmd.Flags().StringVar(&flagILMName, "name", "messages-content-ilm", "ILM policy name")
    deleteCmd.Flags().BoolVar(&flagYesDelete, "yes", false, "Confirm deletion")
}
