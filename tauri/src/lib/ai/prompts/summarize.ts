/**
 * Email summarization prompt
 */

export interface SummarizeEmailParams {
  from: string;
  subject: string;
  bodyText: string;
}

export function buildSummarizePrompt(params: SummarizeEmailParams): string {
  const bodyTruncated = params.bodyText.slice(0, 4000);

  return `Summarize this email.

From: ${params.from}
Subject: ${params.subject}

${bodyTruncated}

Format (skip sections if not applicable):

Summary:
    <1-2 sentences: what is this about>

Action Items:
    - <what needs to be done, by whom, by when>

People/Contacts:
    - <names and roles mentioned>

Links/References:
    - <URLs, documents, or attachments mentioned>

No preamble. Be concise.`;
}

export const SUMMARIZE_MAX_TOKENS = 5000;
