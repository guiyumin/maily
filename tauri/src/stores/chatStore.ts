import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  modelUsed?: string;
}

export interface ChatSession {
  id: string;
  title: string;
  messages: ChatMessage[];
  createdAt: number;
  updatedAt: number;
  context?: {
    type: "email" | "general";
    emailUid?: number;
    emailSubject?: string;
    account?: string;
    mailbox?: string;
  };
}

interface ChatStore {
  sessions: ChatSession[];
  activeSessionId: string | null;

  // Session management
  createSession: (context?: ChatSession["context"]) => string;
  deleteSession: (sessionId: string) => void;
  setActiveSession: (sessionId: string | null) => void;
  getActiveSession: () => ChatSession | null;

  // Message management
  addMessage: (sessionId: string, message: Omit<ChatMessage, "id" | "timestamp">) => void;
  updateLastMessage: (sessionId: string, content: string) => void;
  clearSession: (sessionId: string) => void;
}

function generateId(): string {
  return Math.random().toString(36).substring(2, 15);
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set, get) => ({
      sessions: [],
      activeSessionId: null,

      createSession: (context) => {
        const id = generateId();
        const now = Date.now();

        const title = context?.type === "email" && context.emailSubject
          ? `Chat: ${context.emailSubject.substring(0, 30)}...`
          : "New Chat";

        const session: ChatSession = {
          id,
          title,
          messages: [],
          createdAt: now,
          updatedAt: now,
          context,
        };

        set((state) => ({
          sessions: [session, ...state.sessions],
          activeSessionId: id,
        }));

        return id;
      },

      deleteSession: (sessionId) => {
        set((state) => ({
          sessions: state.sessions.filter((s) => s.id !== sessionId),
          activeSessionId: state.activeSessionId === sessionId ? null : state.activeSessionId,
        }));
      },

      setActiveSession: (sessionId) => {
        set({ activeSessionId: sessionId });
      },

      getActiveSession: () => {
        const state = get();
        if (!state.activeSessionId) return null;
        return state.sessions.find((s) => s.id === state.activeSessionId) || null;
      },

      addMessage: (sessionId, message) => {
        const id = generateId();
        const timestamp = Date.now();

        set((state) => ({
          sessions: state.sessions.map((session) =>
            session.id === sessionId
              ? {
                  ...session,
                  messages: [...session.messages, { ...message, id, timestamp }],
                  updatedAt: timestamp,
                }
              : session
          ),
        }));
      },

      updateLastMessage: (sessionId, content) => {
        set((state) => ({
          sessions: state.sessions.map((session) => {
            if (session.id !== sessionId) return session;
            const messages = [...session.messages];
            if (messages.length > 0) {
              messages[messages.length - 1] = {
                ...messages[messages.length - 1],
                content,
              };
            }
            return { ...session, messages, updatedAt: Date.now() };
          }),
        }));
      },

      clearSession: (sessionId) => {
        set((state) => ({
          sessions: state.sessions.map((session) =>
            session.id === sessionId
              ? { ...session, messages: [], updatedAt: Date.now() }
              : session
          ),
        }));
      },
    }),
    {
      name: "maily-chat-storage",
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        sessions: state.sessions.slice(0, 20), // Keep only last 20 sessions
        activeSessionId: state.activeSessionId,
      }),
    }
  )
);
