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
  return `Summarize the NEWEST/MOST RECENT message in this email thread only. User's timezone: ${params.timezone}

IMPORTANT - Language requirement:
- You MUST respond in the SAME language as the email content
- If the email is in English, respond in English
- If the email is in Traditional Chinese (繁體中文), respond in Traditional Chinese
- If the email is in Simplified Chinese (简体中文), respond in Simplified Chinese
- If the email is in Japanese, respond in Japanese
- If the email is in any other language, respond in that same language
- Pay special attention to distinguish between Traditional Chinese (used in Taiwan, Hong Kong, Macau) and Simplified Chinese (used in Mainland China, Singapore)

IMPORTANT: This email may contain quoted replies or forwarded content from previous messages.
- Focus ONLY on the new content at the TOP of the email
- IGNORE any quoted text (lines starting with ">", "On ... wrote:", blockquotes, "Original Message", "Forwarded message", etc.)
- IGNORE older messages in the thread - summarize only what the sender is saying NOW

From: ${params.from}
Subject: ${params.subject}

${params.bodyText}

Format (skip sections if not applicable):

Summary:
    <1-2 bullet points: what the sender is saying in their NEW message>

Action Items:
    - <what needs to be done, by whom, by when (convert to user's timezone)>

People/Contacts:
    - <names and roles mentioned in the new content>

Links/References:
    - <URLs, documents, or attachments mentioned in the new content>

No preamble. Be concise. Remember: summarize ONLY the newest message, not the entire thread.`;
}

export const SUMMARIZE_MAX_TOKENS = 5000;
