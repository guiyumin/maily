/**
 * Tag generation prompt
 */

export interface GenerateTagsParams {
  from: string;
  subject: string;
  bodyText: string;
}

export function buildTagsPrompt(params: GenerateTagsParams): string {
  const bodyTruncated = params.bodyText.slice(0, 2000);

  return `Analyze this email and suggest 1-5 short tags to categorize it.

From: ${params.from}
Subject: ${params.subject}

${bodyTruncated}

Return ONLY a comma-separated list of short tags (1-2 words each). Examples: work, urgent, newsletter, receipt, travel, meeting, personal, finance, shipping, social

Tags:`;
}

export const TAGS_SYSTEM_PROMPT =
  "You are a helpful assistant that categorizes emails with short, descriptive tags. Only output comma-separated tags, nothing else.";

export const TAGS_MAX_TOKENS = 100;

/**
 * Parse tags from AI response
 */
export function parseTags(response: string): string[] {
  return response
    .split(",")
    .map((s) => s.trim().toLowerCase())
    .filter((s) => s.length > 0 && s.length <= 30)
    .slice(0, 5);
}
