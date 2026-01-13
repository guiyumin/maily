import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  Archive,
  ChevronLeft,
  ChevronRight,
  Download,
  Forward,
  Mail,
  MailOpen,
  MoreHorizontal,
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
import { Skeleton } from "@/components/ui/skeleton";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
  onNavigate,
  canNavigatePrev,
  canNavigateNext,
}: EmailReaderProps) {
  const [emailFull, setEmailFull] = useState<EmailFull | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!emailSummary || !account) {
      setEmailFull(null);
      return;
    }

    setLoading(true);
    setError(null);

    invoke<EmailFull>("get_email", {
      account,
      mailbox,
      uid: emailSummary.uid,
    })
      .then((data) => {
        setEmailFull(data);
        // Mark as read when opening
        if (data.unread) {
          invoke("mark_email_read", {
            account,
            mailbox,
            uid: emailSummary.uid,
            unread: false,
          }).catch(console.error);
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
      onEmailDeleted(emailSummary.uid);
    } catch (err) {
      console.error("Failed to delete:", err);
    } finally {
      setDeleting(false);
      setDeleteDialogOpen(false);
    }
  };

  const toggleReadStatus = async () => {
    if (!emailFull) return;
    try {
      const updated = await invoke<EmailFull>("mark_email_read", {
        account,
        mailbox,
        uid: emailFull.uid,
        unread: !emailFull.unread,
      });
      setEmailFull(updated);
    } catch (err) {
      console.error("Failed to update read status:", err);
    }
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

  // Loading state
  if (loading) {
    return (
      <div className="flex flex-1 flex-col bg-background">
        <div className="flex h-14 items-center justify-between border-b px-4">
          <div className="flex items-center gap-1">
            <Skeleton className="h-8 w-8" />
            <Skeleton className="h-8 w-8" />
            <Skeleton className="h-8 w-8" />
          </div>
        </div>
        <div className="flex-1 p-8">
          <Skeleton className="mb-4 h-8 w-3/4" />
          <Skeleton className="mb-2 h-4 w-1/2" />
          <Skeleton className="mb-6 h-4 w-1/3" />
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-background text-destructive">
        <AlertCircle className="mb-4 h-12 w-12" />
        <p className="text-lg font-medium">Failed to load email</p>
        <p className="text-sm">{error}</p>
      </div>
    );
  }

  if (!emailFull) return null;

  return (
    <div className="flex flex-1 flex-col bg-background">
      {/* Toolbar */}
      <div className="flex h-14 shrink-0 items-center justify-between border-b px-4">
        <div className="flex items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Archive className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Archive</TooltipContent>
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
              <Button variant="ghost" size="icon" onClick={toggleReadStatus}>
                {emailFull.unread ? (
                  <MailOpen className="h-4 w-4" />
                ) : (
                  <Mail className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              {emailFull.unread ? "Mark as read" : "Mark as unread"}
            </TooltipContent>
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
      </div>

      {/* Email content */}
      <div className="scrollbar-thin flex-1 overflow-y-auto">
        <div className="mx-auto max-w-4xl p-8">
          {/* Subject */}
          <h1 className="text-2xl font-bold leading-tight">
            {emailFull.subject || "(No subject)"}
          </h1>

          {/* Sender info */}
          <div className="mt-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Avatar className="h-10 w-10">
                <AvatarFallback className="bg-primary text-primary-foreground">
                  {getInitials(emailFull.from)}
                </AvatarFallback>
              </Avatar>
              <div className="flex flex-col">
                <span className="font-semibold">{emailFull.from}</span>
                <span className="text-xs text-muted-foreground">
                  to {emailFull.to}
                  {emailFull.cc && `, cc: ${emailFull.cc}`}
                </span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">
                {new Date(emailFull.date).toLocaleString()}
              </span>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon">
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem>
                    <Sparkles className="mr-2 h-4 w-4" />
                    Summarize with AI
                  </DropdownMenuItem>
                  <DropdownMenuItem>
                    <Calendar className="mr-2 h-4 w-4" />
                    Extract calendar event
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem>View raw message</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          <Separator className="my-6" />

          {/* Attachments */}
          {emailFull.attachments && emailFull.attachments.length > 0 && (
            <div className="mb-6">
              <div className="mb-3 flex items-center gap-2">
                <Paperclip className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm font-medium">
                  {emailFull.attachments.length} attachment
                  {emailFull.attachments.length > 1 ? "s" : ""}
                </span>
              </div>
              <div className="flex flex-wrap gap-2">
                {emailFull.attachments.map((attachment, index) => (
                  <div
                    key={index}
                    className="group flex items-center gap-2 rounded-lg border bg-muted/50 px-3 py-2"
                  >
                    <Paperclip className="h-4 w-4 text-muted-foreground" />
                    <div className="flex flex-col">
                      <span className="max-w-[200px] truncate text-sm font-medium">
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
            {emailFull.body_html ? (
              <IsolatedHtml
                html={emailFull.body_html}
                className="min-h-[100px]"
              />
            ) : (
              <p className="italic text-muted-foreground">
                No content available
              </p>
            )}
          </div>

          {/* Actions */}
          <div className="mt-8 flex gap-3">
            <Button>
              <Reply className="mr-2 h-4 w-4" />
              Reply
            </Button>
            <Button variant="outline">
              <ReplyAll className="mr-2 h-4 w-4" />
              Reply All
            </Button>
            <Button variant="outline">
              <Forward className="mr-2 h-4 w-4" />
              Forward
            </Button>
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
