package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagTCName       string
	flagTCPackage    string
	flagTCClass      string
	flagTCTitle      string
	flagTCExperiment string
	flagTCRole       string
	flagTCStatus     string
	flagTCError      string
	flagTCTags       []string
	flagTCLevel      string
	flagTCFile       string
	flagTCLine       int
	flagTCTime       float64
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a testcase",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTCTitle) == "" {
			return errors.New("--title is required")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		tc := &pgdao.Testcase{Title: flagTCTitle, RoleName: flagTCRole, Status: flagTCStatus}
		if flagTCName != "" {
			tc.Name = sql.NullString{String: flagTCName, Valid: true}
		}
		if flagTCPackage != "" {
			tc.Package = sql.NullString{String: flagTCPackage, Valid: true}
		}
		if flagTCClass != "" {
			tc.Classname = sql.NullString{String: flagTCClass, Valid: true}
		}
		if flagTCExperiment != "" {
			tc.ExperimentID = sql.NullString{String: flagTCExperiment, Valid: true}
		}
		if flagTCError != "" {
			tc.ErrorMessage = sql.NullString{String: flagTCError, Valid: true}
		}
		if len(flagTCTags) > 0 {
			tc.Tags = parseTags(flagTCTags)
		}
		if flagTCLevel != "" {
			tc.Level = sql.NullString{String: flagTCLevel, Valid: true}
		}
		if flagTCFile != "" {
			tc.File = sql.NullString{String: flagTCFile, Valid: true}
		}
		if flagTCLine > 0 {
			tc.Line = sql.NullInt64{Int64: int64(flagTCLine), Valid: true}
		}
		if flagTCTime > 0 {
			tc.ExecutionTime = sql.NullFloat64{Float64: flagTCTime, Valid: true}
		}
		if err := pgdao.InsertTestcase(ctx, db, tc); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "testcase created id=%s title=%q status=%s\n", tc.ID, tc.Title, tc.Status)
		out := map[string]any{"id": tc.ID, "title": tc.Title, "status": tc.Status}
		if tc.Created.Valid {
			out["created"] = tc.Created.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TestcaseCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&flagTCName, "name", "", "Name of the test")
	createCmd.Flags().StringVar(&flagTCPackage, "package", "", "Namespace or module name")
	createCmd.Flags().StringVar(&flagTCClass, "classname", "", "Fully qualified class name")
	createCmd.Flags().StringVar(&flagTCTitle, "title", "", "Title (required)")
	createCmd.Flags().StringVar(&flagTCExperiment, "experiment", "", "Experiment UUID")
	createCmd.Flags().StringVar(&flagTCRole, "role", "user", "Role name")
	createCmd.Flags().StringVar(&flagTCStatus, "status", "KO", "Status (OK/KO...)")
	createCmd.Flags().StringVar(&flagTCError, "error", "", "Error message")
	createCmd.Flags().StringSliceVar(&flagTCTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
	createCmd.Flags().StringVar(&flagTCLevel, "level", "", "Level: h1..h6")
	createCmd.Flags().StringVar(&flagTCFile, "file", "", "Path to source file")
	createCmd.Flags().IntVar(&flagTCLine, "line", 0, "Source line number")
	createCmd.Flags().Float64Var(&flagTCTime, "execution-time", 0.0, "Execution time in seconds")
}

// parseTags converts k=v pairs (or bare keys) into a map.
func parseTags(items []string) map[string]any {
	if len(items) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, raw := range items {
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if eq := strings.IndexByte(p, '='); eq > 0 {
				k := strings.TrimSpace(p[:eq])
				v := strings.TrimSpace(p[eq+1:])
				if k != "" {
					out[k] = v
				}
			} else {
				out[p] = true
			}
		}
	}
	return out
}
