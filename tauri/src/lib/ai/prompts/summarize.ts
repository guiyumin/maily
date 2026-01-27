/**
 * Email summarization prompt
 */

export interface SummarizeEmailParams {
  from: string;
  subject: string;
  bodyText: string;
  timezone: string;
}

export function buildSummarizePrompt(params: SummarizeEmailParams): string {
  return `Summarize this email with bullet points. User's timezone: ${params.timezone}

From: ${params.from}
Subject: ${params.subject}

${params.bodyText}

Format (skip sections if not applicable):

Summary:
    <1-2 bullet points: what is this about>

Action Items:
    - <what needs to be done, by whom, by when (convert to user's timezone)>

People/Contacts:
    - <names and roles mentioned>

Links/References:
    - <URLs, documents, or attachments mentioned>

No preamble. Be concise.`;
}

export const SUMMARIZE_MAX_TOKENS = 5000;
