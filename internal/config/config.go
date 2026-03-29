package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	InputFile      string `yaml:"-"`
	OutputFile     string `yaml:"-"`
	Endpoint       string `yaml:"endpoint"`
	Model          string `yaml:"model"`
	TargetLanguage string `yaml:"target_language"`
	Style          string `yaml:"style"`
	BatchSize      int    `yaml:"batch_size"`
	ContextLines   int    `yaml:"context_lines"`
	Temperature    float64           `yaml:"temperature"`
	APIKeyEnv      string            `yaml:"api_key_env"`
	APIKey         string            `yaml:"-"`
	KeepNamesLatin bool              `yaml:"keep_names_in_latin"`
	GlossaryFile   string            `yaml:"glossary_file"`
	Glossary       map[string]string `yaml:"-"`
	PromptFile     string            `yaml:"prompt_file"`
	Format         string            `yaml:"format"`
	DryRun         bool              `yaml:"-"`
	Verbose        bool              `yaml:"verbose"`
	Concurrency    int               `yaml:"concurrency"`
}

func Default() *Config {
	return &Config{
		Endpoint:       "http://localhost:4000/v1",
		Model:          "gpt-4o-mini",
		TargetLanguage: "en",
		Style:          "natural",
		BatchSize:      40,
		Temperature:    0.3,
		APIKeyEnv:      "SUBLOCALIZE_API_KEY",
		KeepNamesLatin: true,
		Format:         "json",
		Concurrency:    5,
	}
}

// LoadFile reads a YAML config file, starting from defaults so unspecified
// fields keep their default values.
func LoadFile(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

func (c *Config) ResolveAPIKey() {
	if c.APIKeyEnv != "" {
		c.APIKey = os.Getenv(c.APIKeyEnv)
	}
}

func (c *Config) LoadGlossary() error {
	if c.GlossaryFile == "" {
		return nil
	}

	data, err := os.ReadFile(c.GlossaryFile)
	if err != nil {
		return fmt.Errorf("reading glossary file: %w", err)
	}

	if err := json.Unmarshal(data, &c.Glossary); err != nil {
		return fmt.Errorf("parsing glossary file: %w", err)
	}

	return nil
}

func (c *Config) Validate() error {
	if c.InputFile == "" {
		return fmt.Errorf("input file is required (--in)")
	}
	if c.OutputFile == "" {
		return fmt.Errorf("output file is required (--out)")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	if c.TargetLanguage == "" {
		return fmt.Errorf("target language is required (--target)")
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1")
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key not set (export %s=... or set api_key_env in config)", c.APIKeyEnv)
	}
	return nil
}

func (c *Config) Print() {
	log.Printf("configuration:")
	log.Printf("  input:            %s", c.InputFile)
	log.Printf("  output:           %s", c.OutputFile)
	log.Printf("  endpoint:         %s", c.Endpoint)
	log.Printf("  model:            %s", c.Model)
	log.Printf("  target language:  %s", c.TargetLanguage)
	log.Printf("  style:            %s", c.Style)
	log.Printf("  batch size:       %d", c.BatchSize)
	log.Printf("  context lines:    %d", c.ContextLines)
	log.Printf("  temperature:      %.2f", c.Temperature)
	log.Printf("  api key env:      %s", c.APIKeyEnv)
	log.Printf("  api key set:      %v", c.APIKey != "")
	log.Printf("  keep names latin: %v", c.KeepNamesLatin)
	log.Printf("  glossary file:    %s", c.GlossaryFile)
	log.Printf("  prompt file:      %s", c.PromptFile)
	log.Printf("  format:           %s", c.Format)
	log.Printf("  concurrency:      %d", c.Concurrency)
	log.Printf("  dry run:          %v", c.DryRun)
}
