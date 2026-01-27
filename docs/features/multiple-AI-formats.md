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

## Provider Fallback

Providers are tried in order. If one fails, the next is attempted:

1. Configured providers (in order)
2. Auto-detected CLI tools (claude, codex, gemini, ollama, opencode, crush, vibe)

## Backward Compatibility

- `type: "api"` is aliased to `type: "openai"` for backward compatibility
- Existing configurations continue to work without changes

## Implementation Details

### Files

| File | Purpose |
|------|---------|
| `tauri/src-tauri/src/config.rs` | Rust config types |
| `tauri/src-tauri/src/ai.rs` | AI provider implementations |
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
