package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
)

// Provider represents an AI CLI provider
type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
	ProviderGemini Provider = "gemini"
	ProviderVibe   Provider = "vibe"
	ProviderOllama Provider = "ollama"
	ProviderNone   Provider = ""
)

// Client handles AI operations using available CLI tools
type Client struct {
	provider Provider
}

// NewClient creates a new AI client, auto-detecting available CLI
func NewClient() *Client {
	return &Client{
		provider: detectProvider(),
	}
}

// Available returns true if an AI CLI is available
func (c *Client) Available() bool {
	return c.provider != ProviderNone
}

// Provider returns the detected provider name
func (c *Client) Provider() string {
	return string(c.provider)
}

// Call executes a prompt using the detected AI CLI and returns the response
func (c *Client) Call(prompt string) (string, error) {
	if c.provider == ProviderNone {
		return "", errors.New("no AI CLI found - install claude, codex, gemini, vibe, or ollama")
	}

	var cmd *exec.Cmd
	var parseFunc func(string) string

	switch c.provider {
	case ProviderClaude:
		// claude -p "prompt" --output-format json outputs JSON with result field
		cmd = exec.Command("claude", "-p", prompt, "--output-format", "json")
		parseFunc = parseClaudeOutput

	case ProviderCodex:
		// codex exec "prompt" --json outputs JSONL, need to parse agent_message
		cmd = exec.Command("codex", "exec", prompt, "--json")
		parseFunc = parseCodexOutput

	case ProviderGemini:
		// gemini -p "prompt"
		cmd = exec.Command("gemini", "-p", prompt)
		parseFunc = func(s string) string { return s }

	case ProviderVibe:
		// vibe "prompt" (direct argument, non-interactive mode)
		cmd = exec.Command("vibe", prompt)
		parseFunc = func(s string) string { return s }

	case ProviderOllama:
		// ollama run model "prompt"
		cmd = exec.Command("ollama", "run", "llama3.2:3b", prompt)
		parseFunc = func(s string) string { return s }

	default:
		return "", errors.New("unknown AI provider")
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

// parseClaudeOutput extracts the result from Claude JSON output
// Claude outputs: {"type":"result","result":"..."}
func parseClaudeOutput(output string) string {
	var response struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}

	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return output // fallback to raw output
	}

	return response.Result
}

// parseCodexOutput extracts the agent message from codex JSON output
// Codex outputs JSONL with events like:
// {"type":"item.completed","item":{"type":"agent_message","text":"..."}}
func parseCodexOutput(output string) string {
	lines := strings.Split(output, "\n")

	// Look for the last agent_message
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
	// Check in order of preference
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
