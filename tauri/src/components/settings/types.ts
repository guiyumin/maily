export interface TestResult {
  success: boolean;
  content: string | null;
  error: string | null;
  model_used: string | null;
}

export interface AIProvider {
  type: "cli" | "api";
  name: string;
  model: string;
  base_url: string;
  api_key: string;
}

export interface Credentials {
  email: string;
  password: string;
  imap_host: string;
  imap_port: number;
  smtp_host: string;
  smtp_port: number;
}

export interface Account {
  name: string;
  provider: string;
  credentials: Credentials;
}

// Notification settings
export interface NativeNotificationConfig {
  enabled: boolean;
  new_email: boolean;
  calendar_reminder: boolean;
}

export interface TelegramConfig {
  enabled: boolean;
  bot_token: string;
  chat_id: string;
}

export interface NotificationConfig {
  native: NativeNotificationConfig;
  telegram?: TelegramConfig;
}

// Integration settings
export interface GitHubConfig {
  enabled: boolean;
  token: string;
  parse_emails: boolean;
}

export interface IntegrationsConfig {
  github?: GitHubConfig;
}

export interface Config {
  max_emails: number;
  default_label: string;
  theme: string;
  language: string;
  ai_providers: AIProvider[];
  notifications?: NotificationConfig;
  integrations?: IntegrationsConfig;
}
