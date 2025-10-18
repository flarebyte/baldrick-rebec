package configcmd

import (
    "fmt"
    "os"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

var printCmd = &cobra.Command{
    Use:   "print",
    Short: "Print the merged configuration to stdout",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        b, err := yaml.Marshal(cfg)
        if err != nil { return err }
        os.Stdout.Write(b)
        if len(b) == 0 || b[len(b)-1] != '\n' { fmt.Fprintln(os.Stdout) }
        return nil
    },
}

