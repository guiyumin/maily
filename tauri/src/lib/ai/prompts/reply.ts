/**
 * Reply generation prompt
 */

export interface GenerateReplyParams {
  originalFrom: string;
  originalSubject: string;
  originalBody: string;
}

export function buildReplyPrompt(params: GenerateReplyParams): string {
  const bodyTruncated = params.originalBody.slice(0, 4000);

  return `Analyze this email thread and write a professional reply.

Replying to: ${params.originalFrom}
Subject: ${params.originalSubject}

Email thread (most recent first):
${bodyTruncated}

Instructions:
- Read the entire email thread to understand the full context
- Write a reply that appropriately addresses the most recent message
- If it's a request/invitation, acknowledge it professionally
- If questions were asked, provide helpful answers or ask for clarification
- If it requires scheduling/confirmation, express interest and ask for details if needed
- Keep it concise and professional
- Do NOT include email headers like "Subject:" or "To:" - just write the reply body
- Do NOT include the quoted thread in your reply - just write the new message
- Start directly with a greeting

Write the reply:`;
}

export const REPLY_SYSTEM_PROMPT =
  "You are a professional email assistant. Write concise, friendly, and helpful email replies. Match the tone of the conversation.";

export const REPLY_MAX_TOKENS = 2000;
