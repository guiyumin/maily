import { useEffect, useState, useCallback, useRef } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AccountRail } from "./AccountRail";
import { MailboxNav } from "./MailboxNav";
import { EmailList, type Email } from "./EmailList";
import { EmailReader } from "./EmailReader";

interface Account {
  name: string;
  provider: string;
}

interface ListEmailsResult {
  emails: Email[];
  total: number;
  offset: number;
  has_more: boolean;
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

export function Home() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccount, setSelectedAccount] = useState<string>("");
  const [selectedMailbox, setSelectedMailbox] = useState<string>("INBOX");
  const [emails, setEmails] = useState<Email[]>([]);
  const [selectedEmail, setSelectedEmail] = useState<Email | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // Pagination state
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const count14DaysRef = useRef(0);
  const backgroundLoadingRef = useRef(false);

  // Load accounts on mount
  useEffect(() => {
    invoke<Account[]>("list_accounts")
      .then((accts) => {
        setAccounts(accts);
        if (accts.length > 0) {
          setSelectedAccount(accts[0].name);
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // Load emails when account or mailbox changes
  useEffect(() => {
    if (!selectedAccount) return;

    setLoading(true);
    setSelectedEmail(null);
    setEmails([]);
    setTotal(0);
    setHasMore(false);
    backgroundLoadingRef.current = false;

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
  }, [selectedAccount, selectedMailbox, emails.length]);

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
    setEmails((prev) => prev.filter((e) => e.uid !== uid));
    setSelectedEmail((prev) => (prev?.uid === uid ? null : prev));
    setTotal((prev) => Math.max(0, prev - 1));
  }, []);

  const handleEmailReadChange = useCallback((uid: number, unread: boolean) => {
    setEmails((prev) =>
      prev.map((e) => (e.uid === uid ? { ...e, unread } : e))
    );
    setSelectedEmail((prev) =>
      prev?.uid === uid ? { ...prev, unread } : prev
    );
  }, []);

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
        />

        <MailboxNav
          selectedMailbox={selectedMailbox}
          onSelectMailbox={setSelectedMailbox}
          unreadCount={unreadCount}
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
      </div>
    </TooltipProvider>
  );
}
