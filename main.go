package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var translator *TranslatorClient

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	baseURL := envOrDefault("LITELLM_BASE_URL", "http://localhost:4000")
	apiKey := os.Getenv("LITELLM_API_KEY")
	model := envOrDefault("LITELLM_MODEL", "gpt-4o-mini")
	promptPath := envOrDefault("PROMPT_PATH", "prompt.md")

	translator = NewTranslatorClient(baseURL, apiKey, model, promptPath)

	http.HandleFunc("/upload", handleUpload)

	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to read uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".srt") {
		http.Error(w, "only .srt files are accepted", http.StatusBadRequest)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

	subtitles, err := parseSRT(bufio.NewScanner(bytes.NewReader(content)))
	if err != nil {
		http.Error(w, "failed to parse SRT: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("parsed %d subtitle(s), translating in batches of %d...", len(subtitles), batchSize)

	translated, err := translator.Translate(r.Context(), subtitles)
	if err != nil {
		http.Error(w, "translation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, s := range translated {
		log.Printf("#%d [%s --> %s] %s", s.Index, formatDuration(s.Start), formatDuration(s.End), s.Text)
	}

	outName := strings.TrimSuffix(header.Filename, ".srt") + "_translated.srt"
	if err := writeSRT(outName, translated); err != nil {
		http.Error(w, "failed to write output: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("wrote %s", outName)
	fmt.Fprintf(w, "translated %d subtitle(s), saved to %s\n", len(translated), outName)
}

func parseSRT(scanner *bufio.Scanner) ([]Subtitle, error) {
	var subtitles []Subtitle
	var current Subtitle
	var textLines []string

	type state int
	const (
		stateIndex state = iota
		stateTimestamp
		stateText
	)

	st := stateIndex

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		switch st {
		case stateIndex:
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			idx, err := strconv.Atoi(line)
			if err != nil {
				return nil, fmt.Errorf("expected subtitle index, got %q", line)
			}
			current.Index = idx
			st = stateTimestamp

		case stateTimestamp:
			start, end, err := parseTimestampLine(line)
			if err != nil {
				return nil, err
			}
			current.Start = start
			current.End = end
			st = stateText

		case stateText:
			if strings.TrimSpace(line) == "" {
				current.Text = strings.Join(textLines, "\n")
				subtitles = append(subtitles, current)
				current = Subtitle{}
				textLines = nil
				st = stateIndex
			} else {
				textLines = append(textLines, line)
			}
		}
	}

	if len(textLines) > 0 {
		current.Text = strings.Join(textLines, "\n")
		subtitles = append(subtitles, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return subtitles, nil
}

// parseTimestampLine parses "00:01:23,456 --> 00:01:25,789"
func parseTimestampLine(line string) (time.Duration, time.Duration, error) {
	parts := strings.Split(line, " --> ")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid timestamp line: %q", line)
	}

	start, err := parseTimestamp(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start timestamp: %w", err)
	}

	end, err := parseTimestamp(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end timestamp: %w", err)
	}

	return start, end, nil
}

// parseTimestamp parses "HH:MM:SS,mmm" into a time.Duration.
func parseTimestamp(s string) (time.Duration, error) {
	s = strings.Replace(s, ",", ".", 1)

	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("expected HH:MM:SS,mmm format, got %q", s)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	secParts := strings.Split(parts[2], ".")
	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, err
	}

	var millis int
	if len(secParts) == 2 {
		millis, err = strconv.Atoi(secParts[1])
		if err != nil {
			return 0, err
		}
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond, nil
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func writeSRT(filename string, subs []Subtitle) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i, s := range subs {
		fmt.Fprintf(w, "%d\n", s.Index)
		fmt.Fprintf(w, "%s --> %s\n", formatDuration(s.Start), formatDuration(s.End))
		lines := strings.Split(s.Text, "\n")
		for _, line := range lines {
			fmt.Fprintf(w, "\u202B%s\u202C\n", line)
		}
		if i < len(subs)-1 {
			fmt.Fprintln(w)
		}
	}
	return w.Flush()
}
