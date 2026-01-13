import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";

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

  if (loading && accounts.length === 0) {
    return (
      <div className="flex h-screen items-center justify-center">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-64 border-r bg-sidebar p-4">
        <h2 className="mb-4 text-lg font-semibold">Accounts</h2>
        <ul className="space-y-1">
          {accounts.map((account) => (
            <li key={account.name}>
              <button
                onClick={() => setSelectedAccount(account.name)}
                className={`w-full rounded-md px-3 py-2 text-left text-sm ${
                  selectedAccount === account.name
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "hover:bg-sidebar-accent/50"
                }`}
              >
                {account.name}
              </button>
            </li>
          ))}
        </ul>
      </aside>

      {/* Email list */}
      <main className="flex-1 overflow-auto">
        <header className="sticky top-0 border-b bg-background p-4">
          <h1 className="text-xl font-semibold">Inbox</h1>
          <p className="text-sm text-muted-foreground">
            {emails.length} emails
          </p>
        </header>

        {loading ? (
          <div className="flex h-64 items-center justify-center">
            <p className="text-muted-foreground">Loading emails...</p>
          </div>
        ) : (
          <ul className="divide-y">
            {emails.map((email) => (
              <li
                key={email.uid}
                className={`cursor-pointer p-4 hover:bg-accent/50 ${
                  email.unread ? "bg-accent/20" : ""
                }`}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <p
                      className={`truncate text-sm ${
                        email.unread ? "font-semibold" : ""
                      }`}
                    >
                      {email.from}
                    </p>
                    <p
                      className={`truncate ${
                        email.unread ? "font-medium" : ""
                      }`}
                    >
                      {email.subject}
                    </p>
                    <p className="truncate text-sm text-muted-foreground">
                      {email.snippet}
                    </p>
                  </div>
                  <time className="shrink-0 text-xs text-muted-foreground">
                    {new Date(email.date).toLocaleDateString()}
                  </time>
                </div>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}
