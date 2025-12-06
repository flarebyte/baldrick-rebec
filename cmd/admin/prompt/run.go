package prompt

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
    toolingdao "github.com/flarebyte/baldrick-rebec/internal/dao/tooling"
    responsesvc "github.com/flarebyte/baldrick-rebec/internal/service/responses"
    factorypkg "github.com/flarebyte/baldrick-rebec/internal/service/responses/factory"
    "github.com/spf13/cobra"
)

var (
    flagToolName       string
    flagInput          string
    flagInputFile      string
    flagToolsPath      string
    flagTemperature    float32
    flagHasTemperature bool
    flagMaxOutTokens   int
    flagHasMaxTokens   bool
    flagJSON           bool
)

var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Run a single prompt against a configured tool",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Attempt to initialize a Postgres-backed ToolDAO from app config; fall back to mocks.
        if cfg, err := cfgpkg.Load(); err == nil {
            ctxInit, cancelInit := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancelInit()
            if db, e := pgdao.OpenApp(ctxInit, cfg); e == nil {
                // We intentionally do not keep the DB open after command finishes.
                // The adapter holds the pool; ensure it's closed on process exit.
                deps.ToolDAO = toolingdao.NewPGToolDAOAdapter(db)
            }
        }
        ensureDefaults()
        if strings.TrimSpace(flagToolName) == "" {
            return errors.New("--tool-name is required")
        }
        // Track flag presence for overriding defaults
        flagHasTemperature = cmd.Flags().Changed("temperature")
        flagHasMaxTokens = cmd.Flags().Changed("max-output-tokens")

        // Load input
        input, err := readInput(flagInput, flagInputFile)
        if err != nil {
            return err
        }
        // Load tools file if provided
        var tools []responsesvc.ToolDefinition
        if strings.TrimSpace(flagToolsPath) != "" {
            data, err := os.ReadFile(flagToolsPath)
            if err != nil {
                return fmt.Errorf("read tools file: %w", err)
            }
            if err := json.Unmarshal(data, &tools); err != nil {
                return fmt.Errorf("parse tools JSON: %w", err)
            }
        }

        // Resolve tool configuration
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        cfg, err := deps.ToolDAO.GetToolByName(ctx, flagToolName)
        if err != nil {
            if errors.Is(err, toolingdao.ErrToolNotFound) {
                return fmt.Errorf("tool %q not found", flagToolName)
            }
            return err
        }

        // Resolve secret if required
        var secret *toolingdao.SecretMetadata
        if strings.TrimSpace(cfg.APIKeySecret) != "" {
            s, err := deps.VaultDAO.GetSecretMetadata(ctx, cfg.APIKeySecret)
            if err != nil {
                return err
            }
            secret = s
        }

        // Build LLM
        llmCfg := convertToFactoryConfig(cfg)
        facSecret := &factorypkg.SecretMetadata{}
        if secret != nil { facSecret.Value = secret.Value }
        llm, err := deps.LLMFactory.NewLLM(ctx, llmCfg, facSecret)
        if err != nil {
            return err
        }

        // Build request
        req := &responsesvc.ResponseRequest{
            Model: cfg.Model,
            Input: input,
            Tools: tools,
        }
        if flagHasTemperature {
            req.Temperature = &flagTemperature
        } else if cfg.Temperature != nil {
            req.Temperature = cfg.Temperature
        }
        if flagHasMaxTokens {
            req.MaxOutputTokens = &flagMaxOutTokens
        } else if cfg.MaxOutputTokens != nil {
            req.MaxOutputTokens = cfg.MaxOutputTokens
        }

        // Call service
        svcCfg := convertToServiceConfig(cfg)
        resp, err := deps.ResponsesService.CreateResponse(ctx, svcCfg, req, llm)
        if err != nil {
            return err
        }

        // Output
        if flagJSON {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(resp)
        }
        // Print only text outputs and tool-call blocks
        printCompact(resp)
        return nil
    },
}

func init() {
    PromptCmd.AddCommand(runCmd)
    runCmd.Flags().StringVar(&flagToolName, "tool-name", "", "Tool name (required)")
    runCmd.Flags().StringVar(&flagInput, "input", "", "Input text (optional)")
    runCmd.Flags().StringVar(&flagInputFile, "input-file", "", "Input file path (optional)")
    runCmd.Flags().StringVar(&flagToolsPath, "tools", "", "Path to JSON tool definitions (optional)")
    runCmd.Flags().BoolVar(&flagJSON, "json", false, "Print full JSON response")
    runCmd.Flags().Float32Var(&flagTemperature, "temperature", 0, "Sampling temperature")
    runCmd.Flags().IntVar(&flagMaxOutTokens, "max-output-tokens", 0, "Max output tokens")
    // Track whether flags were explicitly set
    runCmd.Flags().Lookup("temperature").NoOptDefVal = "0"
    runCmd.Flags().Lookup("temperature").DefValue = ""
    runCmd.Flags().Lookup("max-output-tokens").NoOptDefVal = "0"
}

func readInput(inline, path string) (any, error) {
    if strings.TrimSpace(inline) != "" {
        return inline, nil
    }
    if strings.TrimSpace(path) != "" {
        b, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("read input file: %w", err)
        }
        return string(b), nil
    }
    return nil, errors.New("one of --input or --input-file is required")
}

func convertToFactoryConfig(in *toolingdao.ToolConfig) *factorypkg.ToolConfig {
    if in == nil { return nil }
    return &factorypkg.ToolConfig{
        Name:            in.Name,
        Provider:        factorypkg.ProviderType(in.Provider),
        Model:           in.Model,
        BaseURL:         in.BaseURL,
        APIKeySecret:    in.APIKeySecret,
        Temperature:     in.Temperature,
        MaxOutputTokens: in.MaxOutputTokens,
        TopP:            in.TopP,
        Settings:        in.Settings,
    }
}

func convertToServiceConfig(in *toolingdao.ToolConfig) *responsesvc.ToolConfig {
    if in == nil { return nil }
    return &responsesvc.ToolConfig{
        Name:               in.Name,
        Provider:           string(in.Provider),
        Model:              in.Model,
        APIKeySecret:       in.APIKeySecret,
        Settings:           in.Settings,
        DefaultTemperature: in.Temperature,
        DefaultMaxTokens:   in.MaxOutputTokens,
        DefaultTopP:        in.TopP,
    }
}

func printCompact(resp *responsesvc.Response) {
    for _, b := range resp.Output {
        switch strings.ToLower(b.Type) {
        case "output_text":
            fmt.Println(b.Text)
        case "tool_call":
            if b.ToolCall != nil {
                enc, _ := json.Marshal(b.ToolCall)
                fmt.Printf("[tool_call] %s\n", string(enc))
            }
        }
    }
}
