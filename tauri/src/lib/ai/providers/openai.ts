/**
 * OpenAI SDK provider wrapper
 * Also works for OpenRouter and other OpenAI-compatible APIs
 */

import OpenAI from "openai";
import { fetch } from "@tauri-apps/plugin-http";
import type { AIProvider } from "./types";
import type { CompletionRequest, CompletionResponse, AIProviderConfig } from "../types";

class OpenAIProvider implements AIProvider {
  private getClient(config: AIProviderConfig): OpenAI {
    const baseURL = config.baseUrl || "https://api.openai.com/v1";

    // Handle OpenRouter - it uses OpenAI-compatible API
    const isOpenRouter = config.sdk === "openrouter" || baseURL.includes("openrouter.ai");

    return new OpenAI({
      apiKey: config.apiKey,
      baseURL: isOpenRouter ? "https://openrouter.ai/api/v1" : baseURL,
      dangerouslyAllowBrowser: true, // Safe in Tauri desktop app
      // Use Tauri's HTTP plugin fetch to bypass CORS
      fetch: fetch as unknown as typeof globalThis.fetch,
      defaultHeaders: {
        ...config.customHeaders,
        // OpenRouter requires these headers
        ...(isOpenRouter
          ? {
              "HTTP-Referer": "https://maily.app",
              "X-Title": "Maily",
            }
          : {}),
      },
    });
  }

  async complete(
    request: CompletionRequest,
    config: AIProviderConfig
  ): Promise<CompletionResponse> {
    try {
      const client = this.getClient(config);

      const messages: OpenAI.ChatCompletionMessageParam[] = [];

      if (request.systemPrompt) {
        messages.push({ role: "system", content: request.systemPrompt });
      }

      messages.push({ role: "user", content: request.prompt });

      const response = await client.chat.completions.create({
        model: config.model,
        messages,
        max_tokens: request.maxTokens,
      });

      const content = response.choices[0]?.message?.content;

      if (!content) {
        return {
          success: false,
          content: null,
          error: "No content in response",
          modelUsed: null,
        };
      }

      return {
        success: true,
        content,
        error: null,
        modelUsed: `${config.name}/${config.model}`,
      };
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error";
      return {
        success: false,
        content: null,
        error: errorMessage,
        modelUsed: null,
      };
    }
  }

  async test(config: AIProviderConfig): Promise<CompletionResponse> {
    return this.complete(
      {
        prompt: "Say hello.",
        maxTokens: 200,
      },
      config
    );
  }
}

export const openaiProvider = new OpenAIProvider();
