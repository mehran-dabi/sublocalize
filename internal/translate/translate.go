package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"sublocalize/internal/batching"
	"sublocalize/internal/provider"
	"sublocalize/internal/srt"
)

type Translator struct {
	Provider    provider.Provider
	Model       string
	Prompt      string
	Temperature float64
	Concurrency int
	MaxRetries  int
	BatchDelay  time.Duration
	Verbose     bool
}

type subtitleEntry struct {
	Index       int    `json:"index"`
	Text        string `json:"text"`
	ContextOnly bool   `json:"context_only,omitempty"`
}

func (t *Translator) TranslateBatches(ctx context.Context, batches []batching.Batch, allSubs []srt.Subtitle) ([]srt.Subtitle, error) {
	translated := make([]srt.Subtitle, len(allSubs))
	copy(translated, allSubs)

	totalBatches := len(batches)
	log.Printf("translating %d subtitle(s) in %d batch(es) (concurrency: %d)",
		len(allSubs), totalBatches, t.Concurrency)

	start := time.Now()

	type batchResult struct {
		index   int
		entries []subtitleEntry
		err     error
	}

	results := make([]batchResult, totalBatches)
	sem := make(chan struct{}, t.Concurrency)
	var wg sync.WaitGroup

	for i, batch := range batches {
		if i > 0 {
			time.Sleep(t.BatchDelay)
		}
		wg.Add(1)
		go func(i int, batch batching.Batch) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			batchStart := time.Now()
			label := fmt.Sprintf("batch %d/%d", i+1, totalBatches)
			log.Printf("%s: translating subtitles %d–%d ...",
				label, batch.Items[0].Index, batch.Items[len(batch.Items)-1].Index)

			entries, err := t.translateBatchWithRetry(ctx, batch, label)
			results[i] = batchResult{index: i, entries: entries, err: err}

			if err != nil {
				log.Printf("%s: failed in %s: %v",
					label, time.Since(batchStart).Round(time.Millisecond), err)
			} else {
				log.Printf("%s: received %d translation(s) in %s",
					label, len(entries), time.Since(batchStart).Round(time.Millisecond))
			}
		}(i, batch)
	}

	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("batch %d: %w", r.index+1, r.err)
		}
		resultMap := make(map[int]string, len(r.entries))
		for _, e := range r.entries {
			resultMap[e.Index] = e.Text
		}
		for j := range translated {
			if text, ok := resultMap[translated[j].Index]; ok {
				translated[j].Text = text
			}
		}
	}

	log.Printf("all %d batch(es) completed in %s",
		totalBatches, time.Since(start).Round(time.Millisecond))

	return translated, nil
}

func (t *Translator) translateBatchWithRetry(ctx context.Context, batch batching.Batch, label string) ([]subtitleEntry, error) {
	var lastErr error
	for attempt := range t.MaxRetries + 1 {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			log.Printf("%s: retrying (%d/%d) after %s ...", label, attempt, t.MaxRetries, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		entries, err := t.translateBatch(ctx, batch)
		if err == nil {
			return entries, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (t *Translator) translateBatch(ctx context.Context, batch batching.Batch) ([]subtitleEntry, error) {
	var entries []subtitleEntry

	for _, s := range batch.ContextBefore {
		entries = append(entries, subtitleEntry{Index: s.Index, Text: s.Text, ContextOnly: true})
	}
	for _, s := range batch.Items {
		entries = append(entries, subtitleEntry{Index: s.Index, Text: s.Text})
	}
	for _, s := range batch.ContextAfter {
		entries = append(entries, subtitleEntry{Index: s.Index, Text: s.Text, ContextOnly: true})
	}

	userPayload, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload: %w", err)
	}

	resp, err := t.Provider.Complete(ctx, provider.Request{
		SystemPrompt: t.Prompt,
		UserMessage:  string(userPayload),
		Model:        t.Model,
		Temperature:  t.Temperature,
	})
	if err != nil {
		return nil, err
	}

	jsonStr, err := extractJSONArray(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("extracting JSON: %w\nraw: %s", err, resp.Content)
	}

	var results []subtitleEntry
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("parsing translated subtitles: %w\nraw: %s", err, resp.Content)
	}

	mainIndices := make(map[int]bool, len(batch.Items))
	for _, item := range batch.Items {
		mainIndices[item.Index] = true
	}
	var filtered []subtitleEntry
	for _, r := range results {
		if mainIndices[r.Index] {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func extractJSONArray(s string) (string, error) {
	start := strings.Index(s, "[")
	if start == -1 {
		return "", fmt.Errorf("no JSON array found in response")
	}
	end := strings.LastIndex(s, "]")
	if end == -1 || end < start {
		return "", fmt.Errorf("no closing bracket found in response")
	}
	return s[start : end+1], nil
}
