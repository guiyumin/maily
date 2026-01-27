# Multiple AI API Formats Support

Maily's Tauri desktop app supports multiple AI API formats and CLI tools for maximum flexibility.

## API Formats

### OpenAI-compatible (`type: "openai"`)

The industry standard format. Most providers are OpenAI-compatible:

| Provider | Base URL | Notes |
|----------|----------|-------|
| OpenAI | `https://api.openai.com/v1` | Original |
| Mistral AI | `https://api.mistral.ai/v1` | Full compatibility |
| OpenRouter | `https://openrouter.ai/api/v1` | Gateway to 400+ models |
| Groq | `https://api.groq.com/openai/v1` | Fast inference |
| Together AI | `https://api.together.xyz/v1` | Open-source models |
| DeepSeek | `https://api.deepseek.com/v1` | Coding models |
| Fireworks AI | `https://api.fireworks.ai/inference/v1` | Fast inference |
| Ollama | `http://localhost:11434/v1` | Local models |
| LiteLLM | `http://localhost:4000` | Self-hosted proxy |
| LM Studio | `http://localhost:1234/v1` | Local models |

**Request format:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."}
  ],
  "max_tokens": 4096
}
```

### Anthropic (`type: "anthropic"`)

Native Anthropic Messages API with different request/response structure:

| Provider | Base URL |
|----------|----------|
| Anthropic | `https://api.anthropic.com` |

**Request format:**
```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 4096,
  "system": "...",
  "messages": [
    {"role": "user", "content": "..."}
  ]
}
```

**Headers:**
- `x-api-key`: Your Anthropic API key
- `anthropic-version`: `2023-06-01`

### OpenResponses (`type: "openresponses"`)

Emerging multi-provider standard backed by NVIDIA, Vercel, OpenRouter, Hugging Face, and others.

Currently implemented as OpenAI-compatible with support for custom headers. As the spec stabilizes, specific handling will be added.

See: https://www.openresponses.org/

## CLI Tools

Maily can auto-detect and use these CLI tools:

| Tool | Command | Output Format |
|------|---------|---------------|
| `claude` | `claude -p "prompt" --model haiku --output-format json` | JSON with `result` field |
| `codex` | `codex exec "prompt" --json` | NDJSON stream |
| `gemini` | `gemini "prompt" -m flash -o json` | JSON with `response` field |
| `mistral` | `mistral chat "prompt" -m mistral-small-latest` | Raw stdout |
| `ollama` | `ollama run llama3.2 "prompt"` | Raw stdout |
| `opencode` | `opencode exec "prompt" --json` | NDJSON stream (like codex) |
| `crush` | `crush -p "prompt"` | Raw stdout |
| `vibe` | `vibe "prompt"` | Raw stdout |

## Configuration

### Using Presets

The Settings UI provides presets for easy configuration:

1. Open Settings > AI Providers
2. Click "Add Provider"
3. Select a preset from "Quick Setup" (OpenAI, Anthropic, OpenRouter, etc.)
4. The base URL and type are auto-filled
5. Enter your API key
6. Enter a model name (placeholders show examples for each provider)
7. Click "Add Provider"

Note: Model lists are not hardcoded since new models come out frequently. Enter the model name directly.

### Manual Configuration

In `~/.config/maily/config.yml`:

```yaml
ai_providers:
  # CLI tool
  - type: cli
    name: claude
    model: sonnet

  # OpenAI-compatible API
  - type: openai
    name: openai
    model: gpt-4o
    base_url: https://api.openai.com/v1
    api_key: sk-...

  # Anthropic API
  - type: anthropic
    name: anthropic
    model: claude-sonnet-4-20250514
    base_url: https://api.anthropic.com
    api_key: sk-ant-...

  # With custom headers (e.g., OpenRouter)
  - type: openai
    name: openrouter
    preset: openrouter
    model: anthropic/claude-sonnet-4
    base_url: https://openrouter.ai/api/v1
    api_key: sk-or-...
    custom_headers:
      HTTP-Referer: https://maily.app
      X-Title: Maily
```

### Custom Headers

Some providers require custom HTTP headers:

- **OpenRouter**: Recommends `HTTP-Referer` and `X-Title` for rate limiting
- **Azure OpenAI**: May require `api-version` header

Add headers in the Settings UI or in config.yml:

```yaml
custom_headers:
  HTTP-Referer: https://your-app.com
  X-Title: Your App Name
```

## Provider Selection Logic

All AI tasks (summarization, reply generation, tagging, event extraction) use the same unified provider selection mechanism.

### Priority Order

1. **Specific provider** - If a task requests a specific provider by name, try it first
2. **Configured providers** - Try providers from `ai_providers` list in order
3. **Auto-detected CLI tools** - If no providers configured, auto-detect available CLI tools

### Auto-Detection Order

When no providers are configured, CLI tools are detected in this order:

| Priority | CLI Tool | Default Model |
|----------|----------|---------------|
| 1 | `claude` | haiku |
| 2 | `codex` | o4-mini |
| 3 | `gemini` | gemini-2.5-flash |
| 4 | `opencode` | default |
| 5 | `crush` | default |
| 6 | `mistral` | mistral-small-latest |
| 7 | `vibe` | default |
| 8 | `ollama` | llama3.2:3b |

### Fallback Behavior

- Providers are tried sequentially until one succeeds
- Maximum 3 providers attempted per request
- On success, result is returned immediately (no further providers tried)
- On failure, the next provider is attempted
- If all providers fail, an error is returned listing all attempted providers

### Example Flow

```
Request: Summarize email
         ↓
[1] Try claude/haiku
    → HTTP 429 (rate limited)
         ↓
[2] Try openai/gpt-4o
    → Success!
         ↓
Return summary (stop here)
```

### AI Tasks Using This Logic

| Task | Description | Called From |
|------|-------------|-------------|
| **Summarize** | Generate email summary | `s` key in TUI, Summary button in Tauri |
| **Reply** | Draft email reply | Compose view |
| **Tags** | Auto-generate email tags | Label picker |
| **Event Extraction** | Extract calendar events from email | Email view |
| **Reminder Extraction** | Extract reminders from email | Email view |
| **NLP Event** | Parse natural language into event | Calendar quick-add |

## Backward Compatibility

- `type: "api"` is aliased to `type: "openai"` for backward compatibility
- Existing configurations continue to work without changes

## Implementation Details

### Files

| File | Purpose |
|------|---------|
| `internal/ai/client.go` | Go AI client with fallback logic |
| `config/config.go` | Go config types (AIProvider struct) |
| `tauri/src-tauri/src/config.rs` | Rust config types |
| `tauri/src-tauri/src/ai.rs` | Rust AI provider implementations |
| `tauri/src/lib/ai/client.ts` | TypeScript AI client with fallback |
| `tauri/src/lib/ai/hooks.ts` | React hooks for AI features |
| `tauri/src/lib/ai/providers/` | SDK-specific providers (anthropic, openai, openrouter) |
| `tauri/src/lib/ai/presets.ts` | Provider presets for UI |
| `tauri/src/components/settings/AIProvidersSettings.tsx` | Settings UI |

### Adding a New Provider

1. **If OpenAI-compatible**: Add preset in `presets.ts`, no backend changes needed
2. **If different format**: Add new variant to `AIProviderType` enum and implement `call_*_provider()` function in `ai.rs`

### Sources

- [OpenAI API](https://platform.openai.com/docs/api-reference)
- [Anthropic API](https://docs.anthropic.com/en/api)
- [Mistral API](https://docs.mistral.ai/api)
- [OpenRouter API](https://openrouter.ai/docs/api/reference/overview)
- [LiteLLM](https://docs.litellm.ai/docs/)
- [Ollama OpenAI Compatibility](https://docs.ollama.com/api/openai-compatibility)
- [OpenResponses](https://www.openresponses.org/)
