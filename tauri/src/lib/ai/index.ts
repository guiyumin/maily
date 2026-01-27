/**
 * AI service module
 *
 * Usage:
 * ```ts
 * import { complete, completeWithFallback, testProvider } from "@/lib/ai";
 * import { buildSummarizePrompt, buildReplyPrompt } from "@/lib/ai/prompts";
 * ```
 */

export * from "./types";
export * from "./client";
export * from "./providers";
export * from "./prompts";
export * from "./hooks";
