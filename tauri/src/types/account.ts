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
}

// Account without sensitive credentials (for display)
// SanitizedAccount = { name, provider, avatar, email }
export type SanitizedAccount = Omit<FullAccount, "credentials"> &
  Pick<Credentials, "email">;
