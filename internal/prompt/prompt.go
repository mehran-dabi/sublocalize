package prompt

import (
	"fmt"
	"os"
	"strings"
)

const defaultTemplate = `You are an expert subtitle localizer translating English subtitles into natural spoken %s.

Your job is localization, not literal translation.

Rules:
- Preserve the meaning, tone, urgency, and emotional force of the original.
- Make the translation sound natural in spoken dialogue.
- Avoid word-for-word translation.
- Keep subtitles concise and readable.
- Preserve line breaks when reasonable for subtitle readability.%s%s
- Do not translate bracketed sound cues literally; render them in natural subtitle style.
- For slang, profanity, and idioms, translate the intent and intensity naturally.
- For professional terms (medical, legal, police, etc.), choose the most natural term for the scene, not the most literal.
- Maintain consistency across repeated terms and names.
- Do not add explanations.
- Always translate every subtitle, including song lyrics, poems, and quoted text. Never refuse, skip, or comment on any entry. If you cannot translate, return the original text unchanged.
%s%s
You will receive a JSON array of objects with "index" and "text" fields.
Return ONLY a JSON array in the exact same format with the translated text.
Do not include anything else — no markdown, no SRT formatting, no commentary.`

type Config struct {
	TargetLanguage string
	Style          string
	KeepNamesLatin bool
	Glossary       map[string]string
	HasContext     bool
}

func Build(cfg Config) string {
	langName := languageName(cfg.TargetLanguage)

	nameRule := ""
	if cfg.KeepNamesLatin {
		nameRule = "\n- Keep character and place names unchanged in their original Latin script."
	}

	styleRule := ""
	if cfg.Style != "" && cfg.Style != "natural" {
		styleRule = fmt.Sprintf("\n- Use a %s translation style.", cfg.Style)
	}

	glossarySection := ""
	if len(cfg.Glossary) > 0 {
		var entries []string
		for eng, translated := range cfg.Glossary {
			entries = append(entries, fmt.Sprintf("  - \"%s\" → \"%s\"", eng, translated))
		}
		glossarySection = fmt.Sprintf("\nGlossary (use these exact translations):\n%s\n", strings.Join(entries, "\n"))
	}

	contextNote := ""
	if cfg.HasContext {
		contextNote = "\nItems marked with \"context_only\": true are provided for reference only. Do NOT include them in your output. Only translate items without that field.\n"
	}

	return fmt.Sprintf(defaultTemplate, langName, nameRule, styleRule, glossarySection, contextNote)
}

func LoadFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading prompt file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func languageName(code string) string {
	names := map[string]string{
		"fa": "Persian (Farsi)",
		"de": "German",
		"es": "Spanish",
		"fr": "French",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"zh": "Chinese",
		"ja": "Japanese",
		"ko": "Korean",
		"ar": "Arabic",
		"tr": "Turkish",
		"hi": "Hindi",
		"nl": "Dutch",
		"sv": "Swedish",
		"pl": "Polish",
		"uk": "Ukrainian",
		"cs": "Czech",
		"ro": "Romanian",
		"th": "Thai",
		"vi": "Vietnamese",
		"id": "Indonesian",
		"ms": "Malay",
		"bn": "Bengali",
		"ta": "Tamil",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
