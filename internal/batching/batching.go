package batching

import "sublocalize/internal/srt"

type Batch struct {
	ContextBefore []srt.Subtitle
	Items         []srt.Subtitle
	ContextAfter  []srt.Subtitle
}

// Split divides subtitles into batches of the given size.
// When contextLines > 0, each batch includes surrounding subtitles
// for translation context (these are not translated themselves).
func Split(subs []srt.Subtitle, batchSize, contextLines int) []Batch {
	var batches []Batch

	for i := 0; i < len(subs); i += batchSize {
		end := i + batchSize
		if end > len(subs) {
			end = len(subs)
		}

		b := Batch{
			Items: subs[i:end],
		}

		if contextLines > 0 {
			ctxStart := i - contextLines
			if ctxStart < 0 {
				ctxStart = 0
			}
			if ctxStart < i {
				b.ContextBefore = subs[ctxStart:i]
			}

			ctxEnd := end + contextLines
			if ctxEnd > len(subs) {
				ctxEnd = len(subs)
			}
			if ctxEnd > end {
				b.ContextAfter = subs[end:ctxEnd]
			}
		}

		batches = append(batches, b)
	}

	return batches
}
