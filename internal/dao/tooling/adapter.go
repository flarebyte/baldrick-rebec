package tooling

import (
	"context"
	"fmt"
)

// VaultDAOAdapter is a production adapter stub. Wire your secret backend client here.
type VaultDAOAdapter struct {
	// client any // placeholder for an actual client
}

// NewVaultDAOAdapter returns a new adapter instance.
func NewVaultDAOAdapter() *VaultDAOAdapter { return &VaultDAOAdapter{} }

// GetSecretMetadata resolves secret metadata by name. This stub returns ErrSecretNotFound.
func (v *VaultDAOAdapter) GetSecretMetadata(ctx context.Context, name string) (*SecretMetadata, error) {
	// TODO: integrate real secret store. For now, return a clear not found error.
	return nil, fmt.Errorf("vault adapter: %w: %s", ErrSecretNotFound, name)
}
