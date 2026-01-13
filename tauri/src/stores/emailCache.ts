import { create } from "zustand";

const MAX_CACHE_SIZE = 500;

interface EmailFull {
  uid: number;
  message_id: string;
  internal_date: string;
  from: string;
  reply_to: string;
  to: string;
  cc: string;
  subject: string;
  date: string;
  snippet: string;
  body_html: string;
  unread: boolean;
  attachments: {
    part_id: string;
    filename: string;
    content_type: string;
    size: number;
    encoding: string;
  }[];
}

interface EmailCacheState {
  // LRU cache using Map (maintains insertion order)
  // Key: `${account}:${mailbox}:${uid}`
  emails: Map<string, EmailFull>;
  get: (account: string, mailbox: string, uid: number) => EmailFull | undefined;
  set: (account: string, mailbox: string, email: EmailFull) => void;
  updateReadStatus: (account: string, mailbox: string, uid: number, unread: boolean) => void;
  invalidate: (account: string, mailbox: string, uid: number) => void;
  clear: () => void;
}

const makeKey = (account: string, mailbox: string, uid: number) =>
  `${account}:${mailbox}:${uid}`;

export const useEmailCache = create<EmailCacheState>((set, get) => ({
  emails: new Map(),

  get: (account, mailbox, uid) => {
    const key = makeKey(account, mailbox, uid);
    const state = get();
    const email = state.emails.get(key);

    if (email) {
      // Move to end (most recently used) by re-inserting
      set((state) => {
        const newEmails = new Map(state.emails);
        newEmails.delete(key);
        newEmails.set(key, email);
        return { emails: newEmails };
      });
    }

    return email;
  },

  set: (account, mailbox, email) => {
    set((state) => {
      const key = makeKey(account, mailbox, email.uid);
      const newEmails = new Map(state.emails);

      // Remove if exists (to update position)
      newEmails.delete(key);

      // Evict oldest entries if at capacity
      while (newEmails.size >= MAX_CACHE_SIZE) {
        const oldestKey = newEmails.keys().next().value;
        if (oldestKey) {
          newEmails.delete(oldestKey);
        } else {
          break;
        }
      }

      // Add new entry at end (most recent)
      newEmails.set(key, email);
      return { emails: newEmails };
    });
  },

  updateReadStatus: (account, mailbox, uid, unread) => {
    set((state) => {
      const key = makeKey(account, mailbox, uid);
      const email = state.emails.get(key);
      if (email) {
        const newEmails = new Map(state.emails);
        // Update and move to end (most recently used)
        newEmails.delete(key);
        newEmails.set(key, { ...email, unread });
        return { emails: newEmails };
      }
      return state;
    });
  },

  invalidate: (account, mailbox, uid) => {
    set((state) => {
      const newEmails = new Map(state.emails);
      newEmails.delete(makeKey(account, mailbox, uid));
      return { emails: newEmails };
    });
  },

  clear: () => {
    set({ emails: new Map() });
  },
}));
