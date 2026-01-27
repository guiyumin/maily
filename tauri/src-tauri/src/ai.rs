use rusqlite::params;
use serde::{Deserialize, Serialize};
use std::process::Command;

use crate::config::get_config;
use crate::mail::DB;

/// Email summary stored in database
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct EmailSummary {
    pub email_uid: u32,
    pub account: String,
    pub mailbox: String,
    pub summary: String,
    pub model_used: String,
    pub created_at: i64,
}

/// AI completion request (frontend uses this for CLI providers)
#[derive(Debug, Serialize, Deserialize)]
pub struct CompletionRequest {
    pub prompt: String,
    pub system_prompt: Option<String>,
    pub max_tokens: Option<u32>,
    /// Optional provider name to use (if not specified, uses first available)
    pub provider_name: Option<String>,
}

/// AI completion response
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionResponse {
    pub success: bool,
    pub content: Option<String>,
    pub error: Option<String>,
    pub model_used: Option<String>,
}

// Initialize summaries table
pub fn init_summaries_table() {
    let conn = DB.lock().unwrap();
    conn.execute_batch(r#"
        CREATE TABLE IF NOT EXISTS email_summaries (
            account TEXT NOT NULL,
            mailbox TEXT NOT NULL,
            email_uid INTEGER NOT NULL,
            summary TEXT NOT NULL,
            model_used TEXT NOT NULL,
            created_at INTEGER NOT NULL,
            PRIMARY KEY (account, mailbox, email_uid)
        );
    "#).expect("Failed to create summaries table");
}

/// Get cached summary for an email
pub fn get_cached_summary(account: &str, mailbox: &str, uid: u32) -> Option<EmailSummary> {
    let conn = DB.lock().unwrap();
    let mut stmt = conn.prepare(
        "SELECT summary, model_used, created_at FROM email_summaries WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3"
    ).ok()?;

    stmt.query_row(params![account, mailbox, uid], |row| {
        Ok(EmailSummary {
            email_uid: uid,
            account: account.to_string(),
            mailbox: mailbox.to_string(),
            summary: row.get(0)?,
            model_used: row.get(1)?,
            created_at: row.get(2)?,
        })
    }).ok()
}

/// Save summary to cache
pub fn save_summary(summary: &EmailSummary) -> Result<(), Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();
    conn.execute(
        "INSERT OR REPLACE INTO email_summaries (account, mailbox, email_uid, summary, model_used, created_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
        params![
            summary.account,
            summary.mailbox,
            summary.email_uid,
            summary.summary,
            summary.model_used,
            summary.created_at
        ]
    )?;
    Ok(())
}

/// Delete cached summary
pub fn delete_summary(account: &str, mailbox: &str, uid: u32) -> Result<(), Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();
    conn.execute(
        "DELETE FROM email_summaries WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3",
        params![account, mailbox, uid]
    )?;
    Ok(())
}

/// List all available AI providers (CLI tools that are installed)
pub fn list_available_providers() -> Vec<String> {
    let mut providers = Vec::new();

    // Add configured providers from config
    if let Ok(config) = get_config() {
        for provider in &config.ai_providers {
            providers.push(provider.name.clone());
        }
    }

    // Add available CLI tools
    let cli_tools = ["claude", "codex", "gemini", "ollama"];
    for tool in cli_tools {
        if is_cli_available(tool) && !providers.contains(&tool.to_string()) {
            providers.push(tool.to_string());
        }
    }

    providers
}

fn is_cli_available(name: &str) -> bool {
    Command::new("which")
        .arg(name)
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

/// Direct CLI completion for frontend - bypasses provider selection
/// Used when frontend JS SDKs handle API providers, only CLI goes through Rust
pub fn cli_complete(request: CompletionRequest, provider_name: &str, provider_model: &str) -> CompletionResponse {
    if !is_cli_available(provider_name) {
        return CompletionResponse {
            success: false,
            content: None,
            error: Some(format!("CLI tool '{}' not found in PATH", provider_name)),
            model_used: None,
        };
    }
    call_cli_provider(provider_name, provider_model, &request)
}

/// Save email summary from frontend (used when frontend generates summaries)
pub fn save_summary_from_frontend(
    account: &str,
    mailbox: &str,
    uid: u32,
    summary_text: &str,
    model_used: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as i64;

    let summary = EmailSummary {
        email_uid: uid,
        account: account.to_string(),
        mailbox: mailbox.to_string(),
        summary: summary_text.to_string(),
        model_used: model_used.to_string(),
        created_at: now,
    };

    save_summary(&summary)
}

fn call_cli_provider(name: &str, model: &str, request: &CompletionRequest) -> CompletionResponse {
    let prompt = if let Some(ref sys) = request.system_prompt {
        format!("{}\n\n{}", sys, request.prompt)
    } else {
        request.prompt.clone()
    };

    let result = match name {
        "claude" => {
            // Claude Code CLI: -p for prompt, --model after, --output-format json for structured parsing
            let output = Command::new("claude")
                .args(["-p", &prompt, "--model", model, "--output-format", "json"])
                .output();

            // Parse JSON response to extract result field
            return match output {
                Ok(out) if out.status.success() => {
                    let stdout = String::from_utf8_lossy(&out.stdout);
                    // Parse JSON and extract "result" field
                    if let Ok(json) = serde_json::from_str::<serde_json::Value>(&stdout) {
                        let is_error = json.get("is_error").and_then(|v| v.as_bool()).unwrap_or(false);
                        if is_error {
                            CompletionResponse {
                                success: false,
                                content: None,
                                error: json.get("result").and_then(|v| v.as_str()).map(|s| s.to_string()),
                                model_used: None,
                            }
                        } else {
                            CompletionResponse {
                                success: true,
                                content: json.get("result").and_then(|v| v.as_str()).map(|s| s.to_string()),
                                error: None,
                                model_used: Some(format!("{}/{}", name, model)),
                            }
                        }
                    } else {
                        // Fallback: use raw stdout if JSON parsing fails
                        CompletionResponse {
                            success: true,
                            content: Some(stdout.trim().to_string()),
                            error: None,
                            model_used: Some(format!("{}/{}", name, model)),
                        }
                    }
                }
                Ok(out) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(String::from_utf8_lossy(&out.stderr).to_string()),
                    model_used: None,
                },
                Err(e) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(e.to_string()),
                    model_used: None,
                },
            };
        }
        "codex" => {
            // Codex CLI: codex exec "prompt" --json returns NDJSON stream
            let output = Command::new("codex")
                .args(["exec", &prompt, "--json"])
                .output();

            // Parse NDJSON to find agent_message item
            return match output {
                Ok(out) if out.status.success() => {
                    let stdout = String::from_utf8_lossy(&out.stdout);
                    // Find the agent_message line in NDJSON stream
                    let mut result_text: Option<String> = None;
                    for line in stdout.lines() {
                        if let Ok(json) = serde_json::from_str::<serde_json::Value>(line) {
                            if json.get("type").and_then(|v| v.as_str()) == Some("item.completed") {
                                if let Some(item) = json.get("item") {
                                    if item.get("type").and_then(|v| v.as_str()) == Some("agent_message") {
                                        result_text = item.get("text").and_then(|v| v.as_str()).map(|s| s.to_string());
                                    }
                                }
                            }
                        }
                    }
                    if let Some(text) = result_text {
                        CompletionResponse {
                            success: true,
                            content: Some(text),
                            error: None,
                            model_used: Some(format!("codex/{}", model)),
                        }
                    } else {
                        CompletionResponse {
                            success: false,
                            content: None,
                            error: Some("No agent_message found in codex output".to_string()),
                            model_used: None,
                        }
                    }
                }
                Ok(out) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(String::from_utf8_lossy(&out.stderr).to_string()),
                    model_used: None,
                },
                Err(e) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(e.to_string()),
                    model_used: None,
                },
            };
        }
        "gemini" => {
            // Gemini CLI: positional prompt, -m for model, -o json for output
            let output = Command::new("gemini")
                .args([&prompt, "-m", model, "-o", "json"])
                .output();

            // Parse JSON to extract response field
            return match output {
                Ok(out) if out.status.success() => {
                    let stdout = String::from_utf8_lossy(&out.stdout);
                    // Find JSON object (skip any non-JSON lines like "Loaded cached credentials.")
                    let json_start = stdout.find('{');
                    if let Some(start) = json_start {
                        if let Ok(json) = serde_json::from_str::<serde_json::Value>(&stdout[start..]) {
                            if let Some(response) = json.get("response").and_then(|v| v.as_str()) {
                                return CompletionResponse {
                                    success: true,
                                    content: Some(response.to_string()),
                                    error: None,
                                    model_used: Some(format!("gemini/{}", model)),
                                };
                            }
                        }
                    }
                    // Fallback: use raw stdout
                    CompletionResponse {
                        success: true,
                        content: Some(stdout.trim().to_string()),
                        error: None,
                        model_used: Some(format!("gemini/{}", model)),
                    }
                }
                Ok(out) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(String::from_utf8_lossy(&out.stderr).to_string()),
                    model_used: None,
                },
                Err(e) => CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(e.to_string()),
                    model_used: None,
                },
            };
        }
        "ollama" => {
            Command::new("ollama")
                .args(["run", model, &prompt])
                .output()
        }
        _ => {
            // Generic CLI that accepts -p for prompt
            Command::new(name)
                .args(["-m", model, "-p", &prompt])
                .output()
        }
    };

    match result {
        Ok(output) => {
            if output.status.success() {
                let content = String::from_utf8_lossy(&output.stdout).trim().to_string();
                CompletionResponse {
                    success: true,
                    content: Some(content),
                    error: None,
                    model_used: Some(format!("{}/{}", name, model)),
                }
            } else {
                let stderr = String::from_utf8_lossy(&output.stderr).to_string();
                CompletionResponse {
                    success: false,
                    content: None,
                    error: Some(stderr),
                    model_used: None,
                }
            }
        }
        Err(e) => CompletionResponse {
            success: false,
            content: None,
            error: Some(e.to_string()),
            model_used: None,
        },
    }
}
