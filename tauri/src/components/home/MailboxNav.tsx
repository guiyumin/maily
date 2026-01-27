import { Inbox, Send, FileText, Trash2, Mail, AlertCircle, MailOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { useLocale } from "@/lib/i18n";
import type { TranslationKey } from "@/lib/i18n";

interface MailboxNavProps {
  selectedMailbox: string;
  onSelectMailbox: (mailbox: string) => void;
  unreadCount: number;
  provider?: string; // "gmail", "yahoo", or "imap"
  selectedAccount?: string;
}

// Get provider-specific mailbox names
function getMailboxes(provider?: string, unreadCount?: number) {
  const isGmail = provider === "gmail";
  const isYahoo = provider === "yahoo";

  const mailboxes = [];

  // Only show unread folder when there are unread emails
  if (unreadCount && unreadCount > 0) {
    mailboxes.push({ id: "__UNREAD__", labelKey: "mail.unread" as TranslationKey, icon: MailOpen });
  }

  mailboxes.push(
    { id: "INBOX", labelKey: "mail.inbox" as TranslationKey, icon: Inbox },
    {
      id: isGmail ? "[Gmail]/Sent Mail" : isYahoo ? "Sent" : "Sent",
      labelKey: "mail.sent" as TranslationKey,
      icon: Send
    },
    {
      id: isGmail ? "[Gmail]/Drafts" : isYahoo ? "Draft" : "Drafts",
      labelKey: "mail.drafts" as TranslationKey,
      icon: FileText
    },
    {
      id: isGmail ? "[Gmail]/Spam" : isYahoo ? "Bulk Mail" : "Spam",
      labelKey: "mail.spam" as TranslationKey,
      icon: AlertCircle
    },
    {
      id: isGmail ? "[Gmail]/Trash" : isYahoo ? "Trash" : "Trash",
      labelKey: "mail.trash" as TranslationKey,
      icon: Trash2
    },
  );

  return mailboxes;
}

export function MailboxNav({
  selectedMailbox,
  onSelectMailbox,
  unreadCount,
  provider,
  selectedAccount,
}: MailboxNavProps) {
  const { t } = useLocale();
  const mailboxes = getMailboxes(provider, unreadCount);

  return (
    <nav className="flex w-50 shrink-0 flex-col border-r bg-background">
      <div className="flex h-14 items-center gap-2 px-4 overflow-hidden">
        <Mail className="h-5 w-5 shrink-0" />
        <span className="text-sm font-medium truncate" title={selectedAccount}>
          {selectedAccount || t("app.name")}
        </span>
      </div>

      <Separator />

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 p-2">
          {mailboxes.map((mailbox) => {
            const Icon = mailbox.icon;
            const isSelected = selectedMailbox === mailbox.id;
            const showBadge = (mailbox.id === "INBOX" || mailbox.id === "__UNREAD__") && unreadCount > 0;

            return (
              <Button
                key={mailbox.id}
                variant={isSelected ? "secondary" : "ghost"}
                className="w-full justify-start"
                onClick={() => onSelectMailbox(mailbox.id)}
              >
                <Icon className="mr-2 h-4 w-4" />
                {t(mailbox.labelKey)}
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
