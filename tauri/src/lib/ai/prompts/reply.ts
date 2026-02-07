/**
 * Reply generation prompt
 */

export interface GenerateReplyParams {
  originalFrom: string;
  originalSubject: string;
  originalBody: string;
  senderName?: string;
  userInstruction?: string;
}

export function buildReplyPrompt(params: GenerateReplyParams): string {
  const bodyTruncated = params.originalBody.slice(0, 4000);

  const userInstructionBlock = params.userInstruction
    ? `\nUser's instruction for this reply:\n${params.userInstruction}\n`
    : "";

  return `Analyze this email thread and write a professional reply.

Replying to: ${params.originalFrom}
Subject: ${params.originalSubject}
${params.senderName ? `Sender (you): ${params.senderName}` : ""}

Email thread (most recent first):
${bodyTruncated}
${userInstructionBlock}
Instructions:
- Read the entire email thread to understand the full context${params.userInstruction ? "\n- Follow the user's instruction above as the primary guide for the reply" : ""}
- Write a reply that appropriately addresses the most recent message
- If it's a request/invitation, acknowledge it professionally
- If questions were asked, provide helpful answers or ask for clarification
- If it requires scheduling/confirmation, express interest and ask for details if needed
- Keep it concise and professional
- Do NOT include email headers like "Subject:" or "To:" - just write the reply body
- Do NOT include the quoted thread in your reply - just write the new message
- Start directly with a greeting${params.senderName ? `\n- Sign off as "${params.senderName}" — do NOT use placeholders like [Your Name]` : ""}

Write the reply:`;
}

export const REPLY_SYSTEM_PROMPT =
  "You are a professional email assistant. Write concise, friendly, and helpful email replies. Match the tone of the conversation.";

export const REPLY_MAX_TOKENS = 2000;
