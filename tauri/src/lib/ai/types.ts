/**
 * AI service types
 */

export interface CompletionRequest {
  prompt: string;
  systemPrompt?: string;
  maxTokens?: number;
  providerName?: string;
}

export interface CompletionResponse {
  success: boolean;
  content: string | null;
  error: string | null;
  modelUsed: string | null;
}

export interface AIProviderConfig {
  type: "cli" | "api";
  name: string;
  model: string;
  baseUrl: string;
  apiKey: string;
  sdk?: "openai" | "anthropic" | "openrouter";
  customHeaders?: Record<string, string>;
}

export interface EmailSummary {
  emailUid: number;
  account: string;
  mailbox: string;
  summary: string;
  modelUsed: string;
  createdAt: number;
}

export interface ExtractedEvent {
  title: string;
  startTime: string;
  endTime: string;
  location?: string;
  notes?: string;
  alarmMinutesBefore?: number;
  alarmSpecified?: boolean;
}

export interface ExtractedReminder {
  title: string;
  notes?: string;
  dueDate?: string;
  priority?: number;
}
