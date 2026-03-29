package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"sublocalize/internal/batching"
	"sublocalize/internal/config"
	"sublocalize/internal/output"
	"sublocalize/internal/prompt"
	"sublocalize/internal/provider"
	"sublocalize/internal/srt"
	"sublocalize/internal/translate"
)

type Pipeline struct {
	cfg        *config.Config
	translator *translate.Translator
}

func New(cfg *config.Config) (*Pipeline, error) {
	if err := cfg.LoadGlossary(); err != nil {
		return nil, err
	}

	var systemPrompt string
	if cfg.PromptFile != "" {
		var err error
		systemPrompt, err = prompt.LoadFromFile(cfg.PromptFile)
		if err != nil {
			return nil, err
		}
	} else {
		systemPrompt = prompt.Build(prompt.Config{
			TargetLanguage: cfg.TargetLanguage,
			Style:          cfg.Style,
			KeepNamesLatin: cfg.KeepNamesLatin,
			Glossary:       cfg.Glossary,
			HasContext:     cfg.ContextLines > 0,
		})
	}

	if cfg.Verbose {
		log.Printf("system prompt:\n%s", systemPrompt)
	}

	llm := provider.NewOpenAI(cfg.Endpoint, cfg.APIKey)

	t := &translate.Translator{
		Provider:    llm,
		Model:       cfg.Model,
		Prompt:      systemPrompt,
		Temperature: cfg.Temperature,
		Concurrency: cfg.Concurrency,
		BatchDelay:  time.Second,
		Verbose:     cfg.Verbose,
	}

	return &Pipeline{cfg: cfg, translator: t}, nil
}

func (p *Pipeline) Run(ctx context.Context) error {
	f, err := os.Open(p.cfg.InputFile)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer f.Close()

	subs, err := srt.Parse(f)
	if err != nil {
		return fmt.Errorf("parsing SRT: %w", err)
	}

	log.Printf("parsed %d subtitle(s) from %s", len(subs), p.cfg.InputFile)

	batches := batching.Split(subs, p.cfg.BatchSize, p.cfg.ContextLines)

	if p.cfg.DryRun {
		p.printDryRun(subs, batches)
		return nil
	}

	translated, err := p.translator.TranslateBatches(ctx, batches, subs)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	rtl := isRTL(p.cfg.TargetLanguage)
	if err := output.WriteSRT(p.cfg.OutputFile, translated, rtl); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	log.Printf("wrote %d translated subtitle(s) to %s", len(translated), p.cfg.OutputFile)
	return nil
}

func (p *Pipeline) printDryRun(subs []srt.Subtitle, batches []batching.Batch) {
	log.Printf("[dry-run] would translate %d subtitle(s) in %d batch(es)", len(subs), len(batches))
	log.Printf("[dry-run] target: %s, model: %s, endpoint: %s",
		p.cfg.TargetLanguage, p.cfg.Model, p.cfg.Endpoint)
	for i, b := range batches {
		log.Printf("[dry-run] batch %d: subtitles %d–%d (%d items, %d context before, %d context after)",
			i+1, b.Items[0].Index, b.Items[len(b.Items)-1].Index,
			len(b.Items), len(b.ContextBefore), len(b.ContextAfter))
	}
}

func isRTL(lang string) bool {
	rtlLangs := map[string]bool{
		"fa": true, "ar": true, "he": true, "ur": true,
		"yi": true, "ps": true, "ku": true,
	}
	return rtlLangs[lang]
}
