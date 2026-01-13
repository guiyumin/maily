import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Settings, Mail, Inbox, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

interface Account {
  name: string;
  provider: string;
}

interface Email {
  uid: number;
  from: string;
  subject: string;
  snippet: string;
  date: string;
  unread: boolean;
}

export const Route = createFileRoute("/")({
  component: Index,
});

function Index() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccount, setSelectedAccount] = useState<string>("");
  const [emails, setEmails] = useState<Email[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

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

  useEffect(() => {
    if (!selectedAccount) return;

    setLoading(true);
    invoke<Email[]>("list_emails", { account: selectedAccount, mailbox: "INBOX" })
      .then(setEmails)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [selectedAccount]);

  const handleRefresh = async () => {
    if (!selectedAccount || refreshing) return;
    setRefreshing(true);
    try {
      const newEmails = await invoke<Email[]>("list_emails", {
        account: selectedAccount,
        mailbox: "INBOX",
      });
      setEmails(newEmails);
    } catch (err) {
      console.error(err);
    } finally {
      setRefreshing(false);
    }
  };

  if (loading && accounts.length === 0) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-4">
          <Mail className="h-12 w-12 text-muted-foreground animate-pulse" />
          <p className="text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="flex w-64 flex-col border-r bg-sidebar">
        <div className="p-4">
          <h1 className="text-xl font-bold tracking-tight">Maily</h1>
        </div>
        <Separator />

        <ScrollArea className="flex-1">
          <div className="p-4 space-y-4">
            <div>
              <h2 className="mb-2 px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Accounts
              </h2>
              <div className="space-y-1">
                {accounts.map((account) => (
                  <Button
                    key={account.name}
                    variant={selectedAccount === account.name ? "secondary" : "ghost"}
                    className="w-full justify-start"
                    onClick={() => setSelectedAccount(account.name)}
                  >
                    <Mail className="mr-2 h-4 w-4" />
                    <span className="truncate">{account.name}</span>
                  </Button>
                ))}
                {accounts.length === 0 && (
                  <p className="px-2 text-sm text-muted-foreground">
                    No accounts configured
                  </p>
                )}
              </div>
            </div>

            <Separator />

            <div>
              <h2 className="mb-2 px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Mailboxes
              </h2>
              <div className="space-y-1">
                <Button variant="secondary" className="w-full justify-start">
                  <Inbox className="mr-2 h-4 w-4" />
                  Inbox
                  {emails.filter((e) => e.unread).length > 0 && (
                    <Badge variant="default" className="ml-auto">
                      {emails.filter((e) => e.unread).length}
                    </Badge>
                  )}
                </Button>
              </div>
            </div>
          </div>
        </ScrollArea>

        <Separator />
        <div className="p-4">
          <Button variant="ghost" className="w-full justify-start" asChild>
            <Link to="/settings">
              <Settings className="mr-2 h-4 w-4" />
              Settings
            </Link>
          </Button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <header className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <h2 className="text-lg font-semibold">Inbox</h2>
            <p className="text-sm text-muted-foreground">
              {emails.length} emails
              {emails.filter((e) => e.unread).length > 0 &&
                ` • ${emails.filter((e) => e.unread).length} unread`}
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={refreshing}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </header>

        {/* Email list */}
        <ScrollArea className="flex-1">
          {loading ? (
            <div className="p-4 space-y-4">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="space-y-2">
                  <Skeleton className="h-4 w-1/3" />
                  <Skeleton className="h-4 w-2/3" />
                  <Skeleton className="h-4 w-1/2" />
                </div>
              ))}
            </div>
          ) : emails.length === 0 ? (
            <div className="flex h-64 flex-col items-center justify-center gap-2">
              <Inbox className="h-12 w-12 text-muted-foreground" />
              <p className="text-muted-foreground">No emails</p>
            </div>
          ) : (
            <div className="divide-y">
              {emails.map((email) => (
                <Link
                  key={email.uid}
                  to="/email/$account/$mailbox/$uid"
                  params={{
                    account: selectedAccount,
                    mailbox: "INBOX",
                    uid: String(email.uid),
                  }}
                  className={`block px-6 py-4 transition-colors hover:bg-accent/50 ${
                    email.unread ? "bg-accent/20" : ""
                  }`}
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0 flex-1 space-y-1">
                      <div className="flex items-center gap-2">
                        <p
                          className={`truncate text-sm ${
                            email.unread ? "font-semibold" : "text-muted-foreground"
                          }`}
                        >
                          {email.from}
                        </p>
                        {email.unread && (
                          <span className="h-2 w-2 rounded-full bg-primary" />
                        )}
                      </div>
                      <p
                        className={`truncate ${
                          email.unread ? "font-medium" : ""
                        }`}
                      >
                        {email.subject || "(No subject)"}
                      </p>
                      <p className="truncate text-sm text-muted-foreground">
                        {email.snippet}
                      </p>
                    </div>
                    <time className="shrink-0 text-xs text-muted-foreground">
                      {formatDate(email.date)}
                    </time>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </ScrollArea>
      </main>
    </div>
  );
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days === 0) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  } else if (days === 1) {
    return "Yesterday";
  } else if (days < 7) {
    return date.toLocaleDateString([], { weekday: "short" });
  } else {
    return date.toLocaleDateString([], { month: "short", day: "numeric" });
  }
}
