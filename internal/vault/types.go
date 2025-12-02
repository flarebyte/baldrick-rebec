package vault

import (
    "context"
    "fmt"
    "sync"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
)

// VaultDAO defines the operations required to manage secrets in a backend vault.
// Implementations must never log or print secret values.
type VaultDAO interface {
    // ListSecrets returns metadata for all secrets in the backend for this service.
    ListSecrets(ctx context.Context) ([]SecretMetadata, error)
    // GetSecretMetadata returns metadata for a specific secret name. If the secret is not set,
    // implementations should return metadata with IsSet=false and no error.
    GetSecretMetadata(ctx context.Context, name string) (SecretMetadata, error)
    // SetSecret creates or updates a secret value.
    SetSecret(ctx context.Context, name string, value []byte) error
    // UnsetSecret deletes the secret.
    UnsetSecret(ctx context.Context, name string) error
    // HasSecret indicates whether a secret exists.
    HasSecret(ctx context.Context, name string) (bool, error)
    // GetSecretForInternalUse fetches the raw secret value for internal usage only.
    // CLI code must never print or log this value.
    GetSecretForInternalUse(ctx context.Context, name string) ([]byte, error)
}

// SecretMetadata contains non-sensitive information about a secret.
type SecretMetadata struct {
    Name      string
    IsSet     bool
    Backend   string
    UpdatedAt *time.Time
}

const (
    // ServiceName groups all secrets belonging to this application in the Keychain.
    ServiceName = "rbc-vault"
)

// NewVaultDAO constructs a DAO for the selected backend. For now only "keychain" is supported.
func NewVaultDAO(backend string) (VaultDAO, error) {
    switch backend {
    case "", "keychain":
        return newKeychainVaultDAO()
    default:
        return nil, fmt.Errorf("vault backend not implemented: %s", backend)
    }
}

// package global to cache the chosen backend DAO for internal access.
var (
    cached struct {
        sync.Mutex
        dao VaultDAO
        be  string
    }
)

// GetSecret provides a safe internal accessor for other packages to retrieve a secret at runtime.
// It uses the configured backend from config.yaml and caches the DAO for reuse.
func GetSecret(ctx context.Context, name string) ([]byte, error) {
    cfg, err := cfgpkg.Load()
    if err != nil {
        return nil, err
    }
    backend := cfg.Vault.Backend
    cached.Lock()
    if cached.dao == nil || cached.be != backend {
        dao, derr := NewVaultDAO(backend)
        if derr != nil {
            cached.Unlock()
            return nil, derr
        }
        cached.dao = dao
        cached.be = backend
    }
    dao := cached.dao
    cached.Unlock()
    return dao.GetSecretForInternalUse(ctx, name)
}
