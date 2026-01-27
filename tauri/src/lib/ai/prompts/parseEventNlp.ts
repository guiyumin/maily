/**
 * Natural language event parsing prompt
 */

export interface ParseEventNlpParams {
  userInput: string;
  emailFrom?: string;
  emailSubject?: string;
  emailBody?: string;
}

export function buildParseEventNlpPrompt(params: ParseEventNlpParams): string {
  const now = new Date().toISOString();

  // Build email context if provided
  let emailContext = "";
  if (params.emailFrom || params.emailSubject || params.emailBody) {
    const bodyTruncated = (params.emailBody || "").slice(0, 1000);
    emailContext = `
Email context (use this to understand references like "them", "the meeting", etc.):
From: ${params.emailFrom || ""}
Subject: ${params.emailSubject || ""}
Body: ${bodyTruncated}

`;
  }

  return `Parse this natural language into a calendar event.

Current date/time: ${now}
${emailContext}
User input: "${params.userInput}"

Respond with ONLY a JSON object (no markdown, no explanation):
{
  "title": "event title",
  "start_time": "2024-12-25T10:00:00-08:00",
  "end_time": "2024-12-25T11:00:00-08:00",
  "location": "physical location OR meeting URL",
  "notes": "additional details, agenda, description",
  "alarm_minutes_before": 5,
  "alarm_specified": true
}

Rules:
- start_time and end_time must be in RFC3339 format with timezone
- If no duration specified, default to 1 hour
- If user says "remind me X minutes before" or similar, set alarm_minutes_before and alarm_specified=true
- If no reminder mentioned, set alarm_minutes_before=0 and alarm_specified=false
- Location priority: use physical address if mentioned; if no physical location but there's a virtual meeting link (Zoom, Google Meet, Microsoft Teams, Webex), put the meeting URL in location
- Extract notes: any additional details, agenda items, descriptions, or context
- Use the current date/time to interpret relative dates like "tomorrow", "next Monday"
- Use the email context to resolve references (e.g., "them" = sender, "the meeting" = subject)

Respond with ONLY the JSON, no other text.`;
}

export const PARSE_EVENT_NLP_MAX_TOKENS = 1000;
