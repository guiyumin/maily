/**
 * React hooks for AI features
 */

import { useState, useCallback, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { complete, completeWithFallback, testProvider, getAvailableProviders } from "./client";
import {
  buildSummarizePrompt,
  SUMMARIZE_MAX_TOKENS,
  buildReplyPrompt,
  REPLY_SYSTEM_PROMPT,
  REPLY_MAX_TOKENS,
  buildExtractEventPrompt,
  EXTRACT_EVENT_SYSTEM_PROMPT,
  EXTRACT_EVENT_MAX_TOKENS,
  buildExtractReminderPrompt,
  EXTRACT_REMINDER_SYSTEM_PROMPT,
  EXTRACT_REMINDER_MAX_TOKENS,
  buildTagsPrompt,
  TAGS_SYSTEM_PROMPT,
  TAGS_MAX_TOKENS,
  parseTags,
  buildParseEventNlpPrompt,
  PARSE_EVENT_NLP_MAX_TOKENS,
} from "./prompts";
import type {
  CompletionRequest,
  CompletionResponse,
  AIProviderConfig,
  EmailSummary,
} from "./types";

// Re-export types used by the settings component
export type { AIProviderConfig };

// Rust backend types (snake_case)
interface RustAIProvider {
  type: "cli" | "api";
  name: string;
  model: string;
  base_url: string;
  api_key: string;
  sdk?: string;
  custom_headers?: Record<string, string>;
}

interface RustConfig {
  ai_providers: RustAIProvider[];
}

interface RustEmailSummary {
  email_uid: number;
  account: string;
  mailbox: string;
  summary: string;
  model_used: string;
  created_at: number;
}

/**
 * Convert Rust provider config to frontend format
 */
function fromRustProvider(provider: RustAIProvider): AIProviderConfig {
  return {
    type: provider.type,
    name: provider.name,
    model: provider.model,
    baseUrl: provider.base_url,
    apiKey: provider.api_key,
    sdk: provider.sdk as AIProviderConfig["sdk"],
    customHeaders: provider.custom_headers,
  };
}

/**
 * Hook to fetch and cache available AI providers
 */
export function useAIProviders() {
  const [providers, setProviders] = useState<string[]>([]);
  const [providerConfigs, setProviderConfigs] = useState<AIProviderConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [names, config] = await Promise.all([
        getAvailableProviders(),
        invoke<RustConfig>("get_config"),
      ]);
      setProviders(names);
      setProviderConfigs(config.ai_providers.map(fromRustProvider));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load providers");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { providers, providerConfigs, loading, error, refresh };
}

/**
 * Hook for generic AI completion
 */
export function useAIComplete() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const executeComplete = useCallback(
    async (
      request: CompletionRequest,
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);
      try {
        const response = await completeWithFallback(request, providers);
        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { executeComplete, loading, error };
}

/**
 * Hook for email summarization
 */
export function useSummarize() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const summarize = useCallback(
    async (
      params: {
        account: string;
        mailbox: string;
        uid: number;
        from: string;
        subject: string;
        bodyText: string;
        forceRefresh?: boolean;
      },
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        // Check cache first (unless force refresh)
        if (!params.forceRefresh) {
          const cached = await invoke<RustEmailSummary | null>("get_email_summary", {
            account: params.account,
            mailbox: params.mailbox,
            uid: params.uid,
          });
          if (cached) {
            return {
              success: true,
              content: cached.summary,
              error: null,
              modelUsed: cached.model_used,
            };
          }
        }

        // Generate new summary
        const prompt = buildSummarizePrompt({
          from: params.from,
          subject: params.subject,
          bodyText: params.bodyText,
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        });

        const response = await completeWithFallback(
          { prompt, maxTokens: SUMMARIZE_MAX_TOKENS },
          providers
        );

        // Cache successful response
        if (response.success && response.content) {
          await invoke("save_email_summary", {
            account: params.account,
            mailbox: params.mailbox,
            uid: params.uid,
            summary: response.content,
            modelUsed: response.modelUsed || "unknown",
          });
        }

        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { summarize, loading, error };
}

/**
 * Hook for reply generation
 */
export function useGenerateReply() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generateReply = useCallback(
    async (
      params: {
        originalFrom: string;
        originalSubject: string;
        originalBody: string;
      },
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        const prompt = buildReplyPrompt(params);
        const response = await completeWithFallback(
          {
            prompt,
            systemPrompt: REPLY_SYSTEM_PROMPT,
            maxTokens: REPLY_MAX_TOKENS,
          },
          providers
        );

        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { generateReply, loading, error };
}

/**
 * Hook for calendar event extraction
 */
export function useExtractEvent() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const extractEvent = useCallback(
    async (
      params: {
        from: string;
        subject: string;
        bodyText: string;
        userTimezone: string;
      },
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        const prompt = buildExtractEventPrompt(params);
        const response = await completeWithFallback(
          {
            prompt,
            systemPrompt: EXTRACT_EVENT_SYSTEM_PROMPT,
            maxTokens: EXTRACT_EVENT_MAX_TOKENS,
          },
          providers
        );

        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { extractEvent, loading, error };
}

/**
 * Hook for reminder extraction
 */
export function useExtractReminder() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const extractReminder = useCallback(
    async (
      params: {
        from: string;
        subject: string;
        bodyText: string;
      },
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        const prompt = buildExtractReminderPrompt(params);
        const response = await completeWithFallback(
          {
            prompt,
            systemPrompt: EXTRACT_REMINDER_SYSTEM_PROMPT,
            maxTokens: EXTRACT_REMINDER_MAX_TOKENS,
          },
          providers
        );

        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { extractReminder, loading, error };
}

/**
 * Hook for tag generation
 */
export function useGenerateTags() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generateTags = useCallback(
    async (
      params: {
        from: string;
        subject: string;
        bodyText: string;
      },
      providers: AIProviderConfig[]
    ): Promise<string[]> => {
      setLoading(true);
      setError(null);

      try {
        const prompt = buildTagsPrompt(params);
        const response = await completeWithFallback(
          {
            prompt,
            systemPrompt: TAGS_SYSTEM_PROMPT,
            maxTokens: TAGS_MAX_TOKENS,
          },
          providers
        );

        if (!response.success || !response.content) {
          setError(response.error || "No tags generated");
          return [];
        }

        return parseTags(response.content);
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return [];
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { generateTags, loading, error };
}

/**
 * Hook for NLP event parsing
 */
export function useParseEventNlp() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const parseEvent = useCallback(
    async (
      params: {
        userInput: string;
        emailFrom?: string;
        emailSubject?: string;
        emailBody?: string;
      },
      providers: AIProviderConfig[]
    ): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        const prompt = buildParseEventNlpPrompt(params);
        const response = await completeWithFallback(
          {
            prompt,
            maxTokens: PARSE_EVENT_NLP_MAX_TOKENS,
          },
          providers
        );

        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { parseEvent, loading, error };
}

/**
 * Hook for testing an AI provider
 */
export function useTestProvider() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const test = useCallback(
    async (config: AIProviderConfig): Promise<CompletionResponse> => {
      setLoading(true);
      setError(null);

      try {
        const response = await testProvider(config);
        if (!response.success) {
          setError(response.error);
        }
        return response;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Unknown error";
        setError(errorMessage);
        return {
          success: false,
          content: null,
          error: errorMessage,
          modelUsed: null,
        };
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return { test, loading, error };
}
