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
	s = strings.TrimSuffix(s, "```")

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
	return ParseCalendarEventWithContextPrompt(input, "", "", "", now)
}

// ParseCalendarEventWithContextPrompt builds a prompt for parsing natural language into a calendar event,
// with optional email context to resolve references like "them", "the meeting", etc.
func ParseCalendarEventWithContextPrompt(input, emailFrom, emailSubject, emailBody string, now time.Time) string {
	emailContext := ""
	if emailFrom != "" || emailSubject != "" || emailBody != "" {
		// Truncate body if too long
		body := emailBody
		if len(body) > 1000 {
			body = body[:1000] + "..."
		}
		emailContext = fmt.Sprintf(`
Email context (use this to understand references like "them", "the meeting", etc.):
From: %s
Subject: %s
Body: %s

`, emailFrom, emailSubject, body)
	}

	return fmt.Sprintf(`Parse this natural language into a calendar event.

Current date/time: %s
%s
User input: "%s"

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
- Use the email context to resolve references (e.g., "them" = sender, "the meeting" = subject)

Respond with ONLY the JSON, no other text.`, now.Format(time.RFC3339), emailContext, input)
}

// SummarizePrompt builds a prompt for email summarization
func SummarizePrompt(from, subject, body string) string {
	return fmt.Sprintf(`Summarize this email as bullet points.

From: %s
Subject: %s

%s

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

Keep it brief. No preamble, section titles on their own line, content indented with 4 spaces.`, from, subject, body)
}

// ExtractEventsPrompt builds a prompt for extracting calendar events from email
func ExtractEventsPrompt(from, subject, body string, now time.Time) string {
	return fmt.Sprintf(`Extract the most relevant calendar event, meeting, or deadline from this email.

Current date/time: %s

From: %s
Subject: %s

%s

If an event is found, respond with ONLY a JSON object (no markdown, no explanation):
{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "location if mentioned, otherwise empty string",
  "alarm_minutes_before": 0,
  "alarm_specified": false
}

If NO events found, respond with exactly: NO_EVENTS_FOUND

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no end time/duration specified, default to 1 hour after start
- Extract location if mentioned
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"
- Pick the most important/relevant event if multiple are mentioned
- Set alarm_minutes_before=0 and alarm_specified=false (user will set reminder later)

Respond with ONLY the JSON or NO_EVENTS_FOUND, no other text.`, now.Format(time.RFC3339), from, subject, body)
}
