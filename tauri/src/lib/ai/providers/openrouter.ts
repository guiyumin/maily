/**
 * OpenRouter SDK provider wrapper
 * Uses custom HTTPClient with Tauri's fetch to bypass CORS
 */

import { OpenRouter, HTTPClient } from "@openrouter/sdk";
import { fetch } from "@tauri-apps/plugin-http";
import type { AIProvider } from "./types";
import type { CompletionRequest, CompletionResponse, AIProviderConfig } from "../types";

// OpenRouter uses specific message types
type SystemMessage = { role: "system"; content: string; name?: string };
type UserMessage = { role: "user"; content: string; name?: string };
type ORMessage = SystemMessage | UserMessage;

class OpenRouterProvider implements AIProvider {
  private getClient(config: AIProviderConfig): OpenRouter {
    // Create HTTPClient with Tauri's fetch to bypass CORS
    const httpClient = new HTTPClient({
      fetcher: fetch as unknown as typeof globalThis.fetch,
    });

    return new OpenRouter({
      apiKey: config.apiKey,
      httpClient,
      httpReferer: "https://maily.app",
      xTitle: "Maily",
    });
  }

  async complete(
    request: CompletionRequest,
    config: AIProviderConfig
  ): Promise<CompletionResponse> {
    try {
      const client = this.getClient(config);

      const messages: ORMessage[] = [];

      if (request.systemPrompt) {
        messages.push({ role: "system", content: request.systemPrompt });
      }

      messages.push({ role: "user", content: request.prompt });

      const response = await client.chat.send({
        model: config.model,
        messages,
        maxTokens: request.maxTokens,
        stream: false,
      });

      // Extract content from response
      const choice = response.choices?.[0];
      const content = choice?.message?.content;

      if (!content || typeof content !== "string") {
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

export const openrouterProvider = new OpenRouterProvider();
