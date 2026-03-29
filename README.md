# sublocalize

A CLI tool for translating SRT subtitle files using LLM providers (LiteLLM, OpenAI, and any OpenAI-compatible API).

It sends subtitles in configurable batches to a chat completions endpoint and writes a translated SRT file. Supports glossaries, custom prompts, context-aware batching, and automatic RTL formatting for languages like Persian and Arabic.

## Installation

```bash
go install sublocalize/cmd/sublocalize@latest
```

Or build from source:

```bash
git clone https://github.com/mehrandabestani/sublocalize.git
cd sublocalize
go build -o sublocalize ./cmd/sublocalize
```

## Quick Start

1. Export your API key:

```bash
export SUBLOCALIZE_API_KEY=your-api-key-here
```

2. Translate a subtitle file:

```bash
sublocalize translate \
  --in input.srt \
  --out output.fa.srt \
  --target fa \
  --endpoint https://api.openai.com/v1 \
  --model gpt-4o
```

## Usage

```
sublocalize translate [flags]
```

### Required Flags

| Flag    | Description              |
|---------|--------------------------|
| `--in`  | Input SRT file path      |
| `--out` | Output SRT file path     |

### Optional Flags

| Flag                 | Default                | Description                                       |
|----------------------|------------------------|---------------------------------------------------|
| `--config`           |                        | Path to YAML config file                          |
| `--target`           | `en`                   | Target language code (e.g. `fa`, `de`, `es`)      |
| `--endpoint`         | `http://localhost:4000/v1` | LLM API endpoint URL                          |
| `--model`            | `gpt-4o-mini`          | LLM model name                                    |
| `--api-key-env`      | `SUBLOCALIZE_API_KEY`  | Env var name containing the API key               |
| `--batch-size`       | `40`                   | Subtitles per translation batch                   |
| `--context-lines`    | `0`                    | Surrounding subtitle lines included as context    |
| `--concurrency`      | `5`                    | Max concurrent API requests                       |
| `--temperature`      | `0.3`                  | LLM sampling temperature                          |
| `--style`            | `natural`              | Translation style (natural, formal, casual)       |
| `--keep-names-latin` | `true` (via config)    | Keep character/place names in Latin script        |
| `--glossary`         |                        | Path to glossary JSON file                        |
| `--prompt`           |                        | Path to custom system prompt file                 |
| `--format`           | `json`                 | LLM response format                               |
| `--dry-run`          | `false`                | Show plan without calling the API                 |
| `--verbose`          | `false`                | Enable verbose logging                            |

## Config File

Instead of passing every flag, create a YAML config file:

```yaml
endpoint: https://litellm.internal.company.com/v1
model: claude-opus-4-6
target_language: fa
style: natural
batch_size: 40
temperature: 0.2
api_key_env: SUBLOCALIZE_API_KEY
keep_names_in_latin: true
context_lines: 2
concurrency: 5
```

Then run with:

```bash
sublocalize translate --in input.srt --out output.fa.srt --config sublocalize.yaml
```

CLI flags override config file values when both are provided.

## Glossary

Create a JSON file mapping terms to their translations:

```json
{
  "The Force": "نیرو",
  "lightsaber": "شمشیر نوری",
  "Jedi": "جدای"
}
```

Use it with `--glossary glossary.json`. The terms are injected into the system prompt so the LLM uses exact translations for those terms.

## Custom Prompts

Override the built-in system prompt with `--prompt prompt.txt`. The file contents replace the entire system prompt sent to the LLM.

## Context Lines

The `--context-lines` flag includes surrounding subtitles as context when translating each batch. This helps the LLM produce more coherent translations across batch boundaries. Context lines are sent to the model but not included in the translated output.

## RTL Support

When translating to RTL languages (Persian, Arabic, Hebrew, Urdu, etc.), the output SRT file automatically wraps each line with Unicode RTL embedding marks for correct display in media players.

## Examples

Full flags:

```bash
sublocalize translate \
  --in input.srt \
  --out output.fa.srt \
  --target fa \
  --endpoint https://my-litellm.company.com/v1 \
  --model claude-opus-4-6 \
  --api-key-env SUBLOCALIZE_API_KEY \
  --batch-size 40 \
  --context-lines 2 \
  --glossary glossary.json \
  --temperature 0.2 \
  --verbose
```

Dry run to preview batching:

```bash
sublocalize translate \
  --in input.srt \
  --out output.fa.srt \
  --config sublocalize.yaml \
  --dry-run
```

## Project Structure

```
sublocalize/
  cmd/sublocalize/       Entry point
  internal/
    cli/                 CLI commands and flag parsing
    config/              YAML config loading and validation
    srt/                 SRT parsing and formatting
    batching/            Batch splitting with context overlap
    prompt/              System prompt building
    translate/           Batch translation orchestration
    provider/            LLM API client (OpenAI-compatible)
    pipeline/            End-to-end translation pipeline
    output/              SRT output writer with RTL support
  examples/              Example config and glossary files
```
