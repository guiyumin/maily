import { useEffect, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AccountRail } from "./AccountRail";
import { MailboxNav } from "./MailboxNav";
import { EmailList, type Email } from "./EmailList";
import { EmailReader } from "./EmailReader";

interface Account {
  name: string;
  provider: string;
}

export function Home() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccount, setSelectedAccount] = useState<string>("");
  const [selectedMailbox, setSelectedMailbox] = useState<string>("INBOX");
  const [emails, setEmails] = useState<Email[]>([]);
  const [selectedEmail, setSelectedEmail] = useState<Email | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

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

    invoke<Email[]>("list_emails", {
      account: selectedAccount,
      mailbox: selectedMailbox,
    })
      .then(setEmails)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [selectedAccount, selectedMailbox]);

  const handleRefresh = useCallback(async () => {
    if (!selectedAccount || refreshing) return;

    setRefreshing(true);
    try {
      // Sync with IMAP server first
      await invoke("sync_emails", {
        account: selectedAccount,
        mailbox: selectedMailbox,
      });

      // Then reload from cache
      const newEmails = await invoke<Email[]>("list_emails", {
        account: selectedAccount,
        mailbox: selectedMailbox,
      });
      setEmails(newEmails);
    } catch (err) {
      console.error(err);
    } finally {
      setRefreshing(false);
    }
  }, [selectedAccount, selectedMailbox, refreshing]);

  const handleEmailDeleted = useCallback((uid: number) => {
    setEmails((prev) => prev.filter((e) => e.uid !== uid));
    setSelectedEmail((prev) => (prev?.uid === uid ? null : prev));
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
