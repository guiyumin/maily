use async_openai::{
    config::OpenAIConfig,
    types::{ChatCompletionRequestMessage, ChatCompletionRequestUserMessageArgs, ChatCompletionRequestSystemMessageArgs, CreateChatCompletionRequestArgs},
    Client,
};
use rusqlite::params;
use serde::{Deserialize, Serialize};
use std::process::Command;

use crate::config::{get_config, AIProviderType};
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

/// AI completion request
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

/// Call AI provider for completion
pub fn complete(request: CompletionRequest) -> CompletionResponse {
    let config = match get_config() {
        Ok(c) => c,
        Err(e) => return CompletionResponse {
            success: false,
            content: None,
            error: Some(format!("Failed to load config: {}", e)),
            model_used: None,
        },
    };

    // Track errors from failed providers
    let mut last_error: Option<String> = None;
    let mut tried_providers: Vec<String> = Vec::new();

    // If a specific provider is requested, try it first
    if let Some(ref provider_name) = request.provider_name {
        // Check configured providers
        if let Some(provider) = config.ai_providers.iter().find(|p| &p.name == provider_name) {
            let result = match provider.provider_type {
                AIProviderType::Cli => call_cli_provider(&provider.name, &provider.model, &request),
                AIProviderType::Api => call_api_provider(&provider.name, &provider.model, &provider.base_url, &provider.api_key, &request),
            };
            if result.success {
                return CompletionResponse {
                    model_used: Some(format!("{}/{}", provider.name, provider.model)),
                    ..result
                };
            }
            tried_providers.push(format!("{}/{}", provider.name, provider.model));
            last_error = result.error;
        }

        // Check if it's a CLI tool
        let cli_tools = vec![
            ("claude", "haiku"),
            ("codex", "o4-mini"),
            ("gemini", "flash"),
            ("ollama", "llama3.2"),
        ];

        if let Some((_, model)) = cli_tools.iter().find(|(name, _)| name == provider_name) {
            if is_cli_available(provider_name) {
                let result = call_cli_provider(provider_name, model, &request);
                if result.success {
                    return CompletionResponse {
                        model_used: Some(format!("{}/{}", provider_name, model)),
                        ..result
                    };
                }
                tried_providers.push(format!("{}/{}", provider_name, model));
                last_error = result.error;
            }
        }
    }

    // Try configured providers in order
    for provider in &config.ai_providers {
        let provider_id = format!("{}/{}", provider.name, provider.model);
        // Skip if already tried
        if tried_providers.contains(&provider_id) {
            continue;
        }

        let result = match provider.provider_type {
            AIProviderType::Cli => call_cli_provider(&provider.name, &provider.model, &request),
            AIProviderType::Api => call_api_provider(&provider.name, &provider.model, &provider.base_url, &provider.api_key, &request),
        };

        if result.success {
            return CompletionResponse {
                model_used: Some(format!("{}/{}", provider.name, provider.model)),
                ..result
            };
        }
        tried_providers.push(provider_id);
        last_error = result.error;
    }

    // Try auto-detecting CLI tools
    let cli_tools = vec![
        ("claude", "haiku"),
        ("codex", "o4-mini"),
        ("gemini", "flash"),
        ("ollama", "llama3.2"),
    ];

    for (name, model) in cli_tools {
        let provider_id = format!("{}/{}", name, model);
        // Skip if already tried
        if tried_providers.contains(&provider_id) {
            continue;
        }

        if is_cli_available(name) {
            let result = call_cli_provider(name, model, &request);
            if result.success {
                return CompletionResponse {
                    model_used: Some(format!("{}/{}", name, model)),
                    ..result
                };
            }
            tried_providers.push(provider_id);
            last_error = result.error;
        }
    }

    // Return the last error if we tried any provider, otherwise generic message
    let error_msg = if let Some(err) = last_error {
        format!("AI provider failed: {}", err)
    } else if tried_providers.is_empty() {
        "No AI provider available. Please configure one in Settings.".to_string()
    } else {
        format!("All AI providers failed. Tried: {}", tried_providers.join(", "))
    };

    CompletionResponse {
        success: false,
        content: None,
        error: Some(error_msg),
        model_used: None,
    }
}

/// Test a specific AI provider
pub fn test_provider(provider_name: &str, provider_model: &str, provider_type: &str, base_url: &str, api_key: &str) -> CompletionResponse {
    let request = CompletionRequest {
        prompt: "Say hello.".to_string(),
        system_prompt: None,
        max_tokens: Some(200),
        provider_name: None,
    };

    if provider_type == "cli" {
        call_cli_provider(provider_name, provider_model, &request)
    } else {
        call_api_provider(provider_name, provider_model, base_url, api_key, &request)
    }
}

/// Check if any AI provider is available (without making an API call)
pub fn has_available_provider() -> bool {
    !list_available_providers().is_empty()
}

/// List all available AI providers
pub fn list_available_providers() -> Vec<String> {
    let mut providers = Vec::new();

    // Add configured providers
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

fn call_api_provider(name: &str, model: &str, base_url: &str, api_key: &str, request: &CompletionRequest) -> CompletionResponse {
    // Use async-openai SDK for proper OpenAI-compatible API handling

    let config = OpenAIConfig::new()
        .with_api_key(api_key)
        .with_api_base(base_url);

    let client: Client<OpenAIConfig> = Client::with_config(config);

    // Build messages
    let mut messages: Vec<ChatCompletionRequestMessage> = vec![];

    if let Some(ref sys) = request.system_prompt {
        if let Ok(msg) = ChatCompletionRequestSystemMessageArgs::default()
            .content(sys.clone())
            .build()
        {
            messages.push(msg.into());
        }
    }

    if let Ok(msg) = ChatCompletionRequestUserMessageArgs::default()
        .content(request.prompt.clone())
        .build()
    {
        messages.push(msg.into());
    }

    // Build request
    let mut req_builder = CreateChatCompletionRequestArgs::default();
    req_builder.model(model).messages(messages);

    if let Some(max_tokens) = request.max_tokens {
        req_builder.max_completion_tokens(max_tokens as u32);
    }

    let chat_request = match req_builder.build() {
        Ok(r) => r,
        Err(e) => {
            return CompletionResponse {
                success: false,
                content: None,
                error: Some(format!("Failed to build request: {}", e)),
                model_used: None,
            }
        }
    };

    // Run async request in blocking context
    let runtime = match tokio::runtime::Handle::try_current() {
        Ok(handle) => {
            // We're already in a tokio runtime, use block_in_place
            let result = tokio::task::block_in_place(|| {
                handle.block_on(client.chat().create(chat_request))
            });
            return handle_api_response(result, name, model);
        }
        Err(_) => {
            // Create a new runtime for blocking call
            match tokio::runtime::Runtime::new() {
                Ok(rt) => rt,
                Err(e) => {
                    return CompletionResponse {
                        success: false,
                        content: None,
                        error: Some(format!("Failed to create runtime: {}", e)),
                        model_used: None,
                    }
                }
            }
        }
    };

    let result = runtime.block_on(client.chat().create(chat_request));
    handle_api_response(result, name, model)
}

fn handle_api_response(
    result: Result<async_openai::types::CreateChatCompletionResponse, async_openai::error::OpenAIError>,
    name: &str,
    model: &str,
) -> CompletionResponse {
    match result {
        Ok(response) => {
            if let Some(choice) = response.choices.first() {
                // Try content first, then check for refusal
                let content = choice.message.content.clone()
                    .or_else(|| choice.message.refusal.clone());

                if let Some(content) = content {
                    if content.is_empty() {
                        CompletionResponse {
                            success: false,
                            content: None,
                            error: Some("Response content is empty".to_string()),
                            model_used: None,
                        }
                    } else {
                        CompletionResponse {
                            success: true,
                            content: Some(content),
                            error: None,
                            model_used: Some(format!("{}/{}", name, model)),
                        }
                    }
                } else {
                    CompletionResponse {
                        success: false,
                        content: None,
                        error: Some(format!("Response has no content. Finish reason: {:?}", choice.finish_reason)),
                        model_used: None,
                    }
                }
            } else {
                CompletionResponse {
                    success: false,
                    content: None,
                    error: Some("API returned no choices".to_string()),
                    model_used: None,
                }
            }
        }
        Err(e) => {
            CompletionResponse {
                success: false,
                content: None,
                error: Some(e.to_string()),
                model_used: None,
            }
        }
    }
}

/// Summarize email content
pub fn summarize_email(
    account: &str,
    mailbox: &str,
    uid: u32,
    subject: &str,
    from: &str,
    body_text: &str,
    force_refresh: bool,
) -> CompletionResponse {
    // Check cache first (unless force refresh)
    if !force_refresh {
        if let Some(cached) = get_cached_summary(account, mailbox, uid) {
            return CompletionResponse {
                success: true,
                content: Some(cached.summary),
                error: None,
                model_used: Some(cached.model_used),
            };
        }
    }

    // Truncate body if too long
    let body_truncated: String = body_text.chars().take(4000).collect();

    let prompt = format!(
        r#"Summarize this email as bullet points.

From: {}
Subject: {}

{}

Format your response exactly like this (skip sections if not applicable):

Summary:
    <one sentence summary>

Key Points:
    - <point 1>
    - <point 2>

Action Items:
    - <action 1>
    - <action 2>

Dates/Deadlines:
    - <date/deadline if mentioned>

Keep it brief. No preamble, section titles on their own line, content indented with 4 spaces."#,
        from, subject, body_truncated
    );

    let response = complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(500),
        provider_name: None,
    });

    // Cache successful response
    if response.success {
        if let Some(ref content) = response.content {
            let now = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs() as i64;

            let summary = EmailSummary {
                email_uid: uid,
                account: account.to_string(),
                mailbox: mailbox.to_string(),
                summary: content.clone(),
                model_used: response.model_used.clone().unwrap_or_default(),
                created_at: now,
            };

            let _ = save_summary(&summary);
        }
    }

    response
}

/// Generate tags for an email using AI
pub fn generate_email_tags(
    from: &str,
    subject: &str,
    body_text: &str,
) -> Result<Vec<String>, String> {
    // Truncate body if too long
    let body_truncated: String = body_text.chars().take(2000).collect();

    let prompt = format!(
        r#"Analyze this email and suggest 1-5 short tags to categorize it.

From: {}
Subject: {}

{}

Return ONLY a comma-separated list of short tags (1-2 words each). Examples: work, urgent, newsletter, receipt, travel, meeting, personal, finance, shipping, social

Tags:"#,
        from, subject, body_truncated
    );

    let response = complete(CompletionRequest {
        prompt,
        system_prompt: Some("You are a helpful assistant that categorizes emails with short, descriptive tags. Only output comma-separated tags, nothing else.".to_string()),
        max_tokens: Some(100),
        provider_name: None,
    });

    if response.success {
        if let Some(content) = response.content {
            // Parse comma-separated tags
            let tags: Vec<String> = content
                .split(',')
                .map(|s| s.trim().to_lowercase())
                .filter(|s| !s.is_empty() && s.len() <= 30)
                .take(5)
                .collect();

            if tags.is_empty() {
                return Err("AI returned no valid tags".to_string());
            }
            return Ok(tags);
        }
    }

    Err(response.error.unwrap_or_else(|| "Failed to generate tags".to_string()))
}

/// Generate smart reply suggestions
pub fn generate_reply(
    original_from: &str,
    original_subject: &str,
    original_body: &str,
    reply_intent: &str, // e.g., "accept", "decline", "ask for more info"
) -> CompletionResponse {
    let body_truncated: String = original_body.chars().take(2000).collect();

    let prompt = format!(
        r#"Generate a professional email reply. The intent is to: {}

Original email:
From: {}
Subject: {}
Body: {}

Write a concise, professional reply:"#,
        reply_intent, original_from, original_subject, body_truncated
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: Some("You are a professional email writer. Write concise, clear, and polite emails.".to_string()),
        max_tokens: Some(500),
        provider_name: None,
    })
}

/// Extract calendar event from email
pub fn extract_event(
    from: &str,
    subject: &str,
    body_text: &str,
) -> CompletionResponse {
    let body_truncated: String = body_text.chars().take(3000).collect();

    // Get current time in RFC3339 format
    let now = chrono::Local::now().format("%Y-%m-%dT%H:%M:%S%:z").to_string();

    let prompt = format!(
        r#"Extract the most relevant calendar event, meeting, or deadline from this email.

Current date/time: {}

From: {}
Subject: {}

{}

If an event is found, respond with ONLY a JSON object (no markdown, no explanation):
{{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "physical location OR meeting URL",
  "notes": "agenda, description, or other relevant details from email",
  "alarm_minutes_before": 0,
  "alarm_specified": false
}}

If NO events found, respond with exactly: NO_EVENTS_FOUND

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no end time/duration specified, default to 1 hour after start
- Location priority: use physical address if mentioned; if no physical location but there's a virtual meeting link (Zoom, Google Meet, Microsoft Teams, Webex), put the meeting URL in location
- Extract notes: include agenda, description, or other relevant context from the email
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"
- Pick the most important/relevant event if multiple are mentioned
- Set alarm_minutes_before=0 and alarm_specified=false (user will set reminder later)

Respond with ONLY the JSON or NO_EVENTS_FOUND, no other text."#,
        now, from, subject, body_truncated
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(300),
        provider_name: None,
    })
}

/// Extract actionable reminder/task from email
pub fn extract_reminder(
    from: &str,
    subject: &str,
    body_text: &str,
) -> CompletionResponse {
    let body_truncated: String = body_text.chars().take(3000).collect();

    // Get current time for relative date parsing
    let now = chrono::Local::now().format("%Y-%m-%dT%H:%M:%S%:z").to_string();

    let prompt = format!(
        r#"Extract an actionable task or follow-up from this email.

Current date/time: {}

From: {}
Subject: {}

{}

If a task/action is found, respond with ONLY a JSON object (no markdown, no explanation):
{{
  "title": "brief actionable task title (start with verb)",
  "notes": "relevant context from email",
  "due_date": "2024-12-25T09:00:00-08:00",
  "priority": 5
}}

If NO actionable task found, respond with exactly: NO_TASK_FOUND

Rules:
- title: Start with action verb (Reply to, Review, Send, Schedule, Follow up with, etc.)
- title: Keep it brief (under 50 chars), include person/company name if relevant
- notes: Include key context (what specifically needs to be done, deadline mentioned)
- due_date: RFC3339 format. If deadline mentioned, use it. If "ASAP" or urgent, use today. Otherwise default to tomorrow 9am.
- priority: 1=high (urgent/ASAP), 5=medium (normal), 9=low (whenever)

Examples of good titles:
- "Reply to John about Q4 budget"
- "Review contract from Acme Corp"
- "Send invoice to client"
- "Schedule call with Sarah"

Respond with ONLY the JSON or NO_TASK_FOUND, no other text."#,
        now, from, subject, body_truncated
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(300),
        provider_name: None,
    })
}

/// Parse natural language into a calendar event, with optional email context
pub fn parse_event_nlp(
    user_input: &str,
    email_from: &str,
    email_subject: &str,
    email_body: &str,
) -> CompletionResponse {
    let now = chrono::Local::now().format("%Y-%m-%dT%H:%M:%S%:z").to_string();

    // Build email context if provided
    let email_context = if !email_from.is_empty() || !email_subject.is_empty() || !email_body.is_empty() {
        let body_truncated: String = email_body.chars().take(1000).collect();
        format!(
            r#"
Email context (use this to understand references like "them", "the meeting", etc.):
From: {}
Subject: {}
Body: {}

"#,
            email_from, email_subject, body_truncated
        )
    } else {
        String::new()
    };

    let prompt = format!(
        r#"Parse this natural language into a calendar event.

Current date/time: {}
{}
User input: "{}"

Respond with ONLY a JSON object (no markdown, no explanation):
{{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "physical location OR meeting URL",
  "notes": "additional details, agenda, description",
  "alarm_minutes_before": 5,
  "alarm_specified": true
}}

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no duration specified, default to 1 hour
- If user says "remind me X minutes before" or similar, set alarm_minutes_before and alarm_specified=true
- If no reminder mentioned, set alarm_minutes_before=0 and alarm_specified=false
- Location priority: use physical address if mentioned; if no physical location but there's a virtual meeting link (Zoom, Google Meet, Microsoft Teams, Webex), put the meeting URL in location
- Extract notes: any additional details, agenda items, descriptions, or context
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"
- Use the email context to resolve references (e.g., "them" = sender, "the meeting" = subject)

Respond with ONLY the JSON, no other text."#,
        now, email_context, user_input
    );

    complete(CompletionRequest {
        prompt,
        system_prompt: None,
        max_tokens: Some(300),
        provider_name: None,
    })
}
