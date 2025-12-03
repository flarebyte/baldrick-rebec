//go:build darwin

package vault

import (
    "context"
    "fmt"
    "time"

    keychain "github.com/keybase/go-keychain"
)

// KeychainVaultDAO implements VaultDAO backed by the macOS Keychain.
// All secrets are stored as generic passwords under Service=rbc-vault and Account=<name>.
type KeychainVaultDAO struct{}

func newKeychainVaultDAO() (VaultDAO, error) { return &KeychainVaultDAO{}, nil }

func (d *KeychainVaultDAO) ListSecrets(ctx context.Context) ([]SecretMetadata, error) {
    q := keychain.NewItem()
    q.SetSecClass(keychain.SecClassGenericPassword)
    q.SetService(ServiceName)
    q.SetMatchLimit(keychain.MatchLimitAll)
    q.SetReturnData(false)
    q.SetReturnAttributes(true)
    // It's not strictly necessary, but SetSynchronizable may help avoid iCloud-synced items if desired.
    // q.SetSynchronizable(keychain.SynchronizableNo)

    results, err := keychain.QueryItem(q)
    if err != nil {
        return nil, fmt.Errorf("keychain list: %w", err)
    }
    out := make([]SecretMetadata, 0, len(results))
    for _, r := range results {
        // The library exposes Account/Service/Label/CreationDate/ModificationDate
        md := SecretMetadata{ Name: r.Account, IsSet: true, Backend: "keychain" }
        if !r.ModificationDate.IsZero() {
            // capture a copy for pointer semantics
            t := r.ModificationDate
            md.UpdatedAt = &t
        }
        out = append(out, md)
    }
    return out, nil
}

func (d *KeychainVaultDAO) GetSecretMetadata(ctx context.Context, name string) (SecretMetadata, error) {
    md := SecretMetadata{ Name: name, IsSet: false, Backend: "keychain" }
    q := keychain.NewItem()
    q.SetSecClass(keychain.SecClassGenericPassword)
    q.SetService(ServiceName)
    q.SetAccount(name)
    q.SetMatchLimit(keychain.MatchLimitOne)
    q.SetReturnData(false)
    q.SetReturnAttributes(true)
    rr, err := keychain.QueryItem(q)
    if err != nil {
        return md, fmt.Errorf("keychain query: %w", err)
    }
    if len(rr) == 0 {
        return md, nil
    }
    md.IsSet = true
    if !rr[0].ModificationDate.IsZero() {
        t := rr[0].ModificationDate
        md.UpdatedAt = &t
    }
    return md, nil
}

func (d *KeychainVaultDAO) SetSecret(ctx context.Context, name string, value []byte) error {
    // Try to update existing; if not found, add a new item
    // Update
    query := keychain.NewItem()
    query.SetSecClass(keychain.SecClassGenericPassword)
    query.SetService(ServiceName)
    query.SetAccount(name)

    upd := keychain.NewItem()
    upd.SetSecClass(keychain.SecClassGenericPassword)
    upd.SetService(ServiceName)
    upd.SetAccount(name)
    upd.SetLabel("rbc secret: " + name)
    upd.SetData(value)
    upd.SetAccessible(keychain.AccessibleAfterFirstUnlock)

    if err := keychain.UpdateItem(query, upd); err != nil {
        // If update fails because item not found, fall back to add
        add := keychain.NewItem()
        add.SetSecClass(keychain.SecClassGenericPassword)
        add.SetService(ServiceName)
        add.SetAccount(name)
        add.SetLabel("rbc secret: " + name)
        add.SetData(value)
        add.SetAccessible(keychain.AccessibleAfterFirstUnlock)
        if aerr := keychain.AddItem(add); aerr != nil {
            return fmt.Errorf("keychain add: %w", aerr)
        }
        return nil
    }
    return nil
}

func (d *KeychainVaultDAO) UnsetSecret(ctx context.Context, name string) error {
    del := keychain.NewItem()
    del.SetSecClass(keychain.SecClassGenericPassword)
    del.SetService(ServiceName)
    del.SetAccount(name)
    return keychain.DeleteItem(del)
}

func (d *KeychainVaultDAO) HasSecret(ctx context.Context, name string) (bool, error) {
    q := keychain.NewItem()
    q.SetSecClass(keychain.SecClassGenericPassword)
    q.SetService(ServiceName)
    q.SetAccount(name)
    q.SetMatchLimit(keychain.MatchLimitOne)
    q.SetReturnData(false)
    q.SetReturnAttributes(false)
    rr, err := keychain.QueryItem(q)
    if err != nil {
        return false, fmt.Errorf("keychain query: %w", err)
    }
    return len(rr) > 0, nil
}

func (d *KeychainVaultDAO) GetSecretForInternalUse(ctx context.Context, name string) ([]byte, error) {
    q := keychain.NewItem()
    q.SetSecClass(keychain.SecClassGenericPassword)
    q.SetService(ServiceName)
    q.SetAccount(name)
    q.SetMatchLimit(keychain.MatchLimitOne)
    q.SetReturnData(true)
    q.SetReturnAttributes(false)
    rr, err := keychain.QueryItem(q)
    if err != nil {
        return nil, fmt.Errorf("keychain get: %w", err)
    }
    if len(rr) == 0 {
        return nil, fmt.Errorf("secret not found: %s", name)
    }
    // The library returns Data as []byte in the result
    // Some versions set r.Data to nil unless ReturnData=true which we already did.
    if rr[0].Data == nil {
        return nil, fmt.Errorf("secret has no data: %s", name)
    }
    // Make a copy to avoid holding onto underlying slice
    out := make([]byte, len(rr[0].Data))
    copy(out, rr[0].Data)
    return out, nil
}

// Utility for tests/doctor: returns now as time for UpdatedAt simulation when keychain omits it.
func nowPtr() *time.Time { t := time.Now(); return &t }

