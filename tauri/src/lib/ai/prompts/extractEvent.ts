/**
 * Calendar event extraction prompt
 */

export interface ExtractEventParams {
  from: string;
  subject: string;
  bodyText: string;
  userTimezone: string;
}

export function buildExtractEventPrompt(params: ExtractEventParams): string {
  const bodyTruncated = params.bodyText.slice(0, 4000);
  const now = new Date().toISOString();

  return `Current time: ${now}
User timezone: ${params.userTimezone}

IMPORTANT: Focus on the NEWEST message only. Ignore quoted replies, forwarded content, and previous messages in the thread.

EMAIL:
From: ${params.from}
Subject: ${params.subject}

${bodyTruncated}

---

Extract a calendar event from the NEWEST part of this email. Look for:
- Meetings, calls, appointments
- Webinars, conferences, events
- Deadlines with specific dates/times
- Any scheduled activity

Return JSON (no markdown):
{"title":"...","start_time":"2024-12-25T10:00:00-08:00","end_time":"2024-12-25T11:00:00-08:00","location":"...","notes":"..."}

Rules:
- Convert ALL times to user's timezone (${params.userTimezone})
- Use RFC3339 format for times
- Default to 1 hour duration if not specified
- Include meeting URLs (Zoom/Meet/Teams) in location
- If NO event found, respond: NO_EVENTS_FOUND`;
}

export const EXTRACT_EVENT_SYSTEM_PROMPT =
  "You are an expert at extracting calendar events from emails. Be aggressive - if there's ANY mention of a date/time with an activity, extract it. Always output valid JSON or NO_EVENTS_FOUND.";

export const EXTRACT_EVENT_MAX_TOKENS = 2000;
