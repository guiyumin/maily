import { useState, useCallback, useRef, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open as openFileDialog } from "@tauri-apps/plugin-dialog";
import {
  X,
  Send,
  Paperclip,
  Trash2,
  Loader2,
  Sparkles,
  ChevronDown,
  ChevronUp,
  Upload,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Collapsible,
  CollapsibleContent,
} from "@/components/ui/collapsible";
import { toast } from "sonner";

interface ComposeEmail {
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  body_html: string;
  body_text: string;
  attachments: AttachmentInfo[];
  reply_to_message_id: string | null;
  references: string | null;
}

interface AttachmentInfo {
  path: string;
  filename: string;
  content_type: string;
}

interface SendResult {
  success: boolean;
  message_id: string | null;
  error: string | null;
}

interface CompletionResponse {
  success: boolean;
  content: string | null;
  error: string | null;
  model_used: string | null;
}

interface Draft {
  id: number | null;
  account: string;
  to: string;
  cc: string;
  bcc: string;
  subject: string;
  body_text: string;
  body_html: string;
  attachments_json: string;
  reply_to_message_id: string | null;
  compose_mode: string;
  created_at: number;
  updated_at: number;
}

interface ComposeProps {
  open: boolean;
  onClose: () => void;
  account: string;
  mode?: "new" | "reply" | "reply-all" | "forward";
  draftId?: number | null;
  originalEmail?: {
    from: string;
    to: string;
    cc: string;
    subject: string;
    body_text: string;
    body_html: string;
    message_id: string;
    date: string;
  };
  onDraftSaved?: (draftId: number) => void;
}

function parseRecipients(value: string): string[] {
  return value
    .split(/[,;]/)
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function getContentType(ext: string): string {
  const mimeTypes: Record<string, string> = {
    // Documents
    pdf: "application/pdf",
    doc: "application/msword",
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    xls: "application/vnd.ms-excel",
    xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    ppt: "application/vnd.ms-powerpoint",
    pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    txt: "text/plain",
    csv: "text/csv",
    // Images
    jpg: "image/jpeg",
    jpeg: "image/jpeg",
    png: "image/png",
    gif: "image/gif",
    webp: "image/webp",
    svg: "image/svg+xml",
    // Archives
    zip: "application/zip",
    rar: "application/x-rar-compressed",
    tar: "application/x-tar",
    gz: "application/gzip",
    // Media
    mp3: "audio/mpeg",
    mp4: "video/mp4",
    wav: "audio/wav",
    // Code
    json: "application/json",
    xml: "application/xml",
    html: "text/html",
    css: "text/css",
    js: "application/javascript",
  };
  return mimeTypes[ext] || "application/octet-stream";
}

export function Compose({
  open,
  onClose,
  account,
  mode = "new",
  draftId: initialDraftId,
  originalEmail,
  onDraftSaved,
}: ComposeProps) {
  const [to, setTo] = useState(() => {
    if (mode === "reply" && originalEmail) {
      return originalEmail.from;
    }
    if (mode === "reply-all" && originalEmail) {
      const recipients = [originalEmail.from];
      if (originalEmail.to) {
        recipients.push(originalEmail.to);
      }
      return recipients.join(", ");
    }
    return "";
  });

  const [cc, setCc] = useState(() => {
    if (mode === "reply-all" && originalEmail?.cc) {
      return originalEmail.cc;
    }
    return "";
  });

  const [bcc, setBcc] = useState("");
  const [showCcBcc, setShowCcBcc] = useState(false);

  const [subject, setSubject] = useState(() => {
    if (!originalEmail) return "";
    if (mode === "reply" || mode === "reply-all") {
      const sub = originalEmail.subject;
      return sub.toLowerCase().startsWith("re:") ? sub : `Re: ${sub}`;
    }
    if (mode === "forward") {
      const sub = originalEmail.subject;
      return sub.toLowerCase().startsWith("fwd:") ? sub : `Fwd: ${sub}`;
    }
    return "";
  });

  const [body, setBody] = useState(() => {
    if (!originalEmail) return "";
    if (mode === "reply" || mode === "reply-all") {
      return `\n\n---\nOn ${originalEmail.date}, ${originalEmail.from} wrote:\n\n${originalEmail.body_text}`;
    }
    if (mode === "forward") {
      return `\n\n---\nForwarded message:\nFrom: ${originalEmail.from}\nDate: ${originalEmail.date}\nSubject: ${originalEmail.subject}\n\n${originalEmail.body_text}`;
    }
    return "";
  });

  const [attachments, setAttachments] = useState<AttachmentInfo[]>([]);
  const [sending, setSending] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [draftId, setDraftId] = useState<number | null>(initialDraftId ?? null);
  const [autoSaving, setAutoSaving] = useState(false);
  const dropZoneRef = useRef<HTMLDivElement>(null);
  const autoSaveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load draft if initialDraftId is provided
  useEffect(() => {
    if (initialDraftId && open) {
      invoke<Draft | null>("get_draft", { id: initialDraftId })
        .then((draft) => {
          if (draft) {
            setTo(draft.to);
            setCc(draft.cc);
            setBcc(draft.bcc);
            setSubject(draft.subject);
            setBody(draft.body_text);
            try {
              const attachmentsData = JSON.parse(draft.attachments_json || "[]");
              setAttachments(attachmentsData);
            } catch {
              // Invalid JSON, ignore
            }
          }
        })
        .catch(console.error);
    }
  }, [initialDraftId, open]);

  // Auto-save draft with debouncing
  const saveDraft = useCallback(async (syncToServer = false) => {
    // Don't save empty drafts
    if (!to && !cc && !bcc && !subject && !body) {
      return;
    }

    setAutoSaving(true);
    try {
      const draft: Draft = {
        id: draftId,
        account,
        to,
        cc,
        bcc,
        subject,
        body_text: body,
        body_html: "",
        attachments_json: JSON.stringify(attachments),
        reply_to_message_id: originalEmail?.message_id || null,
        compose_mode: mode,
        created_at: Date.now(),
        updated_at: Date.now(),
      };

      const newId = await invoke<number>("save_draft", { draft });
      if (!draftId) {
        setDraftId(newId);
        onDraftSaved?.(newId);
      }

      // Sync to IMAP server in background (don't block UI)
      if (syncToServer) {
        invoke("sync_draft_to_server", { draft })
          .then(() => toast.success("Draft synced to server"))
          .catch((err) => console.error("Failed to sync draft to server:", err));
      }
    } catch (err) {
      console.error("Failed to save draft:", err);
    } finally {
      setAutoSaving(false);
    }
  }, [to, cc, bcc, subject, body, attachments, draftId, account, mode, originalEmail, onDraftSaved]);

  // Trigger auto-save when content changes (debounced)
  useEffect(() => {
    if (!open) return;

    if (autoSaveTimeoutRef.current) {
      clearTimeout(autoSaveTimeoutRef.current);
    }

    autoSaveTimeoutRef.current = setTimeout(() => {
      saveDraft();
    }, 2000); // Auto-save after 2 seconds of inactivity

    return () => {
      if (autoSaveTimeoutRef.current) {
        clearTimeout(autoSaveTimeoutRef.current);
      }
    };
  }, [to, cc, bcc, subject, body, attachments, open, saveDraft]);

  // Delete draft on successful send
  const deleteDraftIfExists = useCallback(async () => {
    if (draftId) {
      try {
        await invoke("delete_draft", { id: draftId });
        setDraftId(null);
      } catch (err) {
        console.error("Failed to delete draft:", err);
      }
    }
  }, [draftId]);

  // File picker handler
  const handleAttachFiles = useCallback(async () => {
    try {
      const selected = await openFileDialog({
        multiple: true,
        title: "Select files to attach",
      });

      if (selected) {
        const files = Array.isArray(selected) ? selected : [selected];
        const newAttachments: AttachmentInfo[] = files.map((filePath) => {
          const filename = filePath.split("/").pop() || filePath.split("\\").pop() || "file";
          const ext = filename.split(".").pop()?.toLowerCase() || "";
          const contentType = getContentType(ext);
          return {
            path: filePath,
            filename,
            content_type: contentType,
          };
        });
        setAttachments((prev) => [...prev, ...newAttachments]);
      }
    } catch (err) {
      console.error("Failed to select files:", err);
    }
  }, []);

  // Drag & drop handlers
  const handleDragEnter = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // Only set dragging to false if we're leaving the drop zone
    if (dropZoneRef.current && !dropZoneRef.current.contains(e.relatedTarget as Node)) {
      setIsDragging(false);
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      // Note: In Tauri, dropped files give us paths directly
      const items = e.dataTransfer.items;
      const newAttachments: AttachmentInfo[] = [];

      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.kind === "file") {
          const file = item.getAsFile();
          if (file) {
            // For Tauri, we need the actual file path
            // This works because Tauri provides the path in the drop event
            const filePath = (file as any).path || file.name;
            const ext = file.name.split(".").pop()?.toLowerCase() || "";
            newAttachments.push({
              path: filePath,
              filename: file.name,
              content_type: file.type || getContentType(ext),
            });
          }
        }
      }

      if (newAttachments.length > 0) {
        setAttachments((prev) => [...prev, ...newAttachments]);
        toast.success(`Added ${newAttachments.length} attachment(s)`);
      }
    }
  }, []);

  const handleSend = useCallback(async () => {
    const recipients = parseRecipients(to);
    if (recipients.length === 0) {
      toast.error("Please enter at least one recipient");
      return;
    }

    setSending(true);

    const email: ComposeEmail = {
      to: recipients,
      cc: parseRecipients(cc),
      bcc: parseRecipients(bcc),
      subject,
      body_text: body,
      body_html: "", // Could add rich text editor later
      attachments,
      reply_to_message_id: originalEmail?.message_id || null,
      references: originalEmail?.message_id || null,
    };

    try {
      const result = await invoke<SendResult>("send_email", {
        account,
        email,
      });

      if (result.success) {
        toast.success("Email sent successfully");
        await deleteDraftIfExists();
        onClose();
      } else {
        toast.error(result.error || "Failed to send email");
      }
    } catch (err) {
      toast.error(`Failed to send: ${err}`);
    } finally {
      setSending(false);
    }
  }, [to, cc, bcc, subject, body, attachments, account, originalEmail, onClose, deleteDraftIfExists]);

  const handleGenerateReply = useCallback(
    async (intent: string) => {
      if (!originalEmail) return;

      setGenerating(true);

      try {
        const response = await invoke<CompletionResponse>("generate_reply", {
          originalFrom: originalEmail.from,
          originalSubject: originalEmail.subject,
          originalBody: originalEmail.body_text,
          replyIntent: intent,
        });

        if (response.success && response.content) {
          setBody(response.content);
          toast.success(`Generated with ${response.model_used}`);
        } else {
          toast.error(response.error || "Failed to generate reply");
        }
      } catch (err) {
        toast.error(`Failed to generate: ${err}`);
      } finally {
        setGenerating(false);
      }
    },
    [originalEmail]
  );

  const handleRemoveAttachment = (index: number) => {
    setAttachments((prev) => prev.filter((_, i) => i !== index));
  };

  return (
    <Dialog open={open} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        ref={dropZoneRef}
        className="max-w-2xl max-h-[90vh] flex flex-col relative"
        onDragEnter={handleDragEnter}
        onDragLeave={handleDragLeave}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
      >
        {/* Drag overlay */}
        {isDragging && (
          <div className="absolute inset-0 z-50 flex items-center justify-center bg-primary/10 border-2 border-dashed border-primary rounded-lg">
            <div className="text-center">
              <Upload className="h-12 w-12 mx-auto text-primary mb-2" />
              <p className="text-lg font-medium">Drop files to attach</p>
            </div>
          </div>
        )}

        <DialogHeader className="flex-shrink-0">
          <div className="flex items-center justify-between">
            <DialogTitle>
              {mode === "new" && "New Email"}
              {mode === "reply" && "Reply"}
              {mode === "reply-all" && "Reply All"}
              {mode === "forward" && "Forward"}
            </DialogTitle>
            {autoSaving && (
              <span className="text-xs text-muted-foreground flex items-center gap-1">
                <Loader2 className="h-3 w-3 animate-spin" />
                Saving...
              </span>
            )}
            {!autoSaving && draftId && (
              <span className="text-xs text-muted-foreground">Draft saved</span>
            )}
          </div>
        </DialogHeader>

        <div className="flex flex-col gap-4 flex-1 overflow-hidden">
          {/* From (read-only) */}
          <div className="flex items-center gap-2">
            <Label className="w-16 text-right text-muted-foreground">From</Label>
            <Input value={account} disabled className="flex-1 bg-muted" />
          </div>

          {/* To */}
          <div className="flex items-center gap-2">
            <Label className="w-16 text-right">To</Label>
            <Input
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="recipient@example.com"
              className="flex-1"
            />
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowCcBcc(!showCcBcc)}
            >
              {showCcBcc ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
              Cc/Bcc
            </Button>
          </div>

          {/* Cc/Bcc */}
          <Collapsible open={showCcBcc}>
            <CollapsibleContent className="space-y-2">
              <div className="flex items-center gap-2">
                <Label className="w-16 text-right">Cc</Label>
                <Input
                  value={cc}
                  onChange={(e) => setCc(e.target.value)}
                  placeholder="cc@example.com"
                  className="flex-1"
                />
              </div>
              <div className="flex items-center gap-2">
                <Label className="w-16 text-right">Bcc</Label>
                <Input
                  value={bcc}
                  onChange={(e) => setBcc(e.target.value)}
                  placeholder="bcc@example.com"
                  className="flex-1"
                />
              </div>
            </CollapsibleContent>
          </Collapsible>

          {/* Subject */}
          <div className="flex items-center gap-2">
            <Label className="w-16 text-right">Subject</Label>
            <Input
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="Email subject"
              className="flex-1"
            />
          </div>

          {/* Body */}
          <div className="flex-1 overflow-hidden flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>Message</Label>
              {(mode === "reply" || mode === "reply-all") && originalEmail && (
                <div className="flex gap-1">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleGenerateReply("accept and confirm")}
                    disabled={generating}
                  >
                    <Sparkles className="h-3 w-3 mr-1" />
                    Accept
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleGenerateReply("politely decline")}
                    disabled={generating}
                  >
                    <Sparkles className="h-3 w-3 mr-1" />
                    Decline
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleGenerateReply("ask for more information")}
                    disabled={generating}
                  >
                    <Sparkles className="h-3 w-3 mr-1" />
                    Ask More
                  </Button>
                </div>
              )}
            </div>
            <Textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="Write your message..."
              className="flex-1 min-h-[200px] resize-none"
            />
          </div>

          {/* Attachments */}
          {attachments.length > 0 && (
            <div className="space-y-2">
              <Label>Attachments</Label>
              <div className="flex flex-wrap gap-2">
                {attachments.map((att, index) => (
                  <div
                    key={index}
                    className="flex items-center gap-2 rounded-lg border bg-muted/50 px-3 py-1.5"
                  >
                    <Paperclip className="h-3 w-3 text-muted-foreground" />
                    <span className="text-sm">{att.filename}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-5 w-5"
                      onClick={() => handleRemoveAttachment(index)}
                    >
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center justify-between pt-4 border-t">
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleAttachFiles}>
                <Paperclip className="h-4 w-4 mr-1" />
                Attach
              </Button>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={onClose}>
                <Trash2 className="h-4 w-4 mr-1" />
                Discard
              </Button>
              <Button
                variant="outline"
                onClick={() => saveDraft(true)}
                disabled={autoSaving || sending}
              >
                {autoSaving ? (
                  <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                ) : (
                  <Upload className="h-4 w-4 mr-1" />
                )}
                Save
              </Button>
              <Button onClick={handleSend} disabled={sending || generating}>
                {sending ? (
                  <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                ) : (
                  <Send className="h-4 w-4 mr-1" />
                )}
                Send
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
