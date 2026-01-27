package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"maily/config"
)

// provider represents an initialized AI provider ready to use
type provider struct {
	config    config.AIProvider
	apiClient *openai.Client // nil for CLI providers
}

// Client handles AI operations using configured providers
type Client struct {
	providers []provider // tried in order
}

// NewClient creates a new AI client from configured providers
// Priority: API providers first, then CLI providers, then auto-detected CLI tools
func NewClient() *Client {
	cfg, _ := config.Load()

	client := &Client{}

	// Collect API providers first (higher priority)
	for _, p := range cfg.AIProviders {
		if p.Model == "" || p.Type != config.AIProviderTypeAPI {
			continue
		}
		if p.APIKey == "" {
			continue
		}
		baseURL := p.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		apiClient := openai.NewClient(
			option.WithAPIKey(p.APIKey),
			option.WithBaseURL(baseURL),
		)
		client.providers = append(client.providers, provider{
			config:    p,
			apiClient: &apiClient,
		})
	}

	// Then collect CLI providers (fallback)
	for _, p := range cfg.AIProviders {
		if p.Model == "" || p.Type != config.AIProviderTypeCLI {
			continue
		}
		if commandExists(p.Name) {
			client.providers = append(client.providers, provider{config: p})
		}
	}

	// If no providers configured, auto-detect CLI tools
	if len(client.providers) == 0 {
		client.providers = detectProviders()
	}

	return client
}

// Available returns true if at least one AI provider is available
func (c *Client) Available() bool {
	return len(c.providers) > 0
}

// Provider returns the first provider's name and model
func (c *Client) Provider() string {
	if len(c.providers) == 0 {
		return ""
	}
	p := c.providers[0]
	if p.config.Name != "" {
		return p.config.Name + "/" + p.config.Model
	}
	return p.config.Model
}

// maxRetries is the maximum number of providers to try before giving up
const maxRetries = 3

// Call executes a prompt using configured providers in order
func (c *Client) Call(prompt string) (string, error) {
	if len(c.providers) == 0 {
		return "", errors.New("no AI provider available - configure ai_providers in config.yml or install codex, gemini, claude, vibe, or ollama")
	}

	var failedProviders []string
	limit := len(c.providers)
	if limit > maxRetries {
		limit = maxRetries
	}

	for i := 0; i < limit; i++ {
		p := c.providers[i]

		var result string
		var err error

		if p.apiClient != nil {
			result, err = callAPI(*p.apiClient, p.config.Model, prompt)
		} else {
			result, err = callCLI(p.config.Name, p.config.Model, prompt)
		}

		if err == nil {
			return result, nil
		}

		name := p.config.Name
		if name == "" {
			name = p.config.Model
		}
		failedProviders = append(failedProviders, name)
	}

	return "", errors.New("AI failed: " + strings.Join(failedProviders, ", ") + " all failed")
}

// callCLI executes a prompt using a CLI tool
func callCLI(name, model, prompt string) (string, error) {
	var cmd *exec.Cmd
	var parseFunc func(string) string

	switch name {
	case "claude":
		cmd = exec.Command("claude", "-p", prompt, "--model", model, "--output-format", "json", "--no-session-persistence")
		parseFunc = parseClaudeOutput

	case "codex":
		cmd = exec.Command("codex", "exec", prompt, "--model", model, "--json")
		parseFunc = parseCodexOutput

	case "gemini":
		cmd = exec.Command("gemini", "-p", prompt, "-m", model, "--output-format", "json")
		parseFunc = parseGeminiOutput

	case "opencode":
		cmd = exec.Command("opencode", "exec", prompt, "--json")
		parseFunc = parseCodexOutput // similar output format to codex

	case "crush":
		cmd = exec.Command("crush", "-p", prompt)
		parseFunc = func(s string) string { return s }

	case "mistral":
		cmd = exec.Command("mistral", "-p", prompt, "-m", model)
		parseFunc = func(s string) string { return s }

	case "vibe":
		cmd = exec.Command("vibe", prompt)
		parseFunc = func(s string) string { return s }

	case "ollama":
		cmd = exec.Command("ollama", "run", model, prompt)
		parseFunc = func(s string) string { return s }

	default:
		return "", errors.New("unknown CLI provider: " + name)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", errors.New("AI call failed: " + errMsg)
	}

	output := parseFunc(stdout.String())
	return strings.TrimSpace(output), nil
}

// callAPI makes a call to an OpenAI-compatible API
func callAPI(client openai.Client, model, prompt string) (string, error) {
	ctx := context.Background()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", errors.New("API call failed: " + err.Error())
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("API returned no choices")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// parseClaudeOutput extracts the result from Claude JSON output
func parseClaudeOutput(output string) string {
	var response struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}

	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return output
	}

	return response.Result
}

// parseGeminiOutput extracts the response from Gemini JSON output
func parseGeminiOutput(output string) string {
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		return output
	}
	jsonStr := output[jsonStart:]

	var response struct {
		Response string `json:"response"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return output
	}

	return response.Response
}

// parseCodexOutput extracts the agent message from codex JSON output
func parseCodexOutput(output string) string {
	lines := strings.Split(output, "\n")

	var lastMessage string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			lastMessage = event.Item.Text
		}
	}

	return lastMessage
}

// detectProviders auto-detects available CLI tools and returns default configs
func detectProviders() []provider {
	// Check CLIs in order of preference
	clis := []struct {
		name  string
		model string
	}{
		{"claude", "haiku"},
		{"codex", "o4-mini"},
		{"gemini", "gemini-2.5-flash"},
		{"opencode", "default"},
		{"crush", "default"},
		{"mistral", "mistral-small-latest"},
		{"vibe", "default"},
		{"ollama", "llama3.2:3b"},
	}

	var providers []provider
	for _, cli := range clis {
		if commandExists(cli.name) {
			providers = append(providers, provider{
				config: config.AIProvider{
					Type:  config.AIProviderTypeCLI,
					Name:  cli.name,
					Model: cli.model,
				},
			})
		}
	}

	return providers
}

// commandExists checks if a command is available in PATH
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
