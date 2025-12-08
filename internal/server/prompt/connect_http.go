package prompt

import (
    "encoding/json"
    "net/http"
)

// ConnectHandler returns an http.Handler that serves the PromptService.Run
// method using the Connect protocol with JSON encoding (application/connect+json).
// This is a minimal, hand-rolled handler that bridges to the same Service.Run logic.
func (s *Service) ConnectHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Only handle POST /prompt.v1.PromptService/Run
        if r.Method != http.MethodPost || r.URL.Path != "/prompt.v1.PromptService/Run" {
            http.NotFound(w, r)
            return
        }
        // Decode proto-JSON body into our request struct
        var req PromptRunRequest
        dec := json.NewDecoder(r.Body)
        if err := dec.Decode(&req); err != nil {
            writeConnectError(w, "invalid_argument", "invalid JSON body")
            return
        }
        resp, err := s.Run(r.Context(), &req)
        if err != nil {
            code, msg := mapError(err)
            writeConnectError(w, code, msg)
            return
        }
        writeConnectJSON(w, resp)
    })
}

// writeConnectJSON writes a successful Connect JSON response.
func writeConnectJSON(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/connect+json")
    w.Header().Set("Connect-Protocol-Version", "1")
    _ = json.NewEncoder(w).Encode(v)
}

// writeConnectError writes a Connect-style JSON error envelope with protocol header.
func writeConnectError(w http.ResponseWriter, code, message string) {
    w.Header().Set("Content-Type", "application/connect+json")
    w.Header().Set("Connect-Protocol-Version", "1")
    w.Header().Set("Connect-Error-Code", code)
    // Per Connect protocol, unary errors can be 200 with error body.
    // We choose 200 to maximize compatibility with clients.
    _ = json.NewEncoder(w).Encode(map[string]any{
        "error": map[string]any{
            "code":    code,
            "message": message,
            "details": []any{},
        },
    })
}
