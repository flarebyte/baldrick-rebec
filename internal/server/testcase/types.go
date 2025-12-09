package testcase

// JSON-friendly request/response types for gRPC JSON codec.

type CreateTestcaseRequest struct {
    Title         string            `json:"title"`
    Role          string            `json:"role"`
    Experiment    string            `json:"experiment,omitempty"`
    Status        string            `json:"status,omitempty"`
    Name          string            `json:"name,omitempty"`
    Package       string            `json:"package,omitempty"`
    Classname     string            `json:"classname,omitempty"`
    ErrorMessage  string            `json:"error,omitempty"`
    Tags          map[string]any    `json:"tags,omitempty"`
    Level         string            `json:"level,omitempty"`
    File          string            `json:"file,omitempty"`
    Line          int32             `json:"line,omitempty"`
    ExecutionTime float64           `json:"execution_time,omitempty"`
}

type CreateTestcaseResponse struct {
    ID      string `json:"id"`
    Title   string `json:"title"`
    Status  string `json:"status"`
    Created string `json:"created,omitempty"`
}

type ListTestcasesRequest struct {
    Role       string `json:"role"`
    Experiment string `json:"experiment,omitempty"`
    Status     string `json:"status,omitempty"`
    Limit      int32  `json:"limit,omitempty"`
    Offset     int32  `json:"offset,omitempty"`
}

type TestcaseItem struct {
    ID           string            `json:"id"`
    Title        string            `json:"title"`
    Status       string            `json:"status"`
    Created      string            `json:"created,omitempty"`
    Name         string            `json:"name,omitempty"`
    Package      string            `json:"package,omitempty"`
    Classname    string            `json:"classname,omitempty"`
    ExperimentID string            `json:"experiment_id,omitempty"`
    Tags         map[string]any    `json:"tags,omitempty"`
    File         string            `json:"file,omitempty"`
    Line         int32             `json:"line,omitempty"`
    Execution    float64           `json:"execution_time,omitempty"`
}

type ListTestcasesResponse struct {
    Items []TestcaseItem `json:"items"`
}

type DeleteTestcaseRequest struct {
    ID string `json:"id"`
}

type DeleteTestcaseResponse struct {
    Deleted int64 `json:"deleted"`
}

