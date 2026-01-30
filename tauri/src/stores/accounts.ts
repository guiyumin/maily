import { create } from "zustand";
import { invoke } from "@tauri-apps/api/core";
import type { SanitizedAccount } from "@/types/account";

interface AccountsState {
  accounts: SanitizedAccount[];
  avatarUrls: Record<string, string>;
  setAccounts: (accounts: SanitizedAccount[]) => void;
  setAvatarUrl: (email: string, url: string) => void;
  removeAvatarUrl: (email: string) => void;
  refreshAccounts: () => Promise<void>;
  loadAvatarUrls: () => Promise<void>;
}

export const useAccountsStore = create<AccountsState>((set, get) => ({
  accounts: [],
  avatarUrls: {},

  setAccounts: (accounts) => set({ accounts }),

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
      const accounts = await invoke<SanitizedAccount[]>("list_accounts");
      set({ accounts });
      // Also refresh avatar URLs
      get().loadAvatarUrls();
    } catch (err) {
      console.error("Failed to refresh accounts:", err);
    }
  },

  loadAvatarUrls: async () => {
    const { accounts } = get();
    const urls: Record<string, string> = {};

    for (const account of accounts) {
      if (account.avatar) {
        try {
          const dataUrl = await invoke<string | null>(
            "get_account_avatar_data_url",
            { accountName: account.name },
          );
          if (dataUrl) {
            urls[account.email] = dataUrl;
          }
        } catch {
          // Ignore errors
        }
      }
    }

    set({ avatarUrls: urls });
  },
}));
