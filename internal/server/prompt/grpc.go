package prompt

import (
    "context"
    "fmt"
    "strings"

    toolingdao "github.com/flarebyte/baldrick-rebec/internal/dao/tooling"
    responsesvc "github.com/flarebyte/baldrick-rebec/internal/service/responses"
    factorypkg "github.com/flarebyte/baldrick-rebec/internal/service/responses/factory"
    grpcjson "github.com/flarebyte/baldrick-rebec/internal/transport/grpcjson"
    "google.golang.org/grpc"
)

// Service wires DAOs and factories to serve Prompt RPCs.
type Service struct {
    ToolDAO          toolingdao.ToolDAO
    VaultDAO         toolingdao.VaultDAO
    LLMFactory       factorypkg.LLMFactory
    ResponsesService responsesvc.ResponsesService
}

// Register registers the service on the provided gRPC server.
func (s *Service) Register(grpcServer *grpc.Server) {
    // ensure codec registered once
    grpcjson.Register()
    grpcServer.RegisterService(&grpc.ServiceDesc{
        ServiceName: "prompt.v1.PromptService",
        HandlerType: (*Service)(nil),
        Methods: []grpc.MethodDesc{
            {MethodName: "Run", Handler: s.handleRun},
        },
        Streams:  []grpc.StreamDesc{},
        Metadata: "proto/prompt/v1/prompt.proto",
    }, s)
}

// handleRun is the unary handler for Run.
func (s *Service) handleRun(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
    in := new(PromptRunRequest)
    if err := dec(in); err != nil {
        return nil, err
    }
    h := func(ctx context.Context, req any) (any, error) {
        return s.Run(ctx, req.(*PromptRunRequest))
    }
    if interceptor == nil {
        return h(ctx, in)
    }
    info := &grpc.UnaryServerInfo{
        Server:     srv,
        FullMethod: "/prompt.v1.PromptService/Run",
    }
    return interceptor(ctx, in, info, h)
}

// Run executes the prompt using local DAOs and services and returns a response.
func (s *Service) Run(ctx context.Context, req *PromptRunRequest) (*PromptRunResponse, error) {
    if s.ToolDAO == nil || s.LLMFactory == nil || s.ResponsesService == nil {
        return nil, fmt.Errorf("prompt service not initialized")
    }
    if req.ToolName == "" {
        return nil, fmt.Errorf("tool_name is required")
    }
    cfg, err := s.ToolDAO.GetToolByName(ctx, req.ToolName)
    if err != nil {
        return nil, err
    }
    var secret *toolingdao.SecretMetadata
    if cfg.APIKeySecret != "" && s.VaultDAO != nil {
        if secret, err = s.VaultDAO.GetSecretMetadata(ctx, cfg.APIKeySecret); err != nil {
            return nil, err
        }
    }
    // Build LLM
    facCfg := &factorypkg.ToolConfig{
        Name:            cfg.Name,
        Provider:        factorypkg.ProviderType(cfg.Provider),
        Model:           firstNonEmpty(req.Model, cfg.Model),
        BaseURL:         cfg.BaseURL,
        APIKeySecret:    cfg.APIKeySecret,
        Temperature:     cfg.Temperature,
        MaxOutputTokens: cfg.MaxOutputTokens,
        TopP:            cfg.TopP,
        Settings:        cfg.Settings,
    }
    facSecret := &factorypkg.SecretMetadata{}
    if secret != nil { facSecret.Value = secret.Value }
    llm, err := s.LLMFactory.NewLLM(ctx, facCfg, facSecret)
    if err != nil {
        return nil, err
    }
    // Translate tools
    tools := make([]responsesvc.ToolDefinition, 0, len(req.Tools))
    for _, t := range req.Tools {
        tools = append(tools, responsesvc.ToolDefinition{
            Type: t.Type,
            Name: t.Function.Name,
            Parameters: t.Function.Parameters,
        })
    }
    // Build request for service
    svcReq := &responsesvc.ResponseRequest{
        Model:        firstNonEmpty(req.Model, cfg.Model),
        Input:        req.Input,
        Temperature:  cfg.Temperature,
        Tools:        tools,
    }
    if req.Temperature != 0 {
        v := req.Temperature
        svcReq.Temperature = &v
    }
    if req.MaxOutputTokens != 0 {
        v := int(req.MaxOutputTokens)
        svcReq.MaxOutputTokens = &v
    } else if cfg.MaxOutputTokens != nil {
        svcReq.MaxOutputTokens = cfg.MaxOutputTokens
    }
    // Call service
    svcCfg := &responsesvc.ToolConfig{
        Name:               cfg.Name,
        Provider:           string(cfg.Provider),
        Model:              firstNonEmpty(req.Model, cfg.Model),
        APIKeySecret:       cfg.APIKeySecret,
        Settings:           cfg.Settings,
        DefaultTemperature: cfg.Temperature,
        DefaultMaxTokens:   cfg.MaxOutputTokens,
        DefaultTopP:        cfg.TopP,
    }
    out, err := s.ResponsesService.CreateResponse(ctx, svcCfg, svcReq, llm)
    if err != nil {
        return nil, err
    }
    // Map to response
    resp := &PromptRunResponse{
        Id:      out.ID,
        Object:  out.Object,
        Model:   out.Model,
        Created: out.Created,
        Usage:   &Usage{},
    }
    if out.Usage != nil {
        resp.Usage = &Usage{InputTokens: int32(out.Usage.InputTokens), OutputTokens: int32(out.Usage.OutputTokens), TotalTokens: int32(out.Usage.TotalTokens)}
    }
    for _, b := range out.Output {
        var tb *ToolCall
        if b.ToolCall != nil {
            tb = &ToolCall{Name: b.ToolCall.Name, Arguments: b.ToolCall.Arguments}
        }
        resp.Output = append(resp.Output, ContentBlock{Type: b.Type, Text: b.Text, ToolCall: tb})
    }
    return resp, nil
}

func firstNonEmpty(a, b string) string {
    if strings.TrimSpace(a) != "" { return a }
    return b
}
