package vaultcmd

import (
    "bufio"
    "context"
    "errors"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    vpkg "github.com/flarebyte/baldrick-rebec/internal/vault"
    "github.com/spf13/cobra"
    "golang.org/x/term"
)

var setCmd = &cobra.Command{
    Use:   "set <name>",
    Short: "Set or update a secret value in the vault",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := strings.TrimSpace(args[0])
        if name == "" { return errors.New("name must not be empty") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        dao, err := vpkg.NewVaultDAO(cfg.Vault.Backend)
        if err != nil { return err }
        // Prompt for secret without echo if terminal; else read from stdin as-is.
        secret, err := promptSecret(fmt.Sprintf("Enter secret for %q: ", name))
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        if err := dao.SetSecret(ctx, name, secret); err != nil { return err }
        fmt.Fprintf(os.Stderr, "secret %q stored in backend %q\n", name, cfg.Vault.Backend)
        return nil
    },
}

func init() {
    VaultCmd.AddCommand(setCmd)
}

func promptSecret(prompt string) ([]byte, error) {
    // If stdin is a terminal, use no-echo password input.
    if term.IsTerminal(int(os.Stdin.Fd())) {
        fmt.Fprint(os.Stderr, prompt)
        b, err := term.ReadPassword(int(os.Stdin.Fd()))
        fmt.Fprintln(os.Stderr)
        if err != nil { return nil, err }
        // Trim trailing CR/LF if any
        s := strings.TrimRight(string(b), "\r\n")
        return []byte(s), nil
    }
    // Otherwise, read from stdin with a warning.
    fmt.Fprintln(os.Stderr, "warning: reading secret from stdin; input will not be masked")
    r := bufio.NewReader(os.Stdin)
    line, err := r.ReadString('\n')
    if err != nil && !errors.Is(err, os.ErrClosed) && !strings.Contains(err.Error(), "EOF") {
        return nil, err
    }
    return []byte(strings.TrimRight(line, "\r\n")), nil
}

