package testcase

import (
    "encoding/json"
    "net/http"
    "time"

    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
)

// Connect-style JSON handler that routes by path.
func (s *Service) ConnectHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/testcase.v1.TestcaseService/Create":
            s.httpCreate(w, r)
        case "/testcase.v1.TestcaseService/List":
            s.httpList(w, r)
        case "/testcase.v1.TestcaseService/Delete":
            s.httpDelete(w, r)
        default:
            http.NotFound(w, r)
        }
    })
}

func (s *Service) httpCreate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { http.NotFound(w, r); return }
    var in CreateTestcaseRequest
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeErr(w, "invalid_argument", "invalid JSON"); return }
    if s.DB == nil { writeErr(w, "internal", "service not initialized"); return }
    tc := &pgdao.Testcase{Title: in.Title, RoleName: orDefault(in.Role, "user"), Status: orDefault(in.Status, "OK")}
    if in.Name != "" { tc.Name = sqlString(in.Name) }
    if in.Package != "" { tc.Package = sqlString(in.Package) }
    if in.Classname != "" { tc.Classname = sqlString(in.Classname) }
    if in.Experiment != "" { tc.ExperimentID = sqlString(in.Experiment) }
    if in.ErrorMessage != "" { tc.ErrorMessage = sqlString(in.ErrorMessage) }
    if len(in.Tags) > 0 { tc.Tags = in.Tags }
    if in.Level != "" { tc.Level = sqlString(in.Level) }
    if in.File != "" { tc.File = sqlString(in.File) }
    if in.Line > 0 { tc.Line = sqlInt64(int64(in.Line)) }
    if in.ExecutionTime > 0 { tc.ExecutionTime = sqlFloat64(in.ExecutionTime) }
    if err := pgdao.InsertTestcase(r.Context(), s.DB, tc); err != nil { writeErr(w, "internal", err.Error()); return }
    out := CreateTestcaseResponse{ID: tc.ID, Title: tc.Title, Status: tc.Status}
    if tc.Created.Valid { out.Created = tc.Created.Time.Format(time.RFC3339Nano) }
    writeOK(w, out)
}

func (s *Service) httpList(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { http.NotFound(w, r); return }
    var in ListTestcasesRequest
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeErr(w, "invalid_argument", "invalid JSON"); return }
    if s.DB == nil { writeErr(w, "internal", "service not initialized"); return }
    items, err := pgdao.ListTestcases(r.Context(), s.DB, in.Role, in.Experiment, in.Status, int(in.Limit), int(in.Offset))
    if err != nil { writeErr(w, "internal", err.Error()); return }
    var resp ListTestcasesResponse
    for _, t := range items {
        it := TestcaseItem{ID: t.ID, Title: t.Title, Status: t.Status}
        if t.Created.Valid { it.Created = t.Created.Time.Format(time.RFC3339Nano) }
        if t.Name.Valid { it.Name = t.Name.String }
        if t.Package.Valid { it.Package = t.Package.String }
        if t.Classname.Valid { it.Classname = t.Classname.String }
        if t.ExperimentID.Valid { it.ExperimentID = t.ExperimentID.String }
        if len(t.Tags) > 0 { it.Tags = t.Tags }
        if t.File.Valid { it.File = t.File.String }
        if t.Line.Valid { it.Line = int32(t.Line.Int64) }
        if t.ExecutionTime.Valid { it.Execution = t.ExecutionTime.Float64 }
        resp.Items = append(resp.Items, it)
    }
    writeOK(w, resp)
}

func (s *Service) httpDelete(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { http.NotFound(w, r); return }
    var in DeleteTestcaseRequest
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeErr(w, "invalid_argument", "invalid JSON"); return }
    if s.DB == nil { writeErr(w, "internal", "service not initialized"); return }
    n, err := pgdao.DeleteTestcase(r.Context(), s.DB, in.ID)
    if err != nil { writeErr(w, "internal", err.Error()); return }
    writeOK(w, DeleteTestcaseResponse{Deleted: n})
}

func writeOK(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/connect+json")
    w.Header().Set("Connect-Protocol-Version", "1")
    _ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code, msg string) {
    w.Header().Set("Content-Type", "application/connect+json")
    w.Header().Set("Connect-Protocol-Version", "1")
    w.Header().Set("Connect-Error-Code", code)
    _ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": msg}})
}
