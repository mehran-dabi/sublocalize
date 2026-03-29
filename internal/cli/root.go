package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sublocalize",
	Short: "Translate subtitle files using LLM providers",
	Long: `sublocalize translates SRT subtitle files using LLM providers (LiteLLM, OpenAI, etc.).

It sends subtitle text in batches to an OpenAI-compatible chat completions API
and writes the translated output as a new SRT file. Supports glossaries,
custom prompts, configurable batch sizes, and automatic RTL formatting.`,
}

func Execute() error {
	return rootCmd.Execute()
}
