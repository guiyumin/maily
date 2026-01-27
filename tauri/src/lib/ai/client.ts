/**
 * Unified AI client
 * Routes requests to the appropriate provider (JS SDK for API, Rust for CLI)
 */

import { invoke } from "@tauri-apps/api/core";
import { openaiProvider, anthropicProvider, openrouterProvider } from "./providers";
import type { CompletionRequest, CompletionResponse, AIProviderConfig } from "./types";

// Rust backend types (snake_case)
interface RustCompletionRequest {
  prompt: string;
  system_prompt?: string;
  max_tokens?: number;
  provider_name?: string;
}

interface RustCompletionResponse {
  success: boolean;
  content: string | null;
  error: string | null;
  model_used: string | null;
}

/**
 * Convert frontend types to Rust backend types
 */
function toRustRequest(request: CompletionRequest): RustCompletionRequest {
  return {
    prompt: request.prompt,
    system_prompt: request.systemPrompt,
    max_tokens: request.maxTokens,
    provider_name: request.providerName,
  };
}

/**
 * Convert Rust backend response to frontend types
 */
function fromRustResponse(response: RustCompletionResponse): CompletionResponse {
  return {
    success: response.success,
    content: response.content,
    error: response.error,
    modelUsed: response.model_used,
  };
}

/**
 * Call a CLI provider via Rust backend
 */
async function callCliProvider(
  request: CompletionRequest,
  config: AIProviderConfig
): Promise<CompletionResponse> {
  const rustResponse = await invoke<RustCompletionResponse>("cli_complete", {
    request: toRustRequest(request),
    providerName: config.name,
    providerModel: config.model,
  });
  return fromRustResponse(rustResponse);
}

/**
 * Call an API provider via JS SDK
 */
async function callApiProvider(
  request: CompletionRequest,
  config: AIProviderConfig
): Promise<CompletionResponse> {
  const sdk = config.sdk || "openai";

  switch (sdk) {
    case "anthropic":
      return anthropicProvider.complete(request, config);
    case "openrouter":
      return openrouterProvider.complete(request, config);
    case "openai":
    default:
      return openaiProvider.complete(request, config);
  }
}

/**
 * Send a completion request to an AI provider
 */
export async function complete(
  request: CompletionRequest,
  config: AIProviderConfig
): Promise<CompletionResponse> {
  if (config.type === "cli") {
    return callCliProvider(request, config);
  }
  return callApiProvider(request, config);
}

/**
 * Test an AI provider
 */
export async function testProvider(
  config: AIProviderConfig
): Promise<CompletionResponse> {
  const testRequest: CompletionRequest = {
    prompt: "Say hello.",
    maxTokens: 200,
  };

  if (config.type === "cli") {
    return callCliProvider(testRequest, config);
  }

  const sdk = config.sdk || "openai";
  switch (sdk) {
    case "anthropic":
      return anthropicProvider.test(config);
    case "openrouter":
      return openrouterProvider.test(config);
    case "openai":
    default:
      return openaiProvider.test(config);
  }
}

/**
 * Sort providers: API first, then CLI
 */
function sortProvidersByPriority(providers: AIProviderConfig[]): AIProviderConfig[] {
  const apiProviders = providers.filter((p) => p.type !== "cli");
  const cliProviders = providers.filter((p) => p.type === "cli");
  return [...apiProviders, ...cliProviders];
}

/**
 * Try providers in order until one succeeds
 * Priority: API providers first, then CLI providers
 */
export async function completeWithFallback(
  request: CompletionRequest,
  providers: AIProviderConfig[]
): Promise<CompletionResponse> {
  const triedProviders: string[] = [];
  let lastError: string | null = null;

  // Sort providers: API first, then CLI
  const sortedProviders = sortProvidersByPriority(providers);

  // If a specific provider is requested, try it first
  if (request.providerName) {
    const specificProvider = sortedProviders.find((p) => p.name === request.providerName);
    if (specificProvider) {
      const result = await complete(request, specificProvider);
      if (result.success) {
        return result;
      }
      triedProviders.push(`${specificProvider.name}/${specificProvider.model}`);
      lastError = result.error;
    }
  }

  // Try all providers in order (API first, then CLI)
  for (const provider of sortedProviders) {
    const providerId = `${provider.name}/${provider.model}`;
    if (triedProviders.includes(providerId)) {
      continue;
    }

    const result = await complete(request, provider);
    if (result.success) {
      return result;
    }
    triedProviders.push(providerId);
    lastError = result.error;
  }

  // All providers failed
  if (triedProviders.length === 0) {
    return {
      success: false,
      content: null,
      error: "No AI provider available. Please configure one in Settings.",
      modelUsed: null,
    };
  }

  return {
    success: false,
    content: null,
    error: lastError || `All AI providers failed. Tried: ${triedProviders.join(", ")}`,
    modelUsed: null,
  };
}

/**
 * Get list of available AI providers (configured + detected CLI tools)
 */
export async function getAvailableProviders(): Promise<string[]> {
  return invoke<string[]>("get_available_ai_providers");
}
