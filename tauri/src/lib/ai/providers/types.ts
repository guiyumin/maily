/**
 * AI provider interface
 */

import type { CompletionRequest, CompletionResponse, AIProviderConfig } from "../types";

export interface AIProvider {
  /**
   * Send a completion request to the provider
   */
  complete(request: CompletionRequest, config: AIProviderConfig): Promise<CompletionResponse>;

  /**
   * Test the provider connection
   */
  test(config: AIProviderConfig): Promise<CompletionResponse>;
}
