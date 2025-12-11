package tooling

import (
	"context"
)

// MockToolDAO is an in-memory implementation of ToolDAO for tests.
type MockToolDAO struct {
	items map[string]*ToolConfig
}

// NewMockToolDAO constructs a MockToolDAO from a map keyed by tool name.
func NewMockToolDAO(items map[string]*ToolConfig) *MockToolDAO {
	if items == nil {
		items = map[string]*ToolConfig{}
	}
	return &MockToolDAO{items: items}
}

// GetToolByName returns the tool config by name if present.
func (m *MockToolDAO) GetToolByName(ctx context.Context, name string) (*ToolConfig, error) {
	if t, ok := m.items[name]; ok && t != nil {
		return t, nil
	}
	return nil, ErrToolNotFound
}

// MockVaultDAO is an in-memory implementation of VaultDAO for tests.
type MockVaultDAO struct {
	items map[string]*SecretMetadata
}

// NewMockVaultDAO constructs a MockVaultDAO from a map keyed by secret name.
func NewMockVaultDAO(items map[string]*SecretMetadata) *MockVaultDAO {
	if items == nil {
		items = map[string]*SecretMetadata{}
	}
	return &MockVaultDAO{items: items}
}

// GetSecretMetadata returns the secret metadata by name if present.
func (m *MockVaultDAO) GetSecretMetadata(ctx context.Context, name string) (*SecretMetadata, error) {
	if s, ok := m.items[name]; ok && s != nil {
		return s, nil
	}
	return nil, ErrSecretNotFound
}
