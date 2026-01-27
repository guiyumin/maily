/**
 * Reminder extraction prompt
 */

export interface ExtractReminderParams {
  from: string;
  subject: string;
  bodyText: string;
}

export function buildExtractReminderPrompt(params: ExtractReminderParams): string {
  const bodyTruncated = params.bodyText.slice(0, 4000);
  const now = new Date().toISOString();

  return `Current time: ${now}

EMAIL:
From: ${params.from}
Subject: ${params.subject}

${bodyTruncated}

---

Extract an actionable task from this email. Look for:
- Requests that need a response
- Documents to review/sign
- Deadlines to meet
- Follow-ups needed
- Action items mentioned

Return JSON (no markdown):
{"title":"Reply to John about budget","notes":"context here","due_date":"2024-12-25T09:00:00-08:00","priority":5}

Rules:
- title: Start with verb (Reply, Review, Send, Schedule, Follow up, etc.)
- due_date: RFC3339 format. Use deadline if mentioned, otherwise tomorrow 9am
- priority: 1=urgent, 5=normal, 9=low
- If NO task found, respond: NO_TASK_FOUND`;
}

export const EXTRACT_REMINDER_SYSTEM_PROMPT =
  "You are an expert at identifying actionable tasks in emails. Be aggressive - most emails that aren't pure newsletters have some action needed. Always output valid JSON or NO_TASK_FOUND.";

export const EXTRACT_REMINDER_MAX_TOKENS = 2000;
