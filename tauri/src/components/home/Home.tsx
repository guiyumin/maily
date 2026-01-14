import { useEffect, useState, useCallback, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AccountRail } from "./AccountRail";
import { MailboxNav } from "./MailboxNav";
import { EmailList, type Email } from "./EmailList";
import { EmailReader } from "./EmailReader";
import { AIChat } from "@/components/chat/AIChat";
import { MessageSquare, Calendar } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Link } from "@tanstack/react-router";

interface Account {
  name: string;
  provider: string;
}

type UnreadCounts = Record<string, number>;

interface ListEmailsResult {
  emails: Email[];
  total: number;
  offset: number;
  has_more: boolean;
}

interface InitialState {
  accounts: Account[];
  selected_account: string | null;
  emails: ListEmailsResult;
}

// Read initial state injected by Rust (synchronous - no IPC wait)
declare global {
  interface Window {
    __MAILY_INITIAL_STATE__?: InitialState;
  }
}

function getInitialState(): InitialState | null {
  const state = window.__MAILY_INITIAL_STATE__;
  // Clear it so we don't accidentally reuse stale data
  delete window.__MAILY_INITIAL_STATE__;
  return state || null;
}

interface SyncCompleteEvent {
  account: string;
  mailbox: string;
  new_emails: number;
  updated_emails: number;
  total_emails: number;
}

interface SyncErrorEvent {
  account: string;
  mailbox: string;
  error: string;
}

const INITIAL_LOAD = 50;
const BATCH_SIZE = 50;

// Get initial state BEFORE component renders (synchronous)
const PRELOADED_STATE = getInitialState();

export function Home() {
  // Initialize with preloaded data if available - INSTANT first render
  const [accounts, setAccounts] = useState<Account[]>(
    PRELOADED_STATE?.accounts ?? []
  );
  const [selectedAccount, setSelectedAccount] = useState<string>(
    PRELOADED_STATE?.selected_account ?? ""
  );
  const [selectedMailbox, setSelectedMailbox] = useState<string>("INBOX");
  const [emails, setEmails] = useState<Email[]>(
    PRELOADED_STATE?.emails.emails ?? []
  );
  const [selectedEmail, setSelectedEmail] = useState<Email | null>(null);
  const [loading, setLoading] = useState(!PRELOADED_STATE);
  const [refreshing, setRefreshing] = useState(false);
  const [unreadCounts, setUnreadCounts] = useState<UnreadCounts>({});
  const [accountOrder, setAccountOrder] = useState<string[]>([]);

  // Pagination state
  const [total, setTotal] = useState(PRELOADED_STATE?.emails.total ?? 0);
  const [hasMore, setHasMore] = useState(PRELOADED_STATE?.emails.has_more ?? false);
  const [loadingMore, setLoadingMore] = useState(false);
  const count14DaysRef = useRef(0);
  const backgroundLoadingRef = useRef(false);
  const initialLoadDoneRef = useRef(!!PRELOADED_STATE);

  // Fetch unread counts for all accounts
  const fetchUnreadCounts = useCallback(async () => {
    try {
      const counts = await invoke<[string, number][]>("get_all_unread_counts");
      const countsMap: UnreadCounts = {};
      for (const [account, count] of counts) {
        countsMap[account] = count;
      }
      setUnreadCounts(countsMap);
    } catch (err) {
      console.error("Failed to fetch unread counts:", err);
    }
  }, []);

  // Fetch unread counts on mount and periodically
  useEffect(() => {
    fetchUnreadCounts();
    const interval = setInterval(fetchUnreadCounts, 30000); // Every 30 seconds
    return () => clearInterval(interval);
  }, [fetchUnreadCounts]);

  // Fetch account order from config
  useEffect(() => {
    invoke<{ account_order?: string[] }>("get_config")
      .then((config) => {
        if (config.account_order && config.account_order.length > 0) {
          setAccountOrder(config.account_order);
        }
      })
      .catch(console.error);
  }, []);

  // Save account order when changed
  const handleAccountOrderChange = useCallback((newOrder: string[]) => {
    setAccountOrder(newOrder);
    invoke("save_account_order", { order: newOrder }).catch(console.error);
  }, []);

  // Fallback: load via IPC if preloaded state wasn't available
  useEffect(() => {
    if (PRELOADED_STATE) return; // Already have data

    invoke<InitialState>("get_startup_state")
      .then((state) => {
        setAccounts(state.accounts);
        if (state.selected_account) {
          setSelectedAccount(state.selected_account);
          setEmails(state.emails.emails);
          setTotal(state.emails.total);
          setHasMore(state.emails.has_more);
          initialLoadDoneRef.current = true;
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // Load emails when account or mailbox changes (skip initial INBOX - already loaded)
  useEffect(() => {
    if (!selectedAccount) return;

    // Skip if this is the initial INBOX load (already loaded by get_startup_state)
    const isInitialInbox = initialLoadDoneRef.current && selectedMailbox === "INBOX";
    if (isInitialInbox) {
      // Mark initial load as consumed, so subsequent INBOX selections will reload
      initialLoadDoneRef.current = false;

      // Still start background loading for initial emails
      (async () => {
        const count = await invoke<number>("get_email_count_days", {
          account: selectedAccount,
          mailbox: selectedMailbox,
          days: 14,
        });
        count14DaysRef.current = count;

        if (emails.length < count && !backgroundLoadingRef.current) {
          backgroundLoadingRef.current = true;
          backgroundLoadEmails(selectedAccount, selectedMailbox, emails.length, count);
        }
      })();
      return;
    }

    setLoading(true);
    setSelectedEmail(null);
    setEmails([]);
    setTotal(0);
    setHasMore(false);
    backgroundLoadingRef.current = false;

    // Reset refreshing state when switching accounts/mailboxes
    setRefreshing(false);
    syncingRef.current = null;

    // 1. Load first 50 emails immediately
    invoke<ListEmailsResult>("list_emails_page", {
      account: selectedAccount,
      mailbox: selectedMailbox,
      offset: 0,
      limit: INITIAL_LOAD,
    })
      .then(async (result) => {
        setEmails(result.emails);
        setTotal(result.total);
        setHasMore(result.has_more);
        setLoading(false);

        // 2. Get 14-day count and start background loading
        const count = await invoke<number>("get_email_count_days", {
          account: selectedAccount,
          mailbox: selectedMailbox,
          days: 14,
        });
        count14DaysRef.current = count;

        // 3. Background load remaining 14-day emails
        if (result.emails.length < count && !backgroundLoadingRef.current) {
          backgroundLoadingRef.current = true;
          backgroundLoadEmails(selectedAccount, selectedMailbox, result.emails.length, count);
        }
      })
      .catch((err) => {
        console.error(err);
        setLoading(false);
      });
  }, [selectedAccount, selectedMailbox]);

  // Background load emails up to 14-day count
  const backgroundLoadEmails = async (
    account: string,
    mailbox: string,
    startOffset: number,
    targetCount: number
  ) => {
    let offset = startOffset;

    while (offset < targetCount && backgroundLoadingRef.current) {
      try {
        const result = await invoke<ListEmailsResult>("list_emails_page", {
          account,
          mailbox,
          offset,
          limit: BATCH_SIZE,
        });

        // Check if account/mailbox changed while loading
        if (!backgroundLoadingRef.current) break;

        setEmails((prev) => [...prev, ...result.emails]);
        setHasMore(result.has_more);
        offset += result.emails.length;

        // Small delay to avoid blocking
        await new Promise((r) => setTimeout(r, 10));
      } catch (err) {
        console.error("Background load error:", err);
        break;
      }
    }

    backgroundLoadingRef.current = false;
  };

  // Handle "Load more" button for emails older than 14 days
  const handleLoadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;

    setLoadingMore(true);
    try {
      const result = await invoke<ListEmailsResult>("list_emails_page", {
        account: selectedAccount,
        mailbox: selectedMailbox,
        offset: emails.length,
        limit: BATCH_SIZE,
      });

      setEmails((prev) => [...prev, ...result.emails]);
      setHasMore(result.has_more);
    } catch (err) {
      console.error("Load more error:", err);
    } finally {
      setLoadingMore(false);
    }
  }, [selectedAccount, selectedMailbox, emails.length, loadingMore, hasMore]);

  // Track which account/mailbox we're syncing
  const syncingRef = useRef<{ account: string; mailbox: string } | null>(null);

  // Listen for sync events
  useEffect(() => {
    const unlisteners: UnlistenFn[] = [];

    const setupListeners = async () => {
      // Sync started
      unlisteners.push(
        await listen("sync-started", () => {
          // Already showing spinner via refreshing state
        })
      );

      // Sync complete - reload emails from cache
      unlisteners.push(
        await listen<SyncCompleteEvent>("sync-complete", async (event) => {
          const { account, mailbox, new_emails, updated_emails } = event.payload;
          console.log(`[sync] Complete: ${new_emails} new, ${updated_emails} updated`);

          // Only reload if this is for the currently selected account/mailbox
          if (account === selectedAccount && mailbox === selectedMailbox) {
            // Reload from the beginning
            const result = await invoke<ListEmailsResult>("list_emails_page", {
              account,
              mailbox,
              offset: 0,
              limit: Math.max(emails.length, INITIAL_LOAD),
            });
            setEmails(result.emails);
            setTotal(result.total);
            setHasMore(result.has_more);
            setRefreshing(false);
            syncingRef.current = null;
          }
          // Refresh unread counts after any sync
          fetchUnreadCounts();
        })
      );

      // Sync error
      unlisteners.push(
        await listen<SyncErrorEvent>("sync-error", (event) => {
          const { account, mailbox, error } = event.payload;
          console.error(`[sync] Error for ${account}/${mailbox}: ${error}`);

          if (account === selectedAccount && mailbox === selectedMailbox) {
            setRefreshing(false);
            syncingRef.current = null;
          }
        })
      );
    };

    setupListeners();

    return () => {
      unlisteners.forEach((unlisten) => unlisten());
    };
  }, [selectedAccount, selectedMailbox, emails.length, fetchUnreadCounts]);

  const handleRefresh = useCallback(() => {
    if (!selectedAccount || refreshing) return;

    setRefreshing(true);
    syncingRef.current = { account: selectedAccount, mailbox: selectedMailbox };

    // Queue sync - returns immediately, events will notify when done
    invoke("start_sync", {
      account: selectedAccount,
      mailbox: selectedMailbox,
    }).catch((err) => {
      console.error("Failed to start sync:", err);
      setRefreshing(false);
      syncingRef.current = null;
    });
  }, [selectedAccount, selectedMailbox, refreshing]);

  const handleEmailDeleted = useCallback((uid: number) => {
    // Check if the deleted email was unread to update count
    const deletedEmail = emails.find((e) => e.uid === uid);
    if (deletedEmail?.unread && selectedMailbox === "INBOX") {
      setUnreadCounts((prev) => ({
        ...prev,
        [selectedAccount]: Math.max(0, (prev[selectedAccount] || 0) - 1),
      }));
    }
    setEmails((prev) => prev.filter((e) => e.uid !== uid));
    setSelectedEmail((prev) => (prev?.uid === uid ? null : prev));
    setTotal((prev) => Math.max(0, prev - 1));
  }, [emails, selectedAccount, selectedMailbox]);

  const handleEmailReadChange = useCallback((uid: number, unread: boolean) => {
    // Update unread count locally for INBOX
    if (selectedMailbox === "INBOX") {
      const email = emails.find((e) => e.uid === uid);
      if (email && email.unread !== unread) {
        setUnreadCounts((prev) => ({
          ...prev,
          [selectedAccount]: Math.max(0, (prev[selectedAccount] || 0) + (unread ? 1 : -1)),
        }));
      }
    }
    setEmails((prev) =>
      prev.map((e) => (e.uid === uid ? { ...e, unread } : e))
    );
    setSelectedEmail((prev) =>
      prev?.uid === uid ? { ...prev, unread } : prev
    );
  }, [emails, selectedAccount, selectedMailbox]);

  const handleNavigate = useCallback(
    (direction: "prev" | "next") => {
      if (!selectedEmail) return;

      const currentIndex = emails.findIndex((e) => e.uid === selectedEmail.uid);
      if (currentIndex === -1) return;

      const newIndex =
        direction === "prev" ? currentIndex - 1 : currentIndex + 1;
      if (newIndex >= 0 && newIndex < emails.length) {
        setSelectedEmail(emails[newIndex]);
      }
    },
    [selectedEmail, emails]
  );

  const selectedIndex = selectedEmail
    ? emails.findIndex((e) => e.uid === selectedEmail.uid)
    : -1;
  const canNavigatePrev = selectedIndex > 0;
  const canNavigateNext =
    selectedIndex !== -1 && selectedIndex < emails.length - 1;

  const unreadCount = emails.filter((e) => e.unread).length;

  // Handle keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't handle if user is typing in an input
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      switch (e.key) {
        case "j":
        case "ArrowDown":
          e.preventDefault();
          if (!selectedEmail && emails.length > 0) {
            setSelectedEmail(emails[0]);
          } else {
            handleNavigate("next");
          }
          break;
        case "k":
        case "ArrowUp":
          e.preventDefault();
          handleNavigate("prev");
          break;
        case "Escape":
          setSelectedEmail(null);
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedEmail, emails, handleNavigate]);

  return (
    <TooltipProvider>
      <div className="flex h-screen w-full overflow-hidden bg-background">
        <AccountRail
          accounts={accounts}
          selectedAccount={selectedAccount}
          onSelectAccount={setSelectedAccount}
          unreadCounts={unreadCounts}
          accountOrder={accountOrder}
          onOrderChange={handleAccountOrderChange}
        />

        <MailboxNav
          selectedMailbox={selectedMailbox}
          onSelectMailbox={setSelectedMailbox}
          unreadCount={unreadCount}
          provider={accounts.find(a => a.name === selectedAccount)?.provider}
        />

        <EmailList
          emails={emails}
          selectedEmail={selectedEmail}
          onSelectEmail={setSelectedEmail}
          loading={loading}
          refreshing={refreshing}
          onRefresh={handleRefresh}
          mailboxName={selectedMailbox === "INBOX" ? "Inbox" : selectedMailbox}
          total={total}
          hasMore={hasMore}
          loadingMore={loadingMore}
          onLoadMore={handleLoadMore}
        />

        <EmailReader
          email={selectedEmail}
          account={selectedAccount}
          mailbox={selectedMailbox}
          onEmailDeleted={handleEmailDeleted}
          onEmailReadChange={handleEmailReadChange}
          onNavigate={handleNavigate}
          canNavigatePrev={canNavigatePrev}
          canNavigateNext={canNavigateNext}
        />

        {/* Floating action buttons */}
        <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-3">
          {/* Calendar button */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                size="lg"
                variant="outline"
                className="h-14 w-14 rounded-full shadow-lg bg-background"
                asChild
              >
                <Link to="/calendar">
                  <Calendar className="h-6 w-6" />
                </Link>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left">Calendar</TooltipContent>
          </Tooltip>

          {/* AI Chat button */}
          <AIChat
            context={
              selectedEmail
                ? {
                    type: "email",
                    emailUid: selectedEmail.uid,
                    emailSubject: selectedEmail.subject,
                    account: selectedAccount,
                    mailbox: selectedMailbox,
                  }
                : { type: "general" }
            }
            trigger={
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    size="lg"
                    className="h-14 w-14 rounded-full shadow-lg"
                  >
                    <MessageSquare className="h-6 w-6" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="left">AI Chat</TooltipContent>
              </Tooltip>
            }
          />
        </div>
      </div>
    </TooltipProvider>
  );
}
