package ai

import "fmt"

// SummarizePrompt builds a prompt for email summarization
func SummarizePrompt(from, subject, body string) string {
	return fmt.Sprintf(`Summarize this email as bullet points.

From: %s
Subject: %s

%s

Format your response exactly like this (skip sections if not applicable):

• Summary: <one sentence summary>
• Key points:
  - <point 1>
  - <point 2>
• Action items:
  - <action 1>
  - <action 2>
• Dates/Deadlines:
  - <date/deadline if mentioned>

Keep it brief. No preamble, just the bullet points.`, from, subject, body)
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
