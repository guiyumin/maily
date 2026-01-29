import { useEffect, useState, useCallback } from "react";
import Markdown from "react-markdown";
import { Temporal } from "temporal-polyfill";
import { invoke } from "@tauri-apps/api/core";
import {
  useAIProviders,
  completeWithFallback,
  buildSummarizePrompt,
  SUMMARIZE_MAX_TOKENS,
  buildExtractEventPrompt,
  EXTRACT_EVENT_SYSTEM_PROMPT,
  EXTRACT_EVENT_MAX_TOKENS,
  buildExtractReminderPrompt,
  EXTRACT_REMINDER_SYSTEM_PROMPT,
  EXTRACT_REMINDER_MAX_TOKENS,
  buildParseEventNlpPrompt,
  PARSE_EVENT_NLP_MAX_TOKENS,
} from "@/lib/ai";
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Forward,
  Loader2,
  Mail,
  MailOpen,
  MapPin,
  Paperclip,
  Reply,
  ReplyAll,
  Trash2,
  Calendar,
  Sparkles,
  AlertCircle,
  RefreshCw,
  CalendarPlus,
  ListTodo,
  Tags,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { Email as EmailSummary } from "./EmailList";
import { IsolatedHtml } from "./IsolatedHtml";
import { useEmailCache } from "@/stores/emailCache";
import { Compose } from "@/components/compose/Compose";
import { toast } from "sonner";
import { useLocale } from "@/lib/i18n";
import { TagDialog } from "@/components/tags/TagDialog";
import { TagList } from "@/components/tags/TagList";
import { getEmailTags } from "@/lib/tags";
import type { EmailTag } from "@/types/tags";
import type { AIProviderConfig } from "@/lib/ai/types";
import { extractPrimaryContent } from "@/lib/email";

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
  onSnippetUpdate: (uid: number, snippet: string) => void;
  onTagsChange?: (uid: number, tags: EmailTag[]) => void;
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

interface ExtractedEvent {
  title: string;
  start_time: string;
  end_time: string;
  location?: string;
  alarm_minutes_before?: number;
  alarm_specified?: boolean;
}

interface ExtractedReminder {
  title: string;
  notes: string;
  due_date: string;
  priority: number;
}

interface ExtractedReminderDisplayProps {
  reminderJson: string | null;
  emailFrom: string;
  emailSubject: string;
  onClose: () => void;
}

interface ExtractedEventDisplayProps {
  eventJson: string | null; // null means extraction failed or not attempted
  extractionError?: string | null; // error message from AI provider
  emailFrom: string;
  emailSubject: string;
  emailBody: string;
  providerConfigs: AIProviderConfig[];
  onClose: () => void;
}

// Format date/time in user's system timezone for HTML inputs
const USER_TIMEZONE = Temporal.Now.timeZoneId();
const dateFormatter = new Intl.DateTimeFormat("sv-SE", {
  timeZone: USER_TIMEZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});
const timeFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: USER_TIMEZONE,
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function formatDateForInput(date: Date): string {
  return dateFormatter.format(date); // YYYY-MM-DD
}

function formatTimeForInput(date: Date): string {
  return timeFormatter.format(date); // HH:MM
}

// Strip markdown code fences from AI response
function stripMarkdownCodeFences(s: string): string {
  s = s.trim();

  // Remove opening fence (```json or ```)
  if (s.startsWith("```")) {
    const idx = s.indexOf("\n");
    if (idx !== -1) {
      s = s.slice(idx + 1);
    }
  }

  // Remove closing fence
  if (s.endsWith("```")) {
    s = s.slice(0, -3);
  }

  return s.trim();
}

function ExtractedEventDisplay({
  eventJson,
  extractionError,
  emailFrom,
  emailSubject,
  emailBody,
  providerConfigs,
  onClose,
}: ExtractedEventDisplayProps) {
  const [adding, setAdding] = useState(false);
  const [title, setTitle] = useState("");
  const [date, setDate] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [location, setLocation] = useState("");
  const [alarm, setAlarm] = useState("15");
  const [showForm, setShowForm] = useState(false);
  const [nlpInput, setNlpInput] = useState("");
  const [nlpProcessing, setNlpProcessing] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Parse the event JSON and populate form fields
  useEffect(() => {
    if (!eventJson) {
      // Show the actual error from AI provider if available
      if (extractionError) {
        setErrorMessage(`AI error: ${extractionError}`);
      } else {
        setErrorMessage("Failed to extract event. Describe the event below:");
      }
      return;
    }

    const raw = eventJson.trim();

    // Check for NO_EVENTS_FOUND response
    if (raw === "NO_EVENTS_FOUND" || raw.includes("NO_EVENTS_FOUND")) {
      setErrorMessage(
        "No calendar event found in this email. Describe the event below:",
      );
      return;
    }

    try {
      // Strip markdown code fences if present
      const cleaned = stripMarkdownCodeFences(raw);
      console.log("Parsing event JSON:", cleaned);
      const event: ExtractedEvent = JSON.parse(cleaned);

      setTitle(event.title || "");
      setLocation(event.location || "");

      const startDate = new Date(event.start_time);
      const endDate = new Date(event.end_time);

      if (!isNaN(startDate.getTime())) {
        setDate(formatDateForInput(startDate));
        setStartTime(formatTimeForInput(startDate));
      }
      if (!isNaN(endDate.getTime())) {
        setEndTime(formatTimeForInput(endDate));
      }

      if (event.alarm_specified && event.alarm_minutes_before !== undefined) {
        setAlarm(event.alarm_minutes_before.toString());
      }

      setShowForm(true);
      setErrorMessage(null);
    } catch (e) {
      console.error("Failed to parse event JSON. Raw response:", raw, e);
      setErrorMessage("Failed to parse event. Describe the event below:");
    }
  }, [eventJson, extractionError]);

  // Parse NLP input with email context
  const handleParseNlp = async () => {
    if (!nlpInput.trim()) return;

    setNlpProcessing(true);
    try {
      const prompt = buildParseEventNlpPrompt({
        userInput: nlpInput,
        emailFrom,
        emailSubject,
        emailBody,
      });

      const response = await completeWithFallback(
        { prompt, maxTokens: PARSE_EVENT_NLP_MAX_TOKENS },
        providerConfigs,
      );

      if (response.success && response.content) {
        const cleaned = stripMarkdownCodeFences(response.content);
        console.log("Parsed NLP response:", cleaned);
        const event: ExtractedEvent = JSON.parse(cleaned);

        setTitle(event.title || "");
        setLocation(event.location || "");

        const startDate = new Date(event.start_time);
        const endDate = new Date(event.end_time);

        if (!isNaN(startDate.getTime())) {
          setDate(formatDateForInput(startDate));
          setStartTime(formatTimeForInput(startDate));
        }
        if (!isNaN(endDate.getTime())) {
          setEndTime(formatTimeForInput(endDate));
        }

        if (event.alarm_specified && event.alarm_minutes_before !== undefined) {
          setAlarm(event.alarm_minutes_before.toString());
        }

        setShowForm(true);
        setErrorMessage(null);
        setNlpInput("");
      } else {
        toast.error(response.error || "Failed to parse event");
      }
    } catch (e) {
      console.error("NLP parse error:", e);
      toast.error(`Failed to parse: ${e}`);
    } finally {
      setNlpProcessing(false);
    }
  };

  // Show NLP input when there's an error or no event found
  if (!showForm) {
    return (
      <div className="space-y-4">
        {errorMessage && (
          <div className="flex items-center gap-2 text-muted-foreground">
            <AlertCircle className="h-4 w-4" />
            <span>{errorMessage}</span>
          </div>
        )}

        {/* NLP Input */}
        <div className="space-y-2">
          <Label htmlFor="nlp-input">Describe the event</Label>
          <Textarea
            id="nlp-input"
            placeholder='e.g., "Meeting tomorrow at 3pm at the coffee shop" or "the event mentioned in the email"'
            value={nlpInput}
            onChange={(e) => setNlpInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                e.preventDefault();
                handleParseNlp();
              }
            }}
            rows={6}
            className="resize-none"
          />
          <div className="flex items-center justify-between">
            <p className="text-xs text-muted-foreground">
              Use natural language. Press ⌘+Enter to parse.
            </p>
            <Button
              size="sm"
              onClick={handleParseNlp}
              disabled={nlpProcessing || !nlpInput.trim()}
            >
              {nlpProcessing ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Parse"
              )}
            </Button>
          </div>
        </div>

        <div className="flex justify-end">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  const handleAddToCalendar = async () => {
    console.log("handleAddToCalendar called with state:", {
      title,
      date,
      startTime,
      endTime,
      location,
      alarm,
    });

    if (!title.trim()) {
      toast.error("Title is required");
      return;
    }

    if (!date) {
      toast.error("Date is required");
      return;
    }

    if (!startTime || !endTime) {
      toast.error("Start and end time are required");
      return;
    }

    setAdding(true);
    try {
      // Check calendar access first
      const authStatus = await invoke<string>("calendar_get_auth_status");
      console.log("Calendar auth status:", authStatus);

      if (authStatus !== "authorized") {
        // Request access
        await invoke("calendar_request_access");
        const newStatus = await invoke<string>("calendar_get_auth_status");
        if (newStatus !== "authorized") {
          toast.error(
            "Calendar access denied. Please enable in System Settings → Privacy & Security → Calendars",
          );
          setAdding(false);
          return;
        }
      }

      const startDateTime = new Date(`${date}T${startTime}`);
      const endDateTime = new Date(`${date}T${endTime}`);

      console.log("Creating event:", {
        date,
        startTime,
        endTime,
        startDateTime,
        endDateTime,
      });

      if (isNaN(startDateTime.getTime()) || isNaN(endDateTime.getTime())) {
        toast.error("Invalid date or time format");
        setAdding(false);
        return;
      }

      const newEvent = {
        title: title.trim(),
        start_time: Math.floor(startDateTime.getTime() / 1000),
        end_time: Math.floor(endDateTime.getTime() / 1000),
        location: location.trim(),
        notes: "",
        calendar_id: "", // Will use default calendar
        all_day: false,
        alarm_minutes_before: parseInt(alarm) || 0,
      };

      console.log("Invoking calendar_create_event with:", newEvent);
      await invoke("calendar_create_event", { event: newEvent });
      toast.success("Event added to calendar");
      onClose();
    } catch (err) {
      console.error("Failed to create calendar event:", err);
      toast.error(`Failed to add event: ${err}`);
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Title */}
      <div className="space-y-2">
        <Label htmlFor="event-title">Title</Label>
        <Input
          id="event-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Event title"
        />
      </div>

      {/* Date & Time */}
      <div className="grid grid-cols-3 gap-3">
        <div className="space-y-2">
          <Label htmlFor="event-date">Date</Label>
          <Input
            id="event-date"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="event-start">Start</Label>
          <Input
            id="event-start"
            type="time"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="event-end">End</Label>
          <Input
            id="event-end"
            type="time"
            value={endTime}
            onChange={(e) => setEndTime(e.target.value)}
          />
        </div>
      </div>

      {/* Location */}
      <div className="space-y-2">
        <Label htmlFor="event-location">Location</Label>
        <div className="relative">
          <MapPin className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="event-location"
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="Add location"
            className="pl-9"
          />
        </div>
      </div>

      {/* Reminder */}
      <div className="space-y-2">
        <Label>Reminder</Label>
        <Select value={alarm} onValueChange={setAlarm}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="0">None</SelectItem>
            <SelectItem value="5">5 minutes before</SelectItem>
            <SelectItem value="15">15 minutes before</SelectItem>
            <SelectItem value="30">30 minutes before</SelectItem>
            <SelectItem value="60">1 hour before</SelectItem>
            <SelectItem value="1440">1 day before</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Actions */}
      <div className="flex justify-end gap-2 pt-2 border-t">
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button
          onClick={handleAddToCalendar}
          disabled={adding || !title.trim()}
        >
          {adding ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <CalendarPlus className="mr-2 h-4 w-4" />
          )}
          Add to Calendar
        </Button>
      </div>
    </div>
  );
}

function ExtractedReminderDisplay({
  reminderJson,
  emailFrom,
  emailSubject,
  onClose,
}: ExtractedReminderDisplayProps) {
  const [adding, setAdding] = useState(false);
  const [title, setTitle] = useState("");
  const [notes, setNotes] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [dueTime, setDueTime] = useState("09:00");
  const [priority, setPriority] = useState("5");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Parse the reminder JSON and populate form fields
  useEffect(() => {
    if (!reminderJson) {
      // No extraction - use defaults
      setTitle(`Follow up: ${emailSubject}`);
      setNotes(`From: ${emailFrom}`);
      const tomorrow = new Date();
      tomorrow.setDate(tomorrow.getDate() + 1);
      setDueDate(tomorrow.toISOString().split("T")[0]);
      setDueTime("09:00");
      setPriority("5");
      return;
    }

    const raw = reminderJson.trim();

    // Check for NO_TASK_FOUND response
    if (raw === "NO_TASK_FOUND" || raw.includes("NO_TASK_FOUND")) {
      setTitle(`Follow up: ${emailSubject}`);
      setNotes(`From: ${emailFrom}`);
      const tomorrow = new Date();
      tomorrow.setDate(tomorrow.getDate() + 1);
      setDueDate(tomorrow.toISOString().split("T")[0]);
      setDueTime("09:00");
      setPriority("5");
      setErrorMessage("No specific task found. Using defaults:");
      return;
    }

    try {
      const cleaned = stripMarkdownCodeFences(raw);
      const reminder: ExtractedReminder = JSON.parse(cleaned);

      setTitle(reminder.title || "");
      setNotes(reminder.notes || "");
      setPriority(reminder.priority?.toString() || "5");

      if (reminder.due_date) {
        const date = new Date(reminder.due_date);
        if (!isNaN(date.getTime())) {
          setDueDate(date.toISOString().split("T")[0]);
          setDueTime(date.toTimeString().slice(0, 5));
        }
      }

      setErrorMessage(null);
    } catch (e) {
      console.error("Failed to parse reminder JSON:", raw, e);
      setTitle(`Follow up: ${emailSubject}`);
      setNotes(`From: ${emailFrom}`);
      const tomorrow = new Date();
      tomorrow.setDate(tomorrow.getDate() + 1);
      setDueDate(tomorrow.toISOString().split("T")[0]);
      setErrorMessage("Failed to parse AI response. Using defaults:");
    }
  }, [reminderJson, emailFrom, emailSubject]);

  const handleAddToReminders = async () => {
    if (!title.trim()) {
      toast.error("Title is required");
      return;
    }

    setAdding(true);
    try {
      // Check reminders access
      const authStatus = await invoke<string>("reminders_get_auth_status");

      if (authStatus !== "authorized") {
        await invoke("reminders_request_access");
        const newStatus = await invoke<string>("reminders_get_auth_status");
        if (newStatus !== "authorized") {
          toast.error(
            "Reminders access denied. Please enable in System Settings → Privacy & Security → Reminders",
          );
          setAdding(false);
          return;
        }
      }

      // Calculate due date timestamp
      let dueDateTimestamp: number | undefined;
      if (dueDate) {
        const dateTime = new Date(`${dueDate}T${dueTime}`);
        if (!isNaN(dateTime.getTime())) {
          dueDateTimestamp = Math.floor(dateTime.getTime() / 1000);
        }
      }

      await invoke("reminders_create", {
        reminder: {
          title: title.trim(),
          notes: notes.trim(),
          due_date: dueDateTimestamp,
          priority: parseInt(priority) || 5,
          list_id: "",
        },
      });

      toast.success("Reminder created");
      onClose();
    } catch (err) {
      console.error("Failed to create reminder:", err);
      toast.error(`Failed to create reminder: ${err}`);
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="space-y-4">
      {errorMessage && (
        <div className="flex items-center gap-2 text-muted-foreground text-sm">
          <AlertCircle className="h-4 w-4" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Title */}
      <div className="space-y-2">
        <Label htmlFor="reminder-title">Title</Label>
        <Input
          id="reminder-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Task title"
        />
      </div>

      {/* Due Date & Time */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="reminder-date">Due Date</Label>
          <Input
            id="reminder-date"
            type="date"
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="reminder-time">Time</Label>
          <Input
            id="reminder-time"
            type="time"
            value={dueTime}
            onChange={(e) => setDueTime(e.target.value)}
          />
        </div>
      </div>

      {/* Priority */}
      <div className="space-y-2">
        <Label>Priority</Label>
        <Select value={priority} onValueChange={setPriority}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="1">High</SelectItem>
            <SelectItem value="5">Medium</SelectItem>
            <SelectItem value="9">Low</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Notes */}
      <div className="space-y-2">
        <Label htmlFor="reminder-notes">Notes</Label>
        <Textarea
          id="reminder-notes"
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          placeholder="Additional notes"
          rows={3}
          className="resize-none"
        />
      </div>

      {/* Actions */}
      <div className="flex justify-end gap-2 pt-2 border-t">
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button
          onClick={handleAddToReminders}
          disabled={adding || !title.trim()}
        >
          {adding ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <ListTodo className="mr-2 h-4 w-4" />
          )}
          Add to Reminders
        </Button>
      </div>
    </div>
  );
}

export function EmailReader({
  email: emailSummary,
  account,
  mailbox,
  onEmailDeleted,
  onEmailReadChange,
  onSnippetUpdate,
  onTagsChange,
  onNavigate,
  canNavigatePrev,
  canNavigateNext,
}: EmailReaderProps) {
  const { t } = useLocale();
  const { providerConfigs } = useAIProviders();
  const [emailFull, setEmailFull] = useState<EmailFull | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Compose dialog state
  const [composeOpen, setComposeOpen] = useState(false);
  const [composeMode, setComposeMode] = useState<
    "new" | "reply" | "reply-all" | "forward"
  >("reply");

  // AI features state
  const [summaryDialogOpen, setSummaryDialogOpen] = useState(false);
  const [summary, setSummary] = useState<string | null>(null);
  const [summaryModelUsed, setSummaryModelUsed] = useState<string | null>(null);
  const [summarizing, setSummarizing] = useState(false);

  const [eventDialogOpen, setEventDialogOpen] = useState(false);
  const [extractedEvent, setExtractedEvent] = useState<string | null>(null);
  const [extractionError, setExtractionError] = useState<string | null>(null);
  const [extracting, setExtracting] = useState(false);

  const [reminderDialogOpen, setReminderDialogOpen] = useState(false);
  const [extractedReminder, setExtractedReminder] = useState<string | null>(
    null,
  );
  const [extractingReminder, setExtractingReminder] = useState(false);

  // Tags state
  const [tagDialogOpen, setTagDialogOpen] = useState(false);
  const [emailTags, setEmailTags] = useState<EmailTag[]>([]);

  // Handle tag changes - update local state and propagate to parent
  const handleTagsChange = useCallback(
    (newTags: EmailTag[]) => {
      setEmailTags(newTags);
      if (emailSummary && onTagsChange) {
        onTagsChange(emailSummary.uid, newTags);
      }
    },
    [emailSummary, onTagsChange],
  );

  const cache = useEmailCache();

  // Fetch email - shows metadata immediately, loads body in background if needed
  const fetchEmailBody = useCallback(
    async (uid: number, skipCache = false) => {
      if (!account) return;

      // Check frontend cache first (unless skipping)
      if (!skipCache) {
        const cached = cache.get(account, mailbox, uid);
        if (cached) {
          setEmailFull(cached);
          setLoading(false);
          setError(null);

          // Still mark as read if unread
          if (cached.unread) {
            const updated = { ...cached, unread: false };
            setEmailFull(updated);
            cache.set(account, mailbox, updated);
            onEmailReadChange(uid, false);

            invoke("mark_email_read_async", {
              account,
              mailbox,
              uid,
              unread: false,
            }).catch(console.error);
          }
          return;
        }
      }

      // Step 1: Get email metadata from cache immediately (non-blocking)
      setError(null);

      try {
        const metadata = await invoke<EmailFull>("get_email", {
          account,
          mailbox,
          uid,
        });

        // Show metadata immediately
        setEmailFull(metadata);

        // Mark as read
        if (metadata.unread) {
          const updated = { ...metadata, unread: false };
          setEmailFull(updated);
          onEmailReadChange(uid, false);
          invoke("mark_email_read_async", {
            account,
            mailbox,
            uid,
            unread: false,
          }).catch(console.error);
        }

        // Step 2: If body is empty, fetch it in background
        if (!metadata.body_html) {
          setLoading(true); // Show loading spinner in body area only

          try {
            const result = await invoke<[string, string] | null>(
              "fetch_email_body_async",
              {
                account,
                mailbox,
                uid,
              },
            );

            if (result) {
              const [body_html, snippet] = result;

              // Update the email state with body
              setEmailFull((prev) => {
                if (!prev || prev.uid !== uid) return prev;
                const updated = { ...prev, body_html, snippet };
                cache.set(account, mailbox, updated);
                return updated;
              });

              // Update snippet in email list
              onSnippetUpdate(uid, snippet);

              // Update backend cache
              invoke("update_email_body_cache", {
                account,
                mailbox,
                uid,
                bodyHtml: body_html,
                snippet,
              }).catch(console.error);
            } else {
              setError("Email body not found on server");
            }
          } catch (bodyErr) {
            setError(bodyErr?.toString() || "Failed to load email body");
          } finally {
            setLoading(false);
          }
        } else {
          // Body already cached
          cache.set(account, mailbox, metadata);
        }
      } catch (err) {
        setError(err?.toString() || "Failed to load email");
      }
    },
    [account, mailbox, cache, onEmailReadChange, onSnippetUpdate],
  );

  // Retry loading email body (skips cache, forces fresh IMAP fetch)
  const handleRetryLoad = useCallback(() => {
    if (!emailSummary) return;
    // Invalidate frontend cache and fetch fresh
    cache.invalidate(account, mailbox, emailSummary.uid);
    fetchEmailBody(emailSummary.uid, true);
  }, [emailSummary, account, mailbox, cache, fetchEmailBody]);

  useEffect(() => {
    if (!emailSummary || !account) {
      setEmailFull(null);
      return;
    }
    // Clear stale emailFull immediately when email changes
    // This prevents showing old email's content while new email loads
    setEmailFull(null);
    fetchEmailBody(emailSummary.uid);
  }, [emailSummary?.uid, account, mailbox]);

  // Fetch tags when email changes
  useEffect(() => {
    if (!emailSummary || !account) {
      setEmailTags([]);
      return;
    }
    getEmailTags(account, mailbox, emailSummary.uid)
      .then(setEmailTags)
      .catch(console.error);
  }, [emailSummary?.uid, account, mailbox]);

  const handleDelete = async (permanent = false) => {
    if (!emailSummary) return;

    setDeleting(true);
    try {
      await invoke(permanent ? "permanent_delete_email" : "delete_email", {
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

  // Compose handlers
  const handleReply = useCallback(() => {
    setComposeMode("reply");
    setComposeOpen(true);
  }, []);

  const handleReplyAll = useCallback(() => {
    setComposeMode("reply-all");
    setComposeOpen(true);
  }, []);

  const handleForward = useCallback(() => {
    setComposeMode("forward");
    setComposeOpen(true);
  }, []);

  // AI handlers - use JS SDK for API providers, Rust for CLI
  const handleSummarize = async (forceRefresh = false) => {
    if (!emailFull) return;

    // 1. Open dialog + 2. Set loading
    setSummaryDialogOpen(true);
    setSummarizing(true);

    try {
      // Check cache first (unless force refresh)
      if (!forceRefresh) {
        const cached = await invoke<{
          summary: string;
          model_used: string;
        } | null>("get_email_summary", {
          account,
          mailbox,
          uid: emailFull.uid,
        });
        if (cached) {
          setSummary(cached.summary);
          setSummaryModelUsed(cached.model_used);
          setSummarizing(false);
          return;
        }
      }

      // Generate new summary via JS SDK
      const bodyText = extractPrimaryContent(emailFull.body_html);

      const prompt = buildSummarizePrompt({
        from: emailFull.from,
        subject: emailFull.subject,
        bodyText,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      });

      const response = await completeWithFallback(
        { prompt, maxTokens: SUMMARIZE_MAX_TOKENS },
        providerConfigs,
      );

      if (response.success && response.content) {
        setSummary(response.content);
        setSummaryModelUsed(response.modelUsed);
        // Cache the summary
        await invoke("save_email_summary", {
          account,
          mailbox,
          uid: emailFull.uid,
          summary: response.content,
          modelUsed: response.modelUsed || "unknown",
        });
      } else {
        toast.error(response.error || "Failed to summarize email");
        setSummaryDialogOpen(false);
      }
    } catch (err) {
      toast.error(`Failed to summarize: ${err}`);
      setSummaryDialogOpen(false);
    } finally {
      setSummarizing(false);
    }
  };

  const handleExtractEvent = async () => {
    if (!emailFull) return;

    setEventDialogOpen(true);
    setExtracting(true);
    setExtractionError(null);

    try {
      const bodyText = extractPrimaryContent(emailFull.body_html);

      const prompt = buildExtractEventPrompt({
        from: emailFull.from,
        subject: emailFull.subject,
        bodyText,
        userTimezone: USER_TIMEZONE,
      });

      const response = await completeWithFallback(
        {
          prompt,
          systemPrompt: EXTRACT_EVENT_SYSTEM_PROMPT,
          maxTokens: EXTRACT_EVENT_MAX_TOKENS,
        },
        providerConfigs,
      );

      if (response.success && response.content) {
        setExtractedEvent(response.content);
        setExtractionError(null);
      } else {
        // Don't close dialog - show NLP input for manual entry
        setExtractedEvent(null);
        setExtractionError(response.error || "Extraction failed");
        console.error("Extract event failed:", response.error);
      }
    } catch (err) {
      // Don't close dialog - show NLP input for manual entry
      setExtractedEvent(null);
      setExtractionError(String(err));
      console.error("Extract event error:", err);
    } finally {
      setExtracting(false);
    }
  };

  const handleExtractReminder = async () => {
    if (!emailFull) return;

    setReminderDialogOpen(true);
    setExtractingReminder(true);

    try {
      const bodyText = extractPrimaryContent(emailFull.body_html);

      const prompt = buildExtractReminderPrompt({
        from: emailFull.from,
        subject: emailFull.subject,
        bodyText,
      });

      const response = await completeWithFallback(
        {
          prompt,
          systemPrompt: EXTRACT_REMINDER_SYSTEM_PROMPT,
          maxTokens: EXTRACT_REMINDER_MAX_TOKENS,
        },
        providerConfigs,
      );

      if (response.success && response.content) {
        setExtractedReminder(response.content);
      } else {
        // Show dialog with defaults even if extraction fails
        setExtractedReminder(null);
        console.error("Extract reminder failed:", response.error);
      }
    } catch (err) {
      // Show dialog with defaults
      setExtractedReminder(null);
      console.error("Extract reminder error:", err);
    } finally {
      setExtractingReminder(false);
    }
  };

  // Reset AI state when email changes
  useEffect(() => {
    setSummary(null);
    setSummaryModelUsed(null);
    setExtractedEvent(null);
    setExtractionError(null);
    setExtractedReminder(null);
  }, [emailSummary?.uid]);

  // Empty state
  if (!emailSummary) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-muted/20 text-muted-foreground">
        <Mail className="mb-4 h-12 w-12" />
        <p className="text-lg font-medium">{t("mail.selectEmail")}</p>
        <p className="text-sm">{t("mail.selectEmailDescription")}</p>
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
              <Button
                variant="ghost"
                size="icon"
                onClick={handleReply}
                disabled={!emailFull}
              >
                <Reply className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Reply</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleReplyAll}
                disabled={!emailFull}
              >
                <ReplyAll className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Reply All</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleForward}
                disabled={!emailFull}
              >
                <Forward className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Forward</TooltipContent>
          </Tooltip>

          <Separator orientation="vertical" className="mx-1 h-6" />

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={toggleReadStatus}
                disabled={!emailFull}
              >
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

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setTagDialogOpen(true)}
                disabled={!emailFull}
              >
                <Tags className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t("tags.manageTags")}</TooltipContent>
          </Tooltip>

          <Separator orientation="vertical" className="mx-1 h-6" />

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => handleSummarize(false)}
                disabled={!emailFull || summarizing}
              >
                {summarizing ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Sparkles className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>Summarize with AI</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleExtractEvent}
                disabled={!emailFull || extracting}
              >
                {extracting ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Calendar className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>Extract calendar event</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleExtractReminder}
                disabled={!emailFull || extractingReminder}
              >
                {extractingReminder ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ListTodo className="h-4 w-4" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>Add to Reminders</TooltipContent>
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

          <Separator className="my-3" />

          {/* Tags */}
          {emailTags.length > 0 && (
            <div className="mb-4">
              <TagList tags={emailTags} maxDisplay={10} />
            </div>
          )}

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
              <div className="flex flex-col gap-3">
                <div className="flex items-center gap-2 text-destructive">
                  <AlertCircle className="h-4 w-4" />
                  <span>{error}</span>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleRetryLoad}
                  className="w-fit"
                >
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Retry
                </Button>
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
              Choose how to delete this email.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter className="flex-col sm:flex-row gap-2">
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleDelete(false)}
              disabled={deleting}
              className="bg-secondary text-secondary-foreground hover:bg-secondary/80"
            >
              {deleting ? "Moving..." : "Move to Trash"}
            </AlertDialogAction>
            <AlertDialogAction
              onClick={() => handleDelete(true)}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleting ? "Deleting..." : "Permanent Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Summary dialog */}
      <Dialog open={summaryDialogOpen} onOpenChange={setSummaryDialogOpen}>
        <DialogContent className="max-w-2xl p-6">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Sparkles className="h-5 w-5" />
              Email Summary
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            {summarizing ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : summary ? (
              <>
                <div className="max-h-[60vh] overflow-y-auto pr-2">
                  <div className="text-sm leading-relaxed prose prose-sm dark:prose-invert max-w-none">
                    <Markdown
                      components={{
                        p: ({ children }) => <p className="mb-2">{children}</p>,
                        ul: ({ children }) => (
                          <ul className="space-y-1">{children}</ul>
                        ),
                        strong: ({ children }) => (
                          <strong className="font-semibold">{children}</strong>
                        ),
                        a: ({ href, children }) => (
                          <a
                            href={href}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-primary underline"
                          >
                            {children}
                          </a>
                        ),
                      }}
                    >
                      {summary}
                    </Markdown>
                  </div>
                </div>
                <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
                  <span>Generated by {summaryModelUsed}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleSummarize(true)}
                    disabled={summarizing}
                  >
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Regenerate
                  </Button>
                </div>
              </>
            ) : null}
          </div>
        </DialogContent>
      </Dialog>

      {/* Extract event dialog */}
      <Dialog open={eventDialogOpen} onOpenChange={setEventDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Calendar className="h-5 w-5" />
              Extracted Event
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            {extracting ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : emailFull ? (
              <ExtractedEventDisplay
                eventJson={extractedEvent}
                extractionError={extractionError}
                emailFrom={emailFull.from}
                emailSubject={emailFull.subject}
                emailBody={extractPrimaryContent(emailFull.body_html)}
                providerConfigs={providerConfigs}
                onClose={() => setEventDialogOpen(false)}
              />
            ) : null}
          </div>
        </DialogContent>
      </Dialog>

      {/* Extract reminder dialog */}
      <Dialog open={reminderDialogOpen} onOpenChange={setReminderDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <ListTodo className="h-5 w-5" />
              Create Reminder
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            {extractingReminder ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : emailFull ? (
              <ExtractedReminderDisplay
                reminderJson={extractedReminder}
                emailFrom={emailFull.from}
                emailSubject={emailFull.subject}
                onClose={() => setReminderDialogOpen(false)}
              />
            ) : null}
          </div>
        </DialogContent>
      </Dialog>

      {/* Compose dialog */}
      {emailFull && (
        <Compose
          open={composeOpen}
          onClose={() => setComposeOpen(false)}
          account={account}
          mode={composeMode}
          originalEmail={{
            from: emailFull.from,
            to: emailFull.to,
            cc: emailFull.cc,
            subject: emailFull.subject,
            body_text: emailFull.body_html
              ? new DOMParser().parseFromString(
                  emailFull.body_html,
                  "text/html",
                ).body.textContent || ""
              : "",
            body_html: emailFull.body_html,
            message_id: emailFull.message_id,
            date: emailFull.date,
          }}
        />
      )}

      {/* Tag management dialog */}
      {emailSummary && (
        <TagDialog
          open={tagDialogOpen}
          onOpenChange={setTagDialogOpen}
          account={account}
          mailbox={mailbox}
          uid={emailSummary.uid}
          tags={emailTags}
          onTagsChange={handleTagsChange}
          emailContext={
            emailFull
              ? {
                  from: emailFull.from,
                  subject: emailFull.subject,
                  bodyText: extractPrimaryContent(emailFull.body_html),
                }
              : undefined
          }
        />
      )}
    </div>
  );
}
