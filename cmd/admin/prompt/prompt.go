package prompt

import "github.com/spf13/cobra"

// PromptCmd is the root subcommand for prompt-related admin commands.
var PromptCmd = &cobra.Command{
    Use:   "prompt",
    Short: "Prompt utilities (admin)",
}

