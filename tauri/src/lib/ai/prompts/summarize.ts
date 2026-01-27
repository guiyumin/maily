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

  return `Summarize this email comprehensively. Preserve all important details.

From: ${params.from}
Subject: ${params.subject}

${bodyTruncated}

Format your response exactly like this (skip sections if not applicable):

Summary:
    <2-3 sentence summary capturing the main purpose and context>

Key Points:
    - <include ALL important points mentioned>
    - <preserve specific details: names, numbers, amounts, URLs>
    - <don't omit information that might be needed later>

Action Items:
    - <any actions requested or expected>
    - <include who needs to do what>

Dates/Deadlines:
    - <ALL dates, times, and deadlines mentioned>
    - <include timezone if specified>

People/Contacts:
    - <names and roles mentioned>

Links/References:
    - <any URLs, document references, or attachments mentioned>

Be thorough. Include all details that could be useful. No preamble, section titles on their own line, content indented with 4 spaces.`;
}

export const SUMMARIZE_MAX_TOKENS = 5000;
