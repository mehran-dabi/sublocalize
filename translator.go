package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func loadSystemPrompt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load prompt file %s: %v", path, err))
	}
	return strings.TrimSpace(string(data))
}

const (
	batchSize    = 40
	concurrency  = 10
	batchDelay   = time.Second
)

type TranslatorClient struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	HTTPClient   *http.Client
}

func NewTranslatorClient(baseURL, apiKey, model, promptPath string) *TranslatorClient {
	return &TranslatorClient{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: loadSystemPrompt(promptPath),
		HTTPClient:   &http.Client{},
	}
}

// subtitleEntry is the JSON shape sent to and received from the LLM.
type subtitleEntry struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Translate translates all subtitles by batching them and calling LiteLLM for each batch.
func (c *TranslatorClient) Translate(ctx context.Context, subs []Subtitle) ([]Subtitle, error) {
	translated := make([]Subtitle, len(subs))
	copy(translated, subs)

	batches := batchSubtitles(subs, batchSize)
	totalBatches := len(batches)

	log.Printf("splitting %d subtitle(s) into %d batch(es) of up to %d (concurrency: %d)", len(subs), totalBatches, batchSize, concurrency)

	start := time.Now()

	type batchResult struct {
		index   int
		entries []subtitleEntry
		err     error
	}

	results := make([]batchResult, totalBatches)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, batch := range batches {
		if i > 0 {
			time.Sleep(batchDelay)
		}
		wg.Add(1)
		go func(i int, batch []Subtitle) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			batchStart := time.Now()
			log.Printf("batch %d/%d: translating subtitles %d–%d ...", i+1, totalBatches, batch[0].Index, batch[len(batch)-1].Index)

			entries, err := c.translateBatch(ctx, batch)
			results[i] = batchResult{index: i, entries: entries, err: err}

			if err != nil {
				log.Printf("batch %d/%d: failed in %s: %v", i+1, totalBatches, time.Since(batchStart).Round(time.Millisecond), err)
			} else {
				log.Printf("batch %d/%d: received %d translation(s) in %s", i+1, totalBatches, len(entries), time.Since(batchStart).Round(time.Millisecond))
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

	log.Printf("all %d batch(es) completed in %s", totalBatches, time.Since(start).Round(time.Millisecond))

	return translated, nil
}

func (c *TranslatorClient) translateBatch(ctx context.Context, batch []Subtitle) ([]subtitleEntry, error) {
	entries := make([]subtitleEntry, len(batch))
	for i, s := range batch {
		entries[i] = subtitleEntry{Index: s.Index, Text: s.Text}
	}

	userPayload, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("marshal user payload: %w", err)
	}

	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: c.SystemPrompt},
			{Role: "user", Content: string(userPayload)},
		},
		Temperature: 0.3,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LiteLLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	content := chatResp.Choices[0].Message.Content

	jsonStr, err := extractJSONArray(content)
	if err != nil {
		return nil, fmt.Errorf("extract JSON: %w\nraw content: %s", err, content)
	}

	var results []subtitleEntry
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("unmarshal translated subtitles: %w\nraw content: %s", err, content)
	}

	return results, nil
}

func batchSubtitles(subs []Subtitle, size int) [][]Subtitle {
	var batches [][]Subtitle
	for i := 0; i < len(subs); i += size {
		end := i + size
		if end > len(subs) {
			end = len(subs)
		}
		batches = append(batches, subs[i:end])
	}
	return batches
}

// extractJSONArray finds the first JSON array in the string,
// ignoring any surrounding commentary or code fences.
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
