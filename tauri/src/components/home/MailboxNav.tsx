import { Inbox, Send, FileText, Trash2, Mail } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";

interface MailboxNavProps {
  selectedMailbox: string;
  onSelectMailbox: (mailbox: string) => void;
  unreadCount: number;
}

const mailboxes = [
  { id: "INBOX", label: "Inbox", icon: Inbox },
  { id: "Sent", label: "Sent", icon: Send },
  { id: "Drafts", label: "Drafts", icon: FileText },
  { id: "Trash", label: "Trash", icon: Trash2 },
];

export function MailboxNav({
  selectedMailbox,
  onSelectMailbox,
  unreadCount,
}: MailboxNavProps) {
  return (
    <nav className="flex w-55 shrink-0 flex-col border-r bg-background">
      <div className="flex h-14 items-center gap-2 px-4">
        <Mail className="h-5 w-5" />
        <span className="text-lg font-semibold tracking-tight">Mail</span>
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
