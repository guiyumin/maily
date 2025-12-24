package ai

import "fmt"

// SummarizePrompt builds a prompt for email summarization
func SummarizePrompt(from, subject, body string) string {
	return fmt.Sprintf(`Summarize this email concisely (2-4 sentences). Include key points and any action items.

From: %s
Subject: %s

%s

Respond with only the summary, no preamble.`, from, subject, body)
}

// ExtractEventsPrompt builds a prompt for extracting calendar events from email
func ExtractEventsPrompt(from, subject, body string) string {
	return fmt.Sprintf(`Extract any calendar events, meetings, or deadlines from this email.

From: %s
Subject: %s

%s

If found, respond in this format:
Title: <event title>
Date: <date in YYYY-MM-DD format>
Time: <time in HH:MM format, 24h>
Duration: <duration like "1h" or "30m">

If no events found, respond with: No events found.

Respond with only the event details or "No events found", no preamble.`, from, subject, body)
}
