package task

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "regexp"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagRunVariant   string
    flagRunVersion   string
    flagRunExperiment string
    flagRunExecutor  string
    flagRunTimeout   string // go duration; overrides task timeout
    flagRunEnv       []string
)

var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Run a task by variant and version with timeout and experiment id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRunVariant) == "" || strings.TrimSpace(flagRunVersion) == "" {
            return errors.New("--variant and --version are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        // Resolve task
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        task, err := pgdao.GetTaskByKey(ctx, db, flagRunVariant, flagRunVersion)
        if err != nil { return err }
        if !task.Run.Valid || strings.TrimSpace(task.Run.String) == "" {
            return fmt.Errorf("task %q@%s has no run script defined", task.Variant, task.Version)
        }
        // Parse timeout: flag overrides task's timeout
        toDur, err := chooseTimeout(flagRunTimeout, task.Timeout.String)
        if err != nil { return err }
        if toDur <= 0 { toDur = 10 * time.Minute }

        // Start message: status=starting
        startText := fmt.Sprintf("starting task %s@%s (shell=%s, timeout=%s)", task.Variant, task.Version, valueOr(task.Shell.String, "bash"), toDur)
        metaStart := map[string]any{
            "variant": task.Variant,
            "version": task.Version,
            "status":  "starting",
            "timeout": toDur.String(),
            "shell":   valueOr(task.Shell.String, "bash"),
        }
        metaStartJSON, _ := json.Marshal(metaStart)
        contentID, err := pgdao.InsertContent(ctx, db, startText, metaStartJSON)
        if err != nil { return err }
        ev := &pgdao.MessageEvent{
            ContentID: contentID,
            Status:    "starting",
            Tags:      []string{"task", "run"},
        }
        if strings.TrimSpace(task.ID) != "" { ev.TaskID = sql.NullString{String: task.ID, Valid: true} }
        if strings.TrimSpace(flagRunExperiment) != "" { ev.ExperimentID = sql.NullString{String: flagRunExperiment, Valid: true} }
        if strings.TrimSpace(flagRunExecutor) != "" { ev.Executor = sql.NullString{String: flagRunExecutor, Valid: true} }
        msgID, err := pgdao.InsertMessageEvent(ctx, db, ev)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "running task %s@%s (message id=%d)\n", task.Variant, task.Version, msgID)

        // Prepare command with context timeout
        runCtx, cancelRun := context.WithTimeout(context.Background(), toDur)
        defer cancelRun()
        cmdExec, interpreter := buildCommand(runCtx, task)
        // Environment
        if len(flagRunEnv) > 0 {
            cmdExec.Env = append(os.Environ(), flagRunEnv...)
        }
        // Capture output
        var outBuf, errBuf bytes.Buffer
        cmdExec.Stdout = &outBuf
        cmdExec.Stderr = &errBuf

        // Run
        startTime := time.Now()
        runErr := cmdExec.Run()
        dur := time.Since(startTime)

        // Determine status
        status := "succeeded"
        exitCode := 0
        var errMsg string
        if runErr != nil {
            if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
                status = "timeout"
                exitCode = -1
                errMsg = "context deadline exceeded"
            } else {
                status = "failed"
                if ee, ok := runErr.(*exec.ExitError); ok {
                    exitCode = ee.ExitCode()
                } else {
                    exitCode = -1
                }
                errMsg = runErr.Error()
            }
        }

        // Prepare completion content and update message
        compMeta := map[string]any{
            "variant":  task.Variant,
            "version":  task.Version,
            "status":   status,
            "duration": dur.String(),
            "exit_code": exitCode,
            "shell":    interpreter,
        }
        compMetaJSON, _ := json.Marshal(compMeta)
        content := buildCompletionContent(task, interpreter, dur, exitCode, &outBuf, &errBuf, errMsg)
        compCID, err := pgdao.InsertContent(context.Background(), db, content, compMetaJSON)
        if err != nil { return err }
        // Update message row
        upd := pgdao.MessageEvent{
            ContentID:   compCID,
            Status:      status,
            ProcessedAt: sql.NullTime{Time: time.Now(), Valid: true},
        }
        if errMsg != "" { upd.ErrorMessage = sql.NullString{String: errMsg, Valid: true} }
        if err := pgdao.UpdateMessageEvent(context.Background(), db, msgID, upd); err != nil { return err }

        // Human output
        fmt.Fprintf(os.Stderr, "task %s@%s finished status=%s duration=%s exit_code=%d\n", task.Variant, task.Version, status, dur, exitCode)
        // JSON output
        out := map[string]any{
            "message_id": msgID,
            "variant": task.Variant,
            "version": task.Version,
            "status":  status,
            "duration": dur.String(),
            "exit_code": exitCode,
            "experiment_id": flagRunExperiment,
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TaskCmd.AddCommand(runCmd)
    runCmd.Flags().StringVar(&flagRunVariant, "variant", "", "Task selector variant, e.g., unit/go (required)")
    runCmd.Flags().StringVar(&flagRunVersion, "version", "", "Task semver version (required)")
    runCmd.Flags().StringVar(&flagRunExperiment, "experiment", "", "Experiment UUID to link execution")
    runCmd.Flags().StringVar(&flagRunExecutor, "executor", "cli", "Executor identifier")
    runCmd.Flags().StringVar(&flagRunTimeout, "timeout", "", "Override timeout as Go duration, e.g., 5m30s")
    runCmd.Flags().StringSliceVar(&flagRunEnv, "env", nil, "Extra environment variables KEY=VALUE (repeatable)")
}

// chooseTimeout selects a time.Duration given an optional Go duration string and
// a Postgres interval textual representation. Returns error only if an override
// is provided but cannot be parsed.
func chooseTimeout(override, pgInterval string) (time.Duration, error) {
    if strings.TrimSpace(override) != "" {
        return time.ParseDuration(strings.TrimSpace(override))
    }
    // Try parsing PG interval common formats: 'HH:MM:SS' or 'N mins' etc.
    s := strings.TrimSpace(pgInterval)
    if s == "" { return 0, nil }
    if d, err := time.ParseDuration(s); err == nil { return d, nil }
    // HH:MM:SS[.mmm]
    if m := regexp.MustCompile(`^(\d{1,2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?$`).FindStringSubmatch(s); len(m) > 0 {
        h := mustAtoi(m[1]); mm := mustAtoi(m[2]); ss := mustAtoi(m[3])
        n := time.Duration(h)*time.Hour + time.Duration(mm)*time.Minute + time.Duration(ss)*time.Second
        return n, nil
    }
    // Fallback: minutes
    if strings.Contains(s, "minute") {
        // crude extraction of first number
        num := firstNumber(s)
        if num > 0 { return time.Duration(num) * time.Minute, nil }
    }
    if strings.Contains(s, "hour") {
        num := firstNumber(s)
        if num > 0 { return time.Duration(num) * time.Hour, nil }
    }
    if strings.Contains(s, "second") {
        num := firstNumber(s)
        if num > 0 { return time.Duration(num) * time.Second, nil }
    }
    return 0, nil
}

func mustAtoi(s string) int {
    var n int
    for i := 0; i < len(s); i++ {
        if s[i] < '0' || s[i] > '9' { continue }
        n = n*10 + int(s[i]-'0')
    }
    return n
}

func firstNumber(s string) int { return mustAtoi(s) }

func valueOr(s, def string) string { if strings.TrimSpace(s) == "" { return def }; return s }

func buildCommand(ctx context.Context, task *pgdao.Task) (*exec.Cmd, string) {
    shell := strings.TrimSpace(task.Shell.String)
    script := task.Run.String
    switch strings.ToLower(shell) {
    case "bash", "":
        return exec.CommandContext(ctx, "bash", "-c", script), "bash"
    case "sh":
        return exec.CommandContext(ctx, "sh", "-c", script), "sh"
    case "python", "python3":
        return exec.CommandContext(ctx, "python3", "-c", script), "python3"
    case "node", "nodejs":
        return exec.CommandContext(ctx, "node", "-e", script), "node"
    default:
        // Unknown interpreter: try to run via bash -c
        return exec.CommandContext(ctx, "bash", "-c", script), "bash"
    }
}

func buildCompletionContent(task *pgdao.Task, interpreter string, dur time.Duration, exitCode int, stdout, stderr *bytes.Buffer, errMsg string) string {
    b := &strings.Builder{}
    fmt.Fprintf(b, "task: %s@%s\n", task.Variant, task.Version)
    fmt.Fprintf(b, "shell: %s\n", interpreter)
    fmt.Fprintf(b, "duration: %s\n", dur)
    fmt.Fprintf(b, "exit_code: %d\n", exitCode)
    if errMsg != "" { fmt.Fprintf(b, "error: %s\n", errMsg) }
    if stdout != nil && stdout.Len() > 0 {
        fmt.Fprintf(b, "\n=== STDOUT ===\n%s\n", stdout.String())
    }
    if stderr != nil && stderr.Len() > 0 {
        fmt.Fprintf(b, "\n=== STDERR ===\n%s\n", stderr.String())
    }
    return b.String()
}
