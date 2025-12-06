package paths

import (
	"os"
	"path/filepath"
)

const envHome = "BALDRICK_REBEC_HOME_DIR"

// Home returns the base directory for baldrick-rebec configuration/state.
// Defaults to ~/.baldrick-rebec, can be overridden via BALDRICK_REBEC_HOME_DIR.
func Home() string {
	if v := os.Getenv(envHome); v != "" {
		return v
	}
	hd, err := os.UserHomeDir()
	if err != nil || hd == "" {
		return ".baldrick-rebec"
	}
	return filepath.Join(hd, ".baldrick-rebec")
}

func EnsureHome() (string, error) {
	h := Home()
	if err := os.MkdirAll(h, 0o755); err != nil {
		return "", err
	}
	return h, nil
}
