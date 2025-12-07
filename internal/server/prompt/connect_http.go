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
            http.Error(w, "invalid JSON", http.StatusBadRequest)
            return
        }
        resp, err := s.Run(r.Context(), &req)
        if err != nil {
            // For simplicity, surface as 500 with error text. Connect typically encodes structured errors.
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/connect+json")
        enc := json.NewEncoder(w)
        if err := enc.Encode(resp); err != nil {
            http.Error(w, "encode error", http.StatusInternalServerError)
            return
        }
    })
}

