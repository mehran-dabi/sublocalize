package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"sublocalize/internal/config"
	"sublocalize/internal/pipeline"

	"github.com/spf13/cobra"
)

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate an SRT subtitle file",
	Long: `Translate an SRT subtitle file to a target language using an LLM.

Configuration is resolved in this order (later overrides earlier):
  1. Built-in defaults
  2. Config file (--config)
  3. CLI flags`,
	RunE: runTranslate,
}

func init() {
	f := translateCmd.Flags()

	f.String("in", "", "input SRT file path (required)")
	f.String("out", "", "output SRT file path (required)")
	f.String("config", "", "path to YAML config file")

	f.String("target", "", "target language code (e.g. fa, de, es)")
	f.String("endpoint", "", "LLM API endpoint URL")
	f.String("model", "", "LLM model name")
	f.String("api-key-env", "", "env var name containing the API key (default: SUBLOCALIZE_API_KEY)")
	f.String("style", "", "translation style (natural, formal, casual)")
	f.Bool("keep-names-latin", false, "keep character/place names in Latin script")

	f.Int("batch-size", 0, "number of subtitles per translation batch")
	f.Int("context-lines", 0, "surrounding subtitle lines included as context")
	f.Int("concurrency", 0, "max concurrent API requests")
	f.Float64("temperature", 0, "LLM sampling temperature")
	f.String("format", "", "LLM response format (json)")

	f.String("glossary", "", "path to glossary JSON file")
	f.String("prompt", "", "path to custom system prompt file")

	f.Bool("dry-run", false, "show what would be done without calling the API")
	f.Bool("verbose", false, "enable verbose logging")

	_ = translateCmd.MarkFlagRequired("in")
	_ = translateCmd.MarkFlagRequired("out")

	rootCmd.AddCommand(translateCmd)
}

func runTranslate(cmd *cobra.Command, _ []string) error {
	var cfg *config.Config

	if configPath, _ := cmd.Flags().GetString("config"); configPath != "" {
		var err error
		cfg, err = config.LoadFile(configPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	} else {
		cfg = config.Default()
	}

	applyFlags(cmd, cfg)
	cfg.ResolveAPIKey()

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if cfg.Verbose {
		cfg.Print()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	p, err := pipeline.New(cfg)
	if err != nil {
		return err
	}

	return p.Run(ctx)
}

func applyFlags(cmd *cobra.Command, cfg *config.Config) {
	flags := cmd.Flags()

	if flags.Changed("in") {
		cfg.InputFile, _ = flags.GetString("in")
	}
	if flags.Changed("out") {
		cfg.OutputFile, _ = flags.GetString("out")
	}
	if flags.Changed("target") {
		cfg.TargetLanguage, _ = flags.GetString("target")
	}
	if flags.Changed("endpoint") {
		cfg.Endpoint, _ = flags.GetString("endpoint")
	}
	if flags.Changed("model") {
		cfg.Model, _ = flags.GetString("model")
	}
	if flags.Changed("api-key-env") {
		cfg.APIKeyEnv, _ = flags.GetString("api-key-env")
	}
	if flags.Changed("style") {
		cfg.Style, _ = flags.GetString("style")
	}
	if flags.Changed("keep-names-latin") {
		cfg.KeepNamesLatin, _ = flags.GetBool("keep-names-latin")
	}
	if flags.Changed("batch-size") {
		cfg.BatchSize, _ = flags.GetInt("batch-size")
	}
	if flags.Changed("context-lines") {
		cfg.ContextLines, _ = flags.GetInt("context-lines")
	}
	if flags.Changed("concurrency") {
		cfg.Concurrency, _ = flags.GetInt("concurrency")
	}
	if flags.Changed("temperature") {
		cfg.Temperature, _ = flags.GetFloat64("temperature")
	}
	if flags.Changed("format") {
		cfg.Format, _ = flags.GetString("format")
	}
	if flags.Changed("glossary") {
		cfg.GlossaryFile, _ = flags.GetString("glossary")
	}
	if flags.Changed("prompt") {
		cfg.PromptFile, _ = flags.GetString("prompt")
	}
	if flags.Changed("dry-run") {
		cfg.DryRun, _ = flags.GetBool("dry-run")
	}
	if flags.Changed("verbose") {
		cfg.Verbose, _ = flags.GetBool("verbose")
	}
}
