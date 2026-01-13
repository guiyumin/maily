import { create } from "zustand";

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
  // Cache key: `${account}:${mailbox}:${uid}`
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
    return get().emails.get(makeKey(account, mailbox, uid));
  },

  set: (account, mailbox, email) => {
    set((state) => {
      const newEmails = new Map(state.emails);
      newEmails.set(makeKey(account, mailbox, email.uid), email);
      return { emails: newEmails };
    });
  },

  updateReadStatus: (account, mailbox, uid, unread) => {
    set((state) => {
      const key = makeKey(account, mailbox, uid);
      const email = state.emails.get(key);
      if (email) {
        const newEmails = new Map(state.emails);
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
