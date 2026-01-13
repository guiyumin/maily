import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Forward,
  Loader2,
  Mail,
  MailOpen,
  Paperclip,
  Reply,
  ReplyAll,
  Trash2,
  Calendar,
  Sparkles,
  AlertCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { Email as EmailSummary } from "./EmailList";
import { IsolatedHtml } from "./IsolatedHtml";
import { useEmailCache } from "@/stores/emailCache";

interface Attachment {
  part_id: string;
  filename: string;
  content_type: string;
  size: number;
  encoding: string;
}

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
  attachments: Attachment[];
}

interface EmailReaderProps {
  email: EmailSummary | null;
  account: string;
  mailbox: string;
  onEmailDeleted: (uid: number) => void;
  onEmailReadChange: (uid: number, unread: boolean) => void;
  onNavigate: (direction: "prev" | "next") => void;
  canNavigatePrev: boolean;
  canNavigateNext: boolean;
}

function getInitials(from: string): string {
  const match = from.match(/^([^<]+)</);
  const name = match ? match[1].trim() : from.split("@")[0];
  const parts = name.split(/\s+/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function EmailReader({
  email: emailSummary,
  account,
  mailbox,
  onEmailDeleted,
  onEmailReadChange,
  onNavigate,
  canNavigatePrev,
  canNavigateNext,
}: EmailReaderProps) {
  const [emailFull, setEmailFull] = useState<EmailFull | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const cache = useEmailCache();

  useEffect(() => {
    if (!emailSummary || !account) {
      setEmailFull(null);
      return;
    }

    // Check frontend cache first
    const cached = cache.get(account, mailbox, emailSummary.uid);
    if (cached) {
      setEmailFull(cached);
      setLoading(false);
      setError(null);

      // Still mark as read if unread
      if (cached.unread) {
        const updated = { ...cached, unread: false };
        setEmailFull(updated);
        cache.set(account, mailbox, updated);
        onEmailReadChange(emailSummary.uid, false);

        // Queue IMAP update in background - returns immediately
        invoke("mark_email_read_async", {
          account,
          mailbox,
          uid: emailSummary.uid,
          unread: false,
        }).catch(console.error);
      }
      return;
    }

    // Not in cache, fetch from backend
    setLoading(true);
    setError(null);

    invoke<EmailFull>("get_email", {
      account,
      mailbox,
      uid: emailSummary.uid,
    })
      .then((data) => {
        // Mark as read optimistically - update UI immediately
        if (data.unread) {
          const updated = { ...data, unread: false };
          setEmailFull(updated);
          cache.set(account, mailbox, updated);
          onEmailReadChange(emailSummary.uid, false);

          // Queue IMAP update in background - returns immediately
          invoke("mark_email_read_async", {
            account,
            mailbox,
            uid: emailSummary.uid,
            unread: false,
          }).catch(console.error);
        } else {
          setEmailFull(data);
          cache.set(account, mailbox, data);
        }
      })
      .catch((err) => {
        setError(err.toString());
      })
      .finally(() => setLoading(false));
  }, [emailSummary?.uid, account, mailbox]);

  const handleDelete = async () => {
    if (!emailSummary) return;

    setDeleting(true);
    try {
      await invoke("delete_email", {
        account,
        mailbox,
        uid: emailSummary.uid,
      });
      cache.invalidate(account, mailbox, emailSummary.uid);
      onEmailDeleted(emailSummary.uid);
    } catch (err) {
      console.error("Failed to delete:", err);
    } finally {
      setDeleting(false);
      setDeleteDialogOpen(false);
    }
  };

  const toggleReadStatus = () => {
    if (!emailFull) return;
    const newUnread = !emailFull.unread;
    const updated = { ...emailFull, unread: newUnread };

    // Optimistic update
    setEmailFull(updated);
    cache.set(account, mailbox, updated);
    onEmailReadChange(emailFull.uid, newUnread);

    // Queue IMAP update in background - returns immediately
    invoke("mark_email_read_async", {
      account,
      mailbox,
      uid: emailFull.uid,
      unread: newUnread,
    }).catch(console.error);
  };

  // Empty state
  if (!emailSummary) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-muted/20 text-muted-foreground">
        <Mail className="mb-4 h-12 w-12" />
        <p className="text-lg font-medium">Select an email to read</p>
        <p className="text-sm">Choose an email from the list to view its contents</p>
      </div>
    );
  }

  // Use emailFull if available, otherwise use emailSummary for immediate display
  const displayEmail = emailFull || {
    ...emailSummary,
    message_id: "",
    internal_date: "",
    reply_to: "",
    to: "",
    cc: "",
    body_html: "",
    attachments: [],
  };
  const isUnread = emailFull?.unread ?? emailSummary.unread;

  return (
    <div className="flex flex-1 flex-col bg-background">
      {/* Toolbar */}
      <header className="flex items-center justify-between border-b px-6 py-3">
        <div className="flex items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Reply className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Reply</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <ReplyAll className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Reply All</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Forward className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Forward</TooltipContent>
          </Tooltip>

          <Separator orientation="vertical" className="mx-1 h-6" />

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" onClick={toggleReadStatus} disabled={!emailFull}>
                {isUnread ? (
                  <MailOpen className="h-4 w-4" />
                ) : (
                  <Mail className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              {isUnread ? "Mark as read" : "Mark as unread"}
            </TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setDeleteDialogOpen(true)}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Delete</TooltipContent>
          </Tooltip>

          <Separator orientation="vertical" className="mx-1 h-6" />

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Sparkles className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Summarize with AI</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Calendar className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Extract calendar event</TooltipContent>
          </Tooltip>
        </div>

        <div className="flex items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => onNavigate("prev")}
                disabled={!canNavigatePrev}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Previous</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => onNavigate("next")}
                disabled={!canNavigateNext}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Next</TooltipContent>
          </Tooltip>
        </div>
      </header>

      {/* Email content */}
      <div className="scrollbar-thin flex-1 overflow-y-auto">
        <div className="mx-auto max-w-4xl p-8">
          {/* Subject */}
          <h1 className="text-2xl font-bold leading-tight">
            {displayEmail.subject || "(No subject)"}
          </h1>

          {/* Sender info */}
          <div className="mt-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Avatar className="h-10 w-10">
                <AvatarFallback className="bg-primary text-primary-foreground">
                  {getInitials(displayEmail.from)}
                </AvatarFallback>
              </Avatar>
              <div className="flex flex-col">
                <span className="font-semibold">{displayEmail.from}</span>
                {displayEmail.to && (
                  <span className="text-xs text-muted-foreground">
                    to {displayEmail.to}
                    {displayEmail.cc && `, cc: ${displayEmail.cc}`}
                  </span>
                )}
              </div>
            </div>
            <span className="text-sm text-muted-foreground">
              {new Date(displayEmail.date).toLocaleString()}
            </span>
          </div>

          <Separator className="my-6" />

          {/* Attachments */}
          {displayEmail.attachments && displayEmail.attachments.length > 0 && (
            <div className="mb-6">
              <div className="mb-3 flex items-center gap-2">
                <Paperclip className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm font-medium">
                  {displayEmail.attachments.length} attachment
                  {displayEmail.attachments.length > 1 ? "s" : ""}
                </span>
              </div>
              <div className="flex flex-wrap gap-2">
                {displayEmail.attachments.map((attachment, index) => (
                  <div
                    key={index}
                    className="group flex items-center gap-2 rounded-lg border bg-muted/50 px-3 py-2"
                  >
                    <Paperclip className="h-4 w-4 text-muted-foreground" />
                    <div className="flex flex-col">
                      <span className="max-w-50 truncate text-sm font-medium">
                        {attachment.filename}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {formatFileSize(attachment.size)}
                      </span>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 opacity-0 transition-opacity group-hover:opacity-100"
                    >
                      <Download className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
              </div>
              <Separator className="mt-6" />
            </div>
          )}

          {/* Body */}
          <div className="text-sm leading-relaxed">
            {loading ? (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>Loading...</span>
              </div>
            ) : error ? (
              <div className="flex items-center gap-2 text-destructive">
                <AlertCircle className="h-4 w-4" />
                <span>{error}</span>
              </div>
            ) : displayEmail.body_html ? (
              <IsolatedHtml
                html={displayEmail.body_html}
                className="min-h-25"
              />
            ) : (
              <p className="italic text-muted-foreground">
                No content available
              </p>
            )}
          </div>

        </div>
      </div>

      {/* Delete confirmation dialog */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this email?</AlertDialogTitle>
            <AlertDialogDescription>
              This will move the email to trash. You can recover it from the
              trash folder.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
