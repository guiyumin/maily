package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"maily/config"
)

// Provider represents an AI CLI provider
type Provider string

const (
	ProviderOpenAI Provider = "openai" // OpenAI-compatible API (OpenAI, NVIDIA, etc.)
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
	ProviderGemini Provider = "gemini"
	ProviderVibe   Provider = "vibe"
	ProviderOllama Provider = "ollama"
	ProviderNone   Provider = ""
)

// openaiAccount holds a configured OpenAI-compatible client
type openaiAccount struct {
	name   string
	client openai.Client
	model  string
}

// Client handles AI operations using available CLI tools or OpenAI-compatible APIs
type Client struct {
	provider       Provider
	openaiAccounts []openaiAccount // tried in order
	cliProvider    Provider        // fallback CLI provider
}

// NewClient creates a new AI client, preferring OpenAI configs if set, otherwise auto-detecting CLI
func NewClient() *Client {
	cfg, _ := config.Load()

	client := &Client{
		cliProvider: detectProvider(),
	}

	// Build OpenAI accounts from config
	for _, acc := range cfg.AIAccounts {
		if acc.APIKey == "" {
			continue
		}

		baseURL := acc.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := acc.Model
		if model == "" {
			model = "gpt-4o-mini"
		}

		opts := []option.RequestOption{
			option.WithAPIKey(acc.APIKey),
			option.WithBaseURL(baseURL),
		}

		client.openaiAccounts = append(client.openaiAccounts, openaiAccount{
			name:   acc.Name,
			client: openai.NewClient(opts...),
			model:  model,
		})
	}

	// Set provider based on what's available
	if len(client.openaiAccounts) > 0 {
		client.provider = ProviderOpenAI
	} else if client.cliProvider != ProviderNone {
		client.provider = client.cliProvider
	} else {
		client.provider = ProviderNone
	}

	return client
}

// Available returns true if an AI provider is available
func (c *Client) Available() bool {
	return c.provider != ProviderNone
}

// Provider returns the detected provider name with model info
func (c *Client) Provider() string {
	switch c.provider {
	case ProviderOpenAI:
		if len(c.openaiAccounts) > 0 {
			acc := c.openaiAccounts[0]
			if acc.name != "" {
				return acc.name + "/" + acc.model
			}
			return acc.model
		}
		return "openai"
	case ProviderClaude:
		return "claude-haiku"
	case ProviderCodex:
		return "codex"
	case ProviderGemini:
		return "gemini-2.5-flash"
	case ProviderVibe:
		return "vibe"
	case ProviderOllama:
		return "llama3.2:3b"
	default:
		return string(c.provider)
	}
}

// maxAPIRetries is the maximum number of API accounts to try before giving up
const maxAPIRetries = 3

// Call executes a prompt using available AI providers and returns the response
// For OpenAI accounts, tries up to 3 in order until one succeeds
func (c *Client) Call(prompt string) (string, error) {
	if c.provider == ProviderNone {
		return "", errors.New("no AI provider found - configure ai_accounts in config.yml or install claude, codex, gemini, vibe, or ollama")
	}

	// Try OpenAI accounts first (in order, max 3)
	if len(c.openaiAccounts) > 0 {
		var failedAccounts []string
		limit := len(c.openaiAccounts)
		if limit > maxAPIRetries {
			limit = maxAPIRetries
		}

		for i := 0; i < limit; i++ {
			acc := c.openaiAccounts[i]
			result, err := callOpenAIClient(acc.client, acc.model, prompt)
			if err == nil {
				return result, nil
			}
			name := acc.name
			if name == "" {
				name = acc.model
			}
			failedAccounts = append(failedAccounts, name)
		}

		// All tried accounts failed
		return "", errors.New("AI API failed: your first " + strconv.Itoa(len(failedAccounts)) + " providers failed (" + strings.Join(failedAccounts, ", ") + "). Please check your API keys and endpoints in config.yml")
	}

	// Fall back to CLI
	return c.callCLI(prompt)
}

// callCLI executes a prompt using a CLI tool
func (c *Client) callCLI(prompt string) (string, error) {
	var cmd *exec.Cmd
	var parseFunc func(string) string

	switch c.cliProvider {
	case ProviderClaude:
		cmd = exec.Command("claude", "-p", prompt, "--model", "haiku", "--output-format", "json", "--no-session-persistence")
		parseFunc = parseClaudeOutput

	case ProviderCodex:
		cmd = exec.Command("codex", "exec", prompt, "--json")
		parseFunc = parseCodexOutput

	case ProviderGemini:
		cmd = exec.Command("gemini", "-p", prompt, "-m", "gemini-2.5-flash", "--output-format", "json")
		parseFunc = parseGeminiOutput

	case ProviderVibe:
		cmd = exec.Command("vibe", prompt)
		parseFunc = func(s string) string { return s }

	case ProviderOllama:
		cmd = exec.Command("ollama", "run", "llama3.2:3b", prompt)
		parseFunc = func(s string) string { return s }

	default:
		return "", errors.New("no CLI provider available")
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

// callOpenAIClient makes a call to an OpenAI-compatible API
func callOpenAIClient(client openai.Client, model, prompt string) (string, error) {
	ctx := context.Background()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", errors.New("OpenAI API call failed: " + err.Error())
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("OpenAI API returned no choices")
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

// detectProvider checks which AI CLI is available
func detectProvider() Provider {
	providers := []Provider{
		ProviderClaude,
		ProviderCodex,
		ProviderGemini,
		ProviderVibe,
		ProviderOllama,
	}

	for _, p := range providers {
		if commandExists(string(p)) {
			return p
		}
	}

	return ProviderNone
}

// commandExists checks if a command is available in PATH
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
