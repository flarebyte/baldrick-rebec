//go:build !darwin

package vault

import (
    "context"
    "fmt"
)

type KeychainVaultDAO struct{}

func newKeychainVaultDAO() (VaultDAO, error) { return nil, fmt.Errorf("keychain backend not supported on this OS") }

func (d *KeychainVaultDAO) ListSecrets(ctx context.Context) ([]SecretMetadata, error) {
    return nil, fmt.Errorf("keychain backend not supported on this OS")
}
func (d *KeychainVaultDAO) GetSecretMetadata(ctx context.Context, name string) (SecretMetadata, error) {
    return SecretMetadata{Name: name, IsSet: false, Backend: "keychain"}, fmt.Errorf("keychain backend not supported on this OS")
}
func (d *KeychainVaultDAO) SetSecret(ctx context.Context, name string, value []byte) error {
    return fmt.Errorf("keychain backend not supported on this OS")
}
func (d *KeychainVaultDAO) UnsetSecret(ctx context.Context, name string) error {
    return fmt.Errorf("keychain backend not supported on this OS")
}
func (d *KeychainVaultDAO) HasSecret(ctx context.Context, name string) (bool, error) {
    return false, fmt.Errorf("keychain backend not supported on this OS")
}
func (d *KeychainVaultDAO) GetSecretForInternalUse(ctx context.Context, name string) ([]byte, error) {
    return nil, fmt.Errorf("keychain backend not supported on this OS")
}

