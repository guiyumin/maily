package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParsedEvent represents a calendar event parsed from natural language
type ParsedEvent struct {
	Title              string `json:"title"`
	StartTime          string `json:"start_time"`           // ISO 8601 format
	EndTime            string `json:"end_time"`             // ISO 8601 format
	Location           string `json:"location,omitempty"`
	AlarmMinutesBefore int    `json:"alarm_minutes_before"` // 0 means not specified
	AlarmSpecified     bool   `json:"alarm_specified"`      // true if user explicitly mentioned reminder
}

// ParseEventResponse parses the AI JSON response into a ParsedEvent
func ParseEventResponse(response string) (*ParsedEvent, error) {
	// Strip markdown code fences if present
	response = stripMarkdownCodeFences(response)

	var event ParsedEvent
	if err := json.Unmarshal([]byte(response), &event); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}
	return &event, nil
}

// stripMarkdownCodeFences removes ```json ... ``` wrappers from response
func stripMarkdownCodeFences(s string) string {
	s = strings.TrimSpace(s)

	// Remove opening fence (```json or ```)
	if strings.HasPrefix(s, "```") {
		// Find the end of the first line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
	}

	// Remove closing fence
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}

	return strings.TrimSpace(s)
}

// GetStartTime parses the start time string into time.Time
func (e *ParsedEvent) GetStartTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.StartTime)
}

// GetEndTime parses the end time string into time.Time
func (e *ParsedEvent) GetEndTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.EndTime)
}

// ParseCalendarEventPrompt builds a prompt for parsing natural language into a calendar event
func ParseCalendarEventPrompt(input string, now time.Time) string {
	return fmt.Sprintf(`Parse this natural language into a calendar event.

Current date/time: %s

Input: "%s"

Respond with ONLY a JSON object (no markdown, no explanation):
{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "location if mentioned, otherwise empty string",
  "alarm_minutes_before": 5,
  "alarm_specified": true
}

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no duration specified, default to 1 hour
- If user says "remind me X minutes before" or similar, set alarm_minutes_before and alarm_specified=true
- If no reminder mentioned, set alarm_minutes_before=0 and alarm_specified=false
- Extract location if mentioned (e.g., "at the coffee shop")
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"

Respond with ONLY the JSON, no other text.`, now.Format(time.RFC3339), input)
}

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
