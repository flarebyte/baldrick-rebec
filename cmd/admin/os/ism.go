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

var ismCmd = &cobra.Command{
    Use:   "ism",
    Short: "Manage OpenSearch ISM policies",
}

var (
    flagISMName       string
    flagISMPolicyFile string
    flagISMAttachIdx  []string
    flagISMForce      bool
    flagISMDryRun     bool
)

var ismEnsureCmd = &cobra.Command{
    Use:   "ensure",
    Short: "Ensure an ISM policy exists (optionally attach to indexes)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagISMName == "" { flagISMName = "messages-content-ilm" }
        var policy map[string]interface{}
        if flagISMPolicyFile != "" {
            b, err := os.ReadFile(flagISMPolicyFile)
            if err != nil { return err }
            if err := json.Unmarshal(b, &policy); err != nil { return fmt.Errorf("invalid policy json: %w", err) }
        } else {
            policy = defaultISMPolicy()
        }
        if flagISMDryRun {
            fmt.Fprintf(os.Stderr, "ISM(dry-run): would ensure policy %q\n", flagISMName)
            if len(flagISMAttachIdx) > 0 {
                fmt.Fprintf(os.Stderr, "ISM(dry-run): would attach policy %q to indexes: %v\n", flagISMName, flagISMAttachIdx)
            }
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(map[string]interface{}{"policy": policy})
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        if err := client.EnsureISMPolicy(ctx, flagISMName, policy, flagISMForce); err != nil { return err }
        for _, idx := range flagISMAttachIdx {
            if err := client.AttachISMToIndex(ctx, idx, flagISMName); err != nil { return fmt.Errorf("attach ISM to index %s: %w", idx, err) }
        }
        fmt.Fprintf(os.Stderr, "ISM: policy %q ensured\n", flagISMName)
        return nil
    },
}

var ismShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Show an ISM policy JSON",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagISMName == "" { flagISMName = "messages-content-ilm" }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        raw, err := client.GetISMPolicy(ctx, flagISMName)
        if err != nil { return err }
        var m map[string]interface{}
        if err := json.Unmarshal(raw, &m); err == nil {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(m)
        }
        os.Stdout.Write(raw)
        if len(raw) == 0 || raw[len(raw)-1] != '\n' { fmt.Fprintln(os.Stdout) }
        return nil
    },
}

var ismListCmd = &cobra.Command{
    Use:   "list",
    Short: "List ISM policies",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        raw, err := client.ListISMPolicies(ctx)
        if err != nil { return err }
        var out map[string]interface{}
        if err := json.Unmarshal(raw, &out); err == nil {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(out)
        }
        os.Stdout.Write(raw)
        if len(raw) == 0 || raw[len(raw)-1] != '\n' { fmt.Fprintln(os.Stdout) }
        return nil
    },
}

var (
    flagISMYes bool
)

var ismDeleteCmd = &cobra.Command{
    Use:   "delete",
    Short: "Delete an ISM policy (requires --yes)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagISMName == "" { flagISMName = "messages-content-ilm" }
        if !flagISMYes { return fmt.Errorf("refusing to delete ISM policy %q without --yes", flagISMName) }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        if err := client.DeleteISMPolicy(ctx, flagISMName); err != nil { return err }
        fmt.Fprintf(os.Stderr, "ISM: policy %q deleted\n", flagISMName)
        return nil
    },
}

func init() {
    OSCmd.AddCommand(ismCmd)
    ismCmd.AddCommand(ismEnsureCmd)
    ismCmd.AddCommand(ismShowCmd)
    ismCmd.AddCommand(ismListCmd)
    ismCmd.AddCommand(ismDeleteCmd)

    ismEnsureCmd.Flags().StringVar(&flagISMName, "name", "messages-content-ilm", "ISM policy name")
    ismEnsureCmd.Flags().StringVar(&flagISMPolicyFile, "policy-file", "", "Path to ISM policy JSON file (under 'policy')")
    ismEnsureCmd.Flags().StringSliceVar(&flagISMAttachIdx, "attach-to-index", nil, "Indexes to attach the ISM policy to")
    ismEnsureCmd.Flags().BoolVar(&flagISMForce, "force", false, "Recreate policy if it exists")
    ismEnsureCmd.Flags().BoolVar(&flagISMDryRun, "dry-run", false, "Show actions without applying")

    ismShowCmd.Flags().StringVar(&flagISMName, "name", "messages-content-ilm", "ISM policy name")
    ismDeleteCmd.Flags().StringVar(&flagISMName, "name", "messages-content-ilm", "ISM policy name")
    ismDeleteCmd.Flags().BoolVar(&flagISMYes, "yes", false, "Confirm deletion")
}

