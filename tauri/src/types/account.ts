export interface Credentials {
  email: string;
  password: string;
  imap_host: string;
  imap_port: number;
  smtp_host: string;
  smtp_port: number;
}

export interface FullAccount {
  name: string;
  provider: string;
  credentials: Credentials;
  avatar?: string;
  display_name?: string;
}

// Account without sensitive credentials (for display)
// SanitizedAccount = { name, provider, avatar, email, display_name }
export type SanitizedAccount = Omit<FullAccount, "credentials"> &
  Pick<Credentials, "email">;
