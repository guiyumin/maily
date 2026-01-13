import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  ArrowLeft,
  Trash2,
  Reply,
  ReplyAll,
  Forward,
  MailOpen,
  Mail,
  Paperclip,
  Download,
  Calendar,
  Sparkles,
  MoreHorizontal,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface Attachment {
  part_id: string;
  filename: string;
  content_type: string;
  size: number;
  encoding: string;
}

interface Email {
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

export const Route = createFileRoute("/email/$account/$mailbox/$uid")({
  component: EmailDetail,
});

function EmailDetail() {
  const { account, mailbox, uid } = Route.useParams();
  const navigate = useNavigate();
  const [email, setEmail] = useState<Email | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    setLoading(true);
    setError(null);

    invoke<Email>("get_email", {
      account,
      mailbox,
      uid: parseInt(uid),
    })
      .then((data) => {
        setEmail(data);
        // Mark as read when opening
        if (data.unread) {
          invoke("mark_email_read", {
            account,
            mailbox,
            uid: parseInt(uid),
            unread: false,
          }).catch(console.error);
        }
      })
      .catch((err) => {
        setError(err.toString());
      })
      .finally(() => setLoading(false));
  }, [account, mailbox, uid]);

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await invoke("delete_email", {
        account,
        mailbox,
        uid: parseInt(uid),
      });
      navigate({ to: "/" });
    } catch (err) {
      console.error("Failed to delete:", err);
    } finally {
      setDeleting(false);
      setDeleteDialogOpen(false);
    }
  };

  const toggleReadStatus = async () => {
    if (!email) return;
    try {
      const updated = await invoke<Email>("mark_email_read", {
        account,
        mailbox,
        uid: parseInt(uid),
        unread: !email.unread,
      });
      setEmail(updated);
    } catch (err) {
      console.error("Failed to update read status:", err);
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  if (loading) {
    return (
      <div className="flex h-screen flex-col bg-background">
        <header className="flex items-center gap-4 border-b px-6 py-4">
          <Skeleton className="h-8 w-8" />
          <Skeleton className="h-6 w-48" />
        </header>
        <div className="flex-1 p-6 space-y-4">
          <Skeleton className="h-8 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-4 w-1/3" />
          <div className="pt-4 space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        </div>
      </div>
    );
  }

  if (error || !email) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background">
        <p className="text-destructive">{error || "Email not found"}</p>
        <Button variant="outline" asChild>
          <Link to="/">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to inbox
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="flex h-screen flex-col bg-background">
        {/* Header */}
        <header className="flex items-center justify-between border-b px-6 py-3">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" asChild>
              <Link to="/">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <Separator orientation="vertical" className="h-6" />
            <span className="text-sm text-muted-foreground">{account}</span>
          </div>

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

            <Separator orientation="vertical" className="h-6 mx-1" />

            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" onClick={toggleReadStatus}>
                  {email.unread ? (
                    <MailOpen className="h-4 w-4" />
                  ) : (
                    <Mail className="h-4 w-4" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {email.unread ? "Mark as read" : "Mark as unread"}
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
        </header>

        {/* Email content */}
        <ScrollArea className="flex-1">
          <div className="mx-auto max-w-4xl p-6">
            {/* Subject */}
            <h1 className="text-2xl font-semibold">
              {email.subject || "(No subject)"}
            </h1>

            {/* Metadata */}
            <div className="mt-4 space-y-2">
              <div className="flex items-start justify-between">
                <div>
                  <p className="font-medium">{email.from}</p>
                  <p className="text-sm text-muted-foreground">
                    To: {email.to}
                    {email.cc && `, Cc: ${email.cc}`}
                  </p>
                </div>
                <time className="text-sm text-muted-foreground">
                  {new Date(email.date).toLocaleString()}
                </time>
              </div>
            </div>

            <Separator className="my-6" />

            {/* Attachments */}
            {email.attachments && email.attachments.length > 0 && (
              <div className="mb-6">
                <div className="flex items-center gap-2 mb-3">
                  <Paperclip className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">
                    {email.attachments.length} attachment
                    {email.attachments.length > 1 ? "s" : ""}
                  </span>
                </div>
                <div className="flex flex-wrap gap-2">
                  {email.attachments.map((attachment, index) => (
                    <div
                      key={index}
                      className="flex items-center gap-2 rounded-lg border bg-muted/50 px-3 py-2"
                    >
                      <Paperclip className="h-4 w-4 text-muted-foreground" />
                      <div className="flex flex-col">
                        <span className="text-sm font-medium truncate max-w-50">
                          {attachment.filename}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {formatFileSize(attachment.size)}
                        </span>
                      </div>
                      <Button variant="ghost" size="icon" className="h-8 w-8">
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
              {email.body_html ? (
                <div
                  className="prose prose-sm dark:prose-invert max-w-none"
                  dangerouslySetInnerHTML={{ __html: email.body_html }}
                />
              ) : (
                <p className="text-muted-foreground italic">
                  No content available
                </p>
              )}
            </div>
          </div>
        </ScrollArea>

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
    </TooltipProvider>
  );
}
