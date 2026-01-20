import { Inbox, Send, FileText, Trash2, Mail, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";

interface MailboxNavProps {
  selectedMailbox: string;
  onSelectMailbox: (mailbox: string) => void;
  unreadCount: number;
  provider?: string; // "gmail", "yahoo", or "imap"
  selectedAccount?: string;
}

// Get provider-specific mailbox names
function getMailboxes(provider?: string) {
  const isGmail = provider === "gmail";
  const isYahoo = provider === "yahoo";

  return [
    { id: "INBOX", label: "Inbox", icon: Inbox },
    {
      id: isGmail ? "[Gmail]/Sent Mail" : isYahoo ? "Sent" : "Sent",
      label: "Sent",
      icon: Send
    },
    {
      id: isGmail ? "[Gmail]/Drafts" : isYahoo ? "Draft" : "Drafts",
      label: "Drafts",
      icon: FileText
    },
    {
      id: isGmail ? "[Gmail]/Spam" : isYahoo ? "Bulk Mail" : "Spam",
      label: "Spam",
      icon: AlertCircle
    },
    {
      id: isGmail ? "[Gmail]/Trash" : isYahoo ? "Trash" : "Trash",
      label: "Trash",
      icon: Trash2
    },
  ];
}

export function MailboxNav({
  selectedMailbox,
  onSelectMailbox,
  unreadCount,
  provider,
  selectedAccount,
}: MailboxNavProps) {
  const mailboxes = getMailboxes(provider);

  return (
    <nav className="flex w-50 shrink-0 flex-col border-r bg-background">
      <div className="flex h-14 items-center gap-2 px-4 overflow-hidden">
        <Mail className="h-5 w-5 shrink-0" />
        <span className="text-sm font-medium truncate" title={selectedAccount}>
          {selectedAccount || "Mail"}
        </span>
      </div>

      <Separator />

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 p-2">
          {mailboxes.map((mailbox) => {
            const Icon = mailbox.icon;
            const isSelected = selectedMailbox === mailbox.id;
            const showBadge = mailbox.id === "INBOX" && unreadCount > 0;

            return (
              <Button
                key={mailbox.id}
                variant={isSelected ? "secondary" : "ghost"}
                className="w-full justify-start"
                onClick={() => onSelectMailbox(mailbox.id)}
              >
                <Icon className="mr-2 h-4 w-4" />
                {mailbox.label}
                {showBadge && (
                  <span className="ml-auto text-xs font-medium">
                    {unreadCount}
                  </span>
                )}
              </Button>
            );
          })}
        </div>
      </ScrollArea>
    </nav>
  );
}
