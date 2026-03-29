package srt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Subtitle struct {
	Index int
	Start time.Duration
	End   time.Duration
	Text  string
}

func Parse(r io.Reader) ([]Subtitle, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	scanner := bufio.NewScanner(bytes.NewReader(data))

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

func FormatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
