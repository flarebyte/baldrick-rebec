//go:build darwin && !cgo

package vault

import (
	"context"
	"fmt"
)

// KeychainVaultDAO (no-cgo) stub for darwin cross-builds
type KeychainVaultDAONoCgo struct{}

func newKeychainVaultDAO() (VaultDAO, error) {
	return nil, fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}

func (d *KeychainVaultDAONoCgo) ListSecrets(ctx context.Context) ([]SecretMetadata, error) {
	return nil, fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
func (d *KeychainVaultDAONoCgo) GetSecretMetadata(ctx context.Context, name string) (SecretMetadata, error) {
	return SecretMetadata{Name: name, IsSet: false, Backend: "keychain"}, fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
func (d *KeychainVaultDAONoCgo) SetSecret(ctx context.Context, name string, value []byte) error {
	return fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
func (d *KeychainVaultDAONoCgo) UnsetSecret(ctx context.Context, name string) error {
	return fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
func (d *KeychainVaultDAONoCgo) HasSecret(ctx context.Context, name string) (bool, error) {
	return false, fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
func (d *KeychainVaultDAONoCgo) GetSecretForInternalUse(ctx context.Context, name string) ([]byte, error) {
	return nil, fmt.Errorf("keychain backend requires cgo on darwin; rebuild with CGO_ENABLED=1")
}
