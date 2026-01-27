/**
 * Anthropic SDK provider wrapper
 */

import Anthropic from "@anthropic-ai/sdk";
import { fetch } from "@tauri-apps/plugin-http";
import type { AIProvider } from "./types";
import type { CompletionRequest, CompletionResponse, AIProviderConfig } from "../types";

class AnthropicProvider implements AIProvider {
  private getClient(config: AIProviderConfig): Anthropic {
    return new Anthropic({
      apiKey: config.apiKey,
      baseURL: config.baseUrl || undefined,
      dangerouslyAllowBrowser: true, // Safe in Tauri desktop app
      // Use Tauri's HTTP plugin fetch to bypass CORS
      fetch: fetch as unknown as typeof globalThis.fetch,
    });
  }

  async complete(
    request: CompletionRequest,
    config: AIProviderConfig
  ): Promise<CompletionResponse> {
    try {
      const client = this.getClient(config);

      const response = await client.messages.create({
        model: config.model,
        max_tokens: request.maxTokens ?? 4096,
        system: request.systemPrompt,
        messages: [{ role: "user", content: request.prompt }],
      });

      // Extract text from response content blocks
      const textBlock = response.content.find((block) => block.type === "text");

      if (!textBlock || textBlock.type !== "text") {
        return {
          success: false,
          content: null,
          error: "No text content in response",
          modelUsed: null,
        };
      }

      return {
        success: true,
        content: textBlock.text,
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

export const anthropicProvider = new AnthropicProvider();
