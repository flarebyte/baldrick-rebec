package dbutil

import (
    "database/sql"
    "fmt"
    "reflect"
    "strings"
    "time"
)

// ParamSummary returns a privacy-conscious summary of a parameter for logging.
// It avoids leaking actual values while providing useful debugging signals.
//
// Rules:
// - name=null for nil or nil pointers (and sql.Null* with Valid=false)
// - name=empty for empty strings
// - name=len=N for non-empty strings or slices/arrays
// - name=V for integers or floats
// - name=true/false for booleans
// - name=zero-time or name=non-zero-time for time.Time
// - For other kinds, returns name=<kind>
func ParamSummary(name string, v any) string {
    // Handle typed SQL nulls explicitly
    switch x := v.(type) {
    case nil:
        return name + "=null"
    case sql.NullString:
        if !x.Valid { return name + "=null" }
        if x.String == "" { return name + "=empty" }
        return fmt.Sprintf("%s=len=%d", name, len(x.String))
    case *sql.NullString:
        if x == nil || !x.Valid { return name + "=null" }
        if x.String == "" { return name + "=empty" }
        return fmt.Sprintf("%s=len=%d", name, len(x.String))
    case sql.NullBool:
        if !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%t", name, x.Bool)
    case *sql.NullBool:
        if x == nil || !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%t", name, x.Bool)
    case sql.NullInt64:
        if !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%d", name, x.Int64)
    case *sql.NullInt64:
        if x == nil || !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%d", name, x.Int64)
    case sql.NullFloat64:
        if !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%g", name, x.Float64)
    case *sql.NullFloat64:
        if x == nil || !x.Valid { return name + "=null" }
        return fmt.Sprintf("%s=%g", name, x.Float64)
    case sql.NullTime:
        if !x.Valid { return name + "=null" }
        if x.Time.IsZero() { return name + "=zero-time" }
        return name + "=non-zero-time"
    case *sql.NullTime:
        if x == nil || !x.Valid { return name + "=null" }
        if x.Time.IsZero() { return name + "=zero-time" }
        return name + "=non-zero-time"
    }

    // Reflect-based handling (including pointers)
    rv := reflect.ValueOf(v)
    if !rv.IsValid() {
        return name + "=null"
    }

    // Handle pointers generically
    if rv.Kind() == reflect.Ptr {
        if rv.IsNil() { return name + "=null" }
        rv = rv.Elem()
    }

    // time.Time special case
    if rv.Type() == reflect.TypeOf(time.Time{}) {
        tt := rv.Interface().(time.Time)
        if tt.IsZero() { return name + "=zero-time" }
        return name + "=non-zero-time"
    }

    switch rv.Kind() {
    case reflect.String:
        if rv.Len() == 0 { return name + "=empty" }
        return fmt.Sprintf("%s=len=%d", name, rv.Len())
    case reflect.Slice, reflect.Array:
        return fmt.Sprintf("%s=len=%d", name, rv.Len())
    case reflect.Bool:
        return fmt.Sprintf("%s=%t", name, rv.Bool())
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        return fmt.Sprintf("%s=%d", name, rv.Int())
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
        return fmt.Sprintf("%s=%d", name, rv.Uint())
    case reflect.Float32, reflect.Float64:
        return fmt.Sprintf("%s=%g", name, rv.Float())
    default:
        return fmt.Sprintf("%s=%s", name, rv.Kind().String())
    }
}

// ErrWrap returns a formatted error with an operation label and optional summaries.
// Example: ErrWrap("blackboard.upsert", err, ParamSummary("id", id), ParamSummary("role", role))
func ErrWrap(op string, err error, parts ...string) error {
    if err == nil { return nil }
    if len(parts) == 0 {
        return fmt.Errorf("%s: %w", op, err)
    }
    return fmt.Errorf("%s: %w; %s", op, err, strings.Join(parts, ","))
}
