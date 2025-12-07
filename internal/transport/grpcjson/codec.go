package grpcjson

import (
    "encoding/json"
    "google.golang.org/grpc/encoding"
)

// Codec is a JSON codec for gRPC unary calls.
type Codec struct{}

func (Codec) Name() string { return "json" }
func (Codec) Marshal(v any) ([]byte, error)   { return json.Marshal(v) }
func (Codec) Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// Register registers the codec globally; safe to call multiple times.
func Register() { encoding.RegisterCodec(Codec{}) }

