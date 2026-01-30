import { create } from "zustand";
import { invoke } from "@tauri-apps/api/core";
import type { SanitizedAccount, FullAccount } from "@/types/account";

interface AccountsState {
  // Sanitized accounts (for Home, AccountRail - no credentials)
  sanitizedAccounts: SanitizedAccount[];
  // Full accounts (for Settings - includes credentials)
  fullAccounts: FullAccount[];
  avatarUrls: Record<string, string>;
  setSanitizedAccounts: (accounts: SanitizedAccount[]) => void;
  setFullAccounts: (accounts: FullAccount[]) => void;
  setAvatarUrl: (email: string, url: string) => void;
  removeAvatarUrl: (email: string) => void;
  refreshAccounts: () => Promise<void>;
  loadAvatarUrls: () => Promise<void>;
}

export const useAccountsStore = create<AccountsState>((set, get) => ({
  sanitizedAccounts: [],
  fullAccounts: [],
  avatarUrls: {},

  setSanitizedAccounts: (sanitizedAccounts) => set({ sanitizedAccounts }),

  setFullAccounts: (fullAccounts) => set({ fullAccounts }),

  setAvatarUrl: (email, url) =>
    set((state) => ({
      avatarUrls: { ...state.avatarUrls, [email]: url },
    })),

  removeAvatarUrl: (email) =>
    set((state) => {
      const { [email]: _, ...rest } = state.avatarUrls;
      return { avatarUrls: rest };
    }),

  refreshAccounts: async () => {
    try {
      const accounts = await invoke<FullAccount[]>("list_full_accounts");
      const sanitizedAccounts: SanitizedAccount[] = accounts.map((a) => ({
        name: a.name,
        provider: a.provider,
        email: a.credentials.email,
        avatar: a.avatar,
      }));
      set({ sanitizedAccounts });
      // Also refresh avatar URLs
      get().loadAvatarUrls();
    } catch (err) {
      console.error("Failed to refresh accounts:", err);
    }
  },

  loadAvatarUrls: async () => {
    try {
      const avatarUrls = await invoke<Record<string, string>>(
        "get_account_avatar_urls",
      );
      set({ avatarUrls });
    } catch {
      // Ignore errors
    }
  },
}));
