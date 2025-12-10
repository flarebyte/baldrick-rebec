package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	grpcjson "github.com/flarebyte/baldrick-rebec/internal/transport/grpcjson"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

// Service exposes testcase CRUD via gRPC JSON codec.
type Service struct {
	DB *pgxpool.Pool
}

// TestcaseServiceServer defines the interface used by gRPC registration.
type TestcaseServiceServer interface {
	Create(context.Context, *CreateTestcaseRequest) (*CreateTestcaseResponse, error)
	List(context.Context, *ListTestcasesRequest) (*ListTestcasesResponse, error)
	Delete(context.Context, *DeleteTestcaseRequest) (*DeleteTestcaseResponse, error)
}

// Register registers the service with the provided gRPC server using a JSON codec.
func (s *Service) Register(gs *grpc.Server) {
	grpcjson.Register()
	gs.RegisterService(&grpc.ServiceDesc{
		ServiceName: "testcase.v1.TestcaseService",
		HandlerType: (*TestcaseServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Create", Handler: s.handleCreate},
			{MethodName: "List", Handler: s.handleList},
			{MethodName: "Delete", Handler: s.handleDelete},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/testcase/v1/testcase.proto",
	}, s)
}

func (s *Service) handleCreate(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in CreateTestcaseRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if s.DB == nil {
		return nil, fmt.Errorf("testcase service not initialized")
	}
	// Build DAO entity
	tc := &pgdao.Testcase{Title: in.Title, RoleName: orDefault(in.Role, "user"), Status: orDefault(in.Status, "OK")}
	if in.Name != "" {
		tc.Name = sqlString(in.Name)
	}
	if in.Package != "" {
		tc.Package = sqlString(in.Package)
	}
	if in.Classname != "" {
		tc.Classname = sqlString(in.Classname)
	}
	if in.Experiment != "" {
		tc.ExperimentID = sqlString(in.Experiment)
	}
	if in.ErrorMessage != "" {
		tc.ErrorMessage = sqlString(in.ErrorMessage)
	}
	if len(in.Tags) > 0 {
		tc.Tags = in.Tags
	}
	if in.Level != "" {
		tc.Level = sqlString(in.Level)
	}
	if in.File != "" {
		tc.File = sqlString(in.File)
	}
	if in.Line > 0 {
		tc.Line = sqlInt64(int64(in.Line))
	}
	if in.ExecutionTime > 0 {
		tc.ExecutionTime = sqlFloat64(in.ExecutionTime)
	}

	if err := pgdao.InsertTestcase(ctx, s.DB, tc); err != nil {
		return nil, err
	}
	out := &CreateTestcaseResponse{ID: tc.ID, Title: tc.Title, Status: tc.Status}
	if tc.Created.Valid {
		out.Created = tc.Created.Time.Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return out, nil
}

func (s *Service) handleList(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in ListTestcasesRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if s.DB == nil {
		return nil, fmt.Errorf("testcase service not initialized")
	}
	limit := int(in.Limit)
	offset := int(in.Offset)
	items, err := pgdao.ListTestcases(ctx, s.DB, in.Role, in.Experiment, in.Status, limit, offset)
	if err != nil {
		return nil, err
	}
	resp := &ListTestcasesResponse{Items: make([]TestcaseItem, 0, len(items))}
	for _, t := range items {
		it := TestcaseItem{ID: t.ID, Title: t.Title, Status: t.Status}
		if t.Created.Valid {
			it.Created = t.Created.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		if t.Name.Valid {
			it.Name = t.Name.String
		}
		if t.Package.Valid {
			it.Package = t.Package.String
		}
		if t.Classname.Valid {
			it.Classname = t.Classname.String
		}
		if t.ExperimentID.Valid {
			it.ExperimentID = t.ExperimentID.String
		}
		if len(t.Tags) > 0 {
			it.Tags = t.Tags
		}
		if t.File.Valid {
			it.File = t.File.String
		}
		if t.Line.Valid {
			it.Line = int32(t.Line.Int64)
		}
		if t.ExecutionTime.Valid {
			it.Execution = t.ExecutionTime.Float64
		}
		resp.Items = append(resp.Items, it)
	}
	return resp, nil
}

func (s *Service) handleDelete(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in DeleteTestcaseRequest
	if err := dec(&in); err != nil {
		return nil, err
	}
	if s.DB == nil {
		return nil, fmt.Errorf("testcase service not initialized")
	}
	n, err := pgdao.DeleteTestcase(ctx, s.DB, in.ID)
	if err != nil {
		return nil, err
	}
	return &DeleteTestcaseResponse{Deleted: n}, nil
}

// Utility for logging/debugging (unused but kept for parity with prompt patterns)
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func sqlString(s string) sql.NullString    { return sql.NullString{String: s, Valid: true} }
func sqlInt64(n int64) sql.NullInt64       { return sql.NullInt64{Int64: n, Valid: true} }
func sqlFloat64(f float64) sql.NullFloat64 { return sql.NullFloat64{Float64: f, Valid: true} }
