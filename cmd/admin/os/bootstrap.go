package oscmd

import (
    "context"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    osdao "github.com/flarebyte/baldrick-rebec/internal/dao/opensearch"
    "github.com/flarebyte/baldrick-rebec/internal/paths"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

var (
    flagBSHost     string
    flagBSPort     int
    flagBSInsecure bool
    flagBSAdminUser string
    flagBSAdminPassTemp string
    flagBSAttachIndex string
)

// bootstrapCmd configures local secure OpenSearch defaults in config.yaml and ensures ILM.
var bootstrapCmd = &cobra.Command{
    Use:   "bootstrap",
    Short: "Configure secure OpenSearch in config and ensure ILM policy",
    RunE: func(cmd *cobra.Command, args []string) error {
        if _, err := paths.EnsureHome(); err != nil { return err }
        cfg, _ := cfgpkg.Load()

        // Default to https on localhost with self-signed cert
        if flagBSHost == "" { flagBSHost = "127.0.0.1" }
        if flagBSPort == 0 { flagBSPort = cfgpkg.DefaultOpenSearchPort }
        cfg.OpenSearch.Scheme = "https"
        cfg.OpenSearch.Host = flagBSHost
        cfg.OpenSearch.Port = flagBSPort
        cfg.OpenSearch.InsecureSkipVerify = true

        // Resolve admin creds, prefer flag then env var
        if flagBSAdminUser == "" { flagBSAdminUser = "admin" }
        if flagBSAdminPassTemp == "" {
            flagBSAdminPassTemp = os.Getenv("OPENSEARCH_INITIAL_ADMIN_PASSWORD")
        }
        if flagBSAdminPassTemp == "" {
            return fmt.Errorf("admin temporary password not provided; set --admin-password-temp or OPENSEARCH_INITIAL_ADMIN_PASSWORD")
        }

        cfg.OpenSearch.Admin.Username = flagBSAdminUser
        cfg.OpenSearch.Admin.PasswordTemp = flagBSAdminPassTemp

        // Write config
        b, err := yaml.Marshal(cfg)
        if err != nil { return err }
        if err := os.WriteFile(cfgpkg.Path(), b, 0o644); err != nil { return err }
        fmt.Fprintf(os.Stderr, "bootstrap: wrote OpenSearch secure settings to %s\n", cfgpkg.Path())

        // Ensure ILM and attach to messages_content by default
        policyName := "messages-content-ilm"
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        client := osdao.NewClientFromConfigAdmin(cfg)
        // Try ILM first; if endpoint not found, fallback to ISM
        if err := client.EnsureILMPolicy(ctx, policyName, defaultPolicy(), false); err != nil {
            // Fallback to ISM
            fmt.Fprintln(os.Stderr, "bootstrap: ILM API not available; falling back to ISM policy")
            ismPolicy := defaultISMPolicy()
            if err2 := client.EnsureISMPolicy(ctx, policyName, ismPolicy, true); err2 != nil {
                return fmt.Errorf("ensure ISM policy: %w", err2)
            }
            idx := flagBSAttachIndex
            if idx == "" { idx = "messages_content" }
            if err2 := client.AttachISMToIndex(ctx, idx, policyName); err2 != nil {
                return fmt.Errorf("attach ISM policy to %s: %w", idx, err2)
            }
            fmt.Fprintln(os.Stderr, "bootstrap: ISM ensured and attached to index")
            return nil
        }
        idx := flagBSAttachIndex
        if idx == "" { idx = "messages_content" }
        if err := client.AttachILMToIndex(ctx, idx, policyName); err != nil {
            return fmt.Errorf("attach ILM policy to %s: %w", idx, err)
        }
        fmt.Fprintln(os.Stderr, "bootstrap: ILM ensured and attached to index")

        return nil
    },
}

func init() {
    OSCmd.AddCommand(bootstrapCmd)
    bootstrapCmd.Flags().StringVar(&flagBSHost, "host", "127.0.0.1", "OpenSearch host")
    bootstrapCmd.Flags().IntVar(&flagBSPort, "port", cfgpkg.DefaultOpenSearchPort, "OpenSearch port")
    bootstrapCmd.Flags().BoolVar(&flagBSInsecure, "insecure", true, "Skip TLS verification (self-signed)")
    bootstrapCmd.Flags().StringVar(&flagBSAdminUser, "admin-username", "admin", "Admin username")
    bootstrapCmd.Flags().StringVar(&flagBSAdminPassTemp, "admin-password-temp", "", "Admin temporary password (or set OPENSEARCH_INITIAL_ADMIN_PASSWORD)")
    bootstrapCmd.Flags().StringVar(&flagBSAttachIndex, "attach-index", "messages_content", "Index to attach the ILM policy to")
}

// defaultISMPolicy returns a simple rollover+delete policy for OpenSearch ISM.
func defaultISMPolicy() map[string]interface{} {
    // This ISM policy approximates the ILM policy behavior with rollover at ~30d or size and delete at 180d.
    return map[string]interface{}{
        "description": "messages content rollover+retention",
        "default_state": "hot",
        "states": []map[string]interface{}{
            {
                "name": "hot",
                "actions": []map[string]interface{}{
                    {"rollover": map[string]interface{}{
                        "min_index_age": "30d",
                        "min_primary_shard_size": "50gb",
                    }},
                },
                "transitions": []map[string]interface{}{
                    {"state_name": "delete", "conditions": map[string]interface{}{"min_index_age": "180d"}},
                },
            },
            {
                "name": "delete",
                "actions": []map[string]interface{}{{"delete": map[string]interface{}{}}},
                "transitions": []map[string]interface{}{},
            },
        },
    }
}
