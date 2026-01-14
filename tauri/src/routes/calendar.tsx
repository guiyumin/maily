import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import {
  ArrowLeft,
  Calendar as CalendarIcon,
  Plus,
  Trash2,
  Clock,
  MapPin,
  Sparkles,
  Loader2,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

interface CalendarInfo {
  id: string;
  title: string;
  color: string;
}

interface CalendarEvent {
  id: string;
  title: string;
  start_time: number;
  end_time: number;
  location: string;
  notes: string;
  calendar: string;
  all_day: boolean;
  alarm_minutes_before: number;
}

interface NewEvent {
  title: string;
  start_time: number;
  end_time: number;
  location: string;
  notes: string;
  calendar_id: string;
  all_day: boolean;
  alarm_minutes_before: number;
}

type AuthStatus = "not_determined" | "restricted" | "denied" | "authorized";

export const Route = createFileRoute("/calendar")({
  component: CalendarPage,
});

function CalendarPage() {
  const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null);
  const [calendars, setCalendars] = useState<CalendarInfo[]>([]);
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedDate, setSelectedDate] = useState(new Date());
  // viewMode for future day/week/month view toggle
  const [_viewMode] = useState<"day" | "week">("week");

  // New event dialog
  const [newEventOpen, setNewEventOpen] = useState(false);
  const [newEventTitle, setNewEventTitle] = useState("");
  const [newEventLocation, setNewEventLocation] = useState("");
  const [newEventNotes, setNewEventNotes] = useState("");
  const [newEventCalendar, setNewEventCalendar] = useState("");
  const [newEventAllDay, setNewEventAllDay] = useState(false);
  const [newEventStartDate, setNewEventStartDate] = useState("");
  const [newEventStartTime, setNewEventStartTime] = useState("09:00");
  const [newEventEndDate, setNewEventEndDate] = useState("");
  const [newEventEndTime, setNewEventEndTime] = useState("10:00");
  const [newEventAlarm, setNewEventAlarm] = useState(15);
  const [creating, setCreating] = useState(false);

  // NLP event creation
  const [nlpInput, setNlpInput] = useState("");
  const [nlpProcessing, setNlpProcessing] = useState(false);

  // Check auth status on mount
  useEffect(() => {
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      const status = await invoke<AuthStatus>("calendar_get_auth_status");
      setAuthStatus(status);

      if (status === "authorized") {
        loadCalendarData();
      } else {
        setLoading(false);
      }
    } catch (err) {
      console.error("Failed to check auth status:", err);
      setLoading(false);
    }
  };

  const requestAccess = async () => {
    try {
      await invoke("calendar_request_access");
      const status = await invoke<AuthStatus>("calendar_get_auth_status");
      setAuthStatus(status);

      if (status === "authorized") {
        loadCalendarData();
      }
    } catch (err) {
      toast.error("Calendar access denied");
    }
  };

  const loadCalendarData = async () => {
    setLoading(true);
    try {
      // Load calendars
      const cals = await invoke<CalendarInfo[]>("calendar_list_calendars");
      setCalendars(cals);
      if (cals.length > 0 && !newEventCalendar) {
        setNewEventCalendar(cals[0].id);
      }

      // Load events for current week
      await loadEvents();
    } catch (err) {
      console.error("Failed to load calendar data:", err);
      toast.error("Failed to load calendar data");
    } finally {
      setLoading(false);
    }
  };

  const loadEvents = useCallback(async () => {
    // Get start of week (Sunday)
    const start = new Date(selectedDate);
    start.setDate(start.getDate() - start.getDay());
    start.setHours(0, 0, 0, 0);

    // Get end of week (Saturday)
    const end = new Date(start);
    end.setDate(end.getDate() + 7);
    end.setHours(23, 59, 59, 999);

    try {
      const evts = await invoke<CalendarEvent[]>("calendar_list_events", {
        startTimestamp: Math.floor(start.getTime() / 1000),
        endTimestamp: Math.floor(end.getTime() / 1000),
      });
      setEvents(evts);
    } catch (err) {
      console.error("Failed to load events:", err);
    }
  }, [selectedDate]);

  useEffect(() => {
    if (authStatus === "authorized") {
      loadEvents();
    }
  }, [selectedDate, authStatus, loadEvents]);

  const createEvent = async () => {
    if (!newEventTitle) return;

    setCreating(true);
    try {
      const startDate = new Date(`${newEventStartDate}T${newEventStartTime}`);
      const endDate = new Date(`${newEventEndDate}T${newEventEndTime}`);

      const event: NewEvent = {
        title: newEventTitle,
        start_time: Math.floor(startDate.getTime() / 1000),
        end_time: Math.floor(endDate.getTime() / 1000),
        location: newEventLocation,
        notes: newEventNotes,
        calendar_id: newEventCalendar,
        all_day: newEventAllDay,
        alarm_minutes_before: newEventAlarm,
      };

      await invoke("calendar_create_event", { event });
      toast.success("Event created");
      setNewEventOpen(false);
      resetNewEventForm();
      loadEvents();
    } catch (err) {
      toast.error(`Failed to create event: ${err}`);
    } finally {
      setCreating(false);
    }
  };

  const deleteEvent = async (eventId: string) => {
    try {
      await invoke("calendar_delete_event", { eventId });
      toast.success("Event deleted");
      loadEvents();
    } catch (err) {
      toast.error(`Failed to delete event: ${err}`);
    }
  };

  const resetNewEventForm = () => {
    setNewEventTitle("");
    setNewEventLocation("");
    setNewEventNotes("");
    setNewEventAllDay(false);
    const today = new Date().toISOString().split("T")[0];
    setNewEventStartDate(today);
    setNewEventEndDate(today);
    setNewEventStartTime("09:00");
    setNewEventEndTime("10:00");
    setNewEventAlarm(15);
  };

  const openNewEventDialog = () => {
    const today = new Date().toISOString().split("T")[0];
    setNewEventStartDate(today);
    setNewEventEndDate(today);
    setNewEventOpen(true);
  };

  // Natural language event creation
  const createEventFromNLP = async () => {
    if (!nlpInput.trim()) return;

    setNlpProcessing(true);
    try {
      // Use AI to extract event details
      const response = await invoke<{
        success: boolean;
        content: string | null;
        error: string | null;
      }>("extract_event", {
        subject: nlpInput,
        bodyText: "",
      });

      if (!response.success || !response.content) {
        toast.error(response.error || "Failed to parse event");
        return;
      }

      const parsed = JSON.parse(response.content);
      if (parsed.found === false) {
        toast.error("Could not extract event details from input");
        return;
      }

      // Fill in the form with extracted data
      if (parsed.title) setNewEventTitle(parsed.title);
      if (parsed.location) setNewEventLocation(parsed.location);
      if (parsed.description) setNewEventNotes(parsed.description);

      // Parse date/time
      if (parsed.date) {
        const eventDate = new Date(parsed.date);
        if (!isNaN(eventDate.getTime())) {
          setNewEventStartDate(eventDate.toISOString().split("T")[0]);
          setNewEventEndDate(eventDate.toISOString().split("T")[0]);
        }
      }
      if (parsed.time) {
        setNewEventStartTime(parsed.time);
        // Default 1 hour duration
        const [hours, mins] = parsed.time.split(":").map(Number);
        const endHours = (hours + 1) % 24;
        setNewEventEndTime(
          `${endHours.toString().padStart(2, "0")}:${mins.toString().padStart(2, "0")}`
        );
      }

      setNlpInput("");
      toast.success("Event details extracted");
    } catch (err) {
      toast.error(`Failed to parse: ${err}`);
    } finally {
      setNlpProcessing(false);
    }
  };

  const navigateWeek = (direction: number) => {
    const newDate = new Date(selectedDate);
    newDate.setDate(newDate.getDate() + direction * 7);
    setSelectedDate(newDate);
  };

  const goToToday = () => {
    setSelectedDate(new Date());
  };

  // Get week days for display
  const getWeekDays = () => {
    const start = new Date(selectedDate);
    start.setDate(start.getDate() - start.getDay());

    const days = [];
    for (let i = 0; i < 7; i++) {
      const day = new Date(start);
      day.setDate(day.getDate() + i);
      days.push(day);
    }
    return days;
  };

  const weekDays = getWeekDays();

  const formatWeekRange = () => {
    const start = weekDays[0];
    const end = weekDays[6];
    const opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" };
    return `${start.toLocaleDateString("en-US", opts)} - ${end.toLocaleDateString("en-US", opts)}, ${end.getFullYear()}`;
  };

  const getEventsForDay = (day: Date) => {
    const dayStart = new Date(day);
    dayStart.setHours(0, 0, 0, 0);
    const dayEnd = new Date(day);
    dayEnd.setHours(23, 59, 59, 999);

    return events.filter((event) => {
      const eventStart = new Date(event.start_time * 1000);
      return eventStart >= dayStart && eventStart <= dayEnd;
    });
  };

  const formatEventTime = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "2-digit",
      hour12: true,
    });
  };

  const getCalendarColor = (calendarId: string) => {
    const cal = calendars.find((c) => c.id === calendarId);
    return cal?.color || "#7C3AED";
  };

  // Not authorized view
  if (authStatus !== "authorized" && !loading) {
    return (
      <div className="min-h-screen bg-background">
        <header className="sticky top-0 z-10 border-b bg-background/95 backdrop-blur">
          <div className="flex h-14 items-center gap-4 px-6">
            <Link
              to="/"
              className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
            >
              <ArrowLeft className="h-4 w-4" />
              <span>Back</span>
            </Link>
            <Separator orientation="vertical" className="h-6" />
            <h1 className="text-lg font-semibold">Calendar</h1>
          </div>
        </header>

        <main className="mx-auto max-w-md p-6">
          <Card>
            <CardHeader className="text-center">
              <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-muted">
                <CalendarIcon className="h-8 w-8 text-muted-foreground" />
              </div>
              <CardTitle>Calendar Access Required</CardTitle>
              <CardDescription>
                Maily needs access to your calendar to show events and create
                new ones.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {authStatus === "denied" && (
                <div className="flex items-start gap-3 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
                  <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                  <div>
                    <p className="font-medium">Access was denied</p>
                    <p className="text-destructive/80">
                      Please enable calendar access in System Preferences &gt;
                      Privacy &amp; Security &gt; Calendars
                    </p>
                  </div>
                </div>
              )}
              <Button onClick={requestAccess} className="w-full">
                Grant Calendar Access
              </Button>
            </CardContent>
          </Card>
        </main>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-background/95 backdrop-blur">
        <div className="flex h-14 items-center gap-4 px-6">
          <Link
            to="/"
            className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Back</span>
          </Link>
          <Separator orientation="vertical" className="h-6" />
          <h1 className="text-lg font-semibold">Calendar</h1>

          <div className="ml-auto flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={goToToday}>
              Today
            </Button>
            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => navigateWeek(-1)}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="min-w-40 text-center text-sm font-medium">
                {formatWeekRange()}
              </span>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => navigateWeek(1)}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>

            <Dialog open={newEventOpen} onOpenChange={setNewEventOpen}>
              <DialogTrigger asChild>
                <Button onClick={openNewEventDialog}>
                  <Plus className="mr-2 h-4 w-4" />
                  New Event
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-lg">
                <DialogHeader>
                  <DialogTitle>Create Event</DialogTitle>
                  <DialogDescription>
                    Add a new event to your calendar
                  </DialogDescription>
                </DialogHeader>

                {/* NLP Input */}
                <div className="space-y-2">
                  <div className="relative">
                    <Sparkles className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                    <Textarea
                      placeholder="e.g., Meeting tomorrow at 2pm with John at Starbucks to discuss project roadmap"
                      value={nlpInput}
                      onChange={(e) => setNlpInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                          e.preventDefault();
                          createEventFromNLP();
                        }
                      }}
                      className="pl-9 resize-none"
                      rows={6}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-muted-foreground">
                      Press ⌘+Enter to parse
                    </span>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={createEventFromNLP}
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

                <Separator />

                <div className="space-y-4">
                  <div className="grid gap-2">
                    <Label htmlFor="event_title">Title</Label>
                    <Input
                      id="event_title"
                      placeholder="Event title"
                      value={newEventTitle}
                      onChange={(e) => setNewEventTitle(e.target.value)}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div className="grid gap-2">
                      <Label>Start</Label>
                      <div className="flex gap-2">
                        <Input
                          type="date"
                          value={newEventStartDate}
                          onChange={(e) => setNewEventStartDate(e.target.value)}
                        />
                        {!newEventAllDay && (
                          <Input
                            type="time"
                            value={newEventStartTime}
                            onChange={(e) =>
                              setNewEventStartTime(e.target.value)
                            }
                            className="w-28"
                          />
                        )}
                      </div>
                    </div>
                    <div className="grid gap-2">
                      <Label>End</Label>
                      <div className="flex gap-2">
                        <Input
                          type="date"
                          value={newEventEndDate}
                          onChange={(e) => setNewEventEndDate(e.target.value)}
                        />
                        {!newEventAllDay && (
                          <Input
                            type="time"
                            value={newEventEndTime}
                            onChange={(e) => setNewEventEndTime(e.target.value)}
                            className="w-28"
                          />
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <Switch
                      id="all_day"
                      checked={newEventAllDay}
                      onCheckedChange={setNewEventAllDay}
                    />
                    <Label htmlFor="all_day">All day</Label>
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="event_location">Location</Label>
                    <div className="relative">
                      <MapPin className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        id="event_location"
                        placeholder="Add location"
                        value={newEventLocation}
                        onChange={(e) => setNewEventLocation(e.target.value)}
                        className="pl-9"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div className="grid gap-2">
                      <Label>Calendar</Label>
                      <Select
                        value={newEventCalendar}
                        onValueChange={setNewEventCalendar}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Select calendar" />
                        </SelectTrigger>
                        <SelectContent>
                          {calendars.map((cal) => (
                            <SelectItem key={cal.id} value={cal.id}>
                              <div className="flex items-center gap-2">
                                <div
                                  className="h-3 w-3 rounded-full"
                                  style={{ backgroundColor: cal.color }}
                                />
                                {cal.title}
                              </div>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <Label>Reminder</Label>
                      <Select
                        value={newEventAlarm.toString()}
                        onValueChange={(v) => setNewEventAlarm(parseInt(v))}
                      >
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
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="event_notes">Notes</Label>
                    <Textarea
                      id="event_notes"
                      placeholder="Meeting URL, agenda, details..."
                      value={newEventNotes}
                      onChange={(e) => setNewEventNotes(e.target.value)}
                      rows={4}
                      className="resize-none"
                    />
                  </div>
                </div>

                <DialogFooter>
                  <Button
                    variant="outline"
                    onClick={() => setNewEventOpen(false)}
                  >
                    Cancel
                  </Button>
                  <Button
                    onClick={createEvent}
                    disabled={!newEventTitle || creating}
                  >
                    {creating ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : null}
                    Create Event
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        </div>
      </header>

      {/* Week View */}
      <main className="p-6">
        <div className="grid grid-cols-7 gap-4">
          {weekDays.map((day) => {
            const isToday = day.toDateString() === new Date().toDateString();
            const dayEvents = getEventsForDay(day);

            return (
              <div key={day.toISOString()} className="min-h-50">
                <div
                  className={cn("mb-2 text-center", isToday && "text-primary")}
                >
                  <div className="text-xs font-medium text-muted-foreground uppercase">
                    {day.toLocaleDateString("en-US", { weekday: "short" })}
                  </div>
                  <div
                    className={cn(
                      "mt-1 inline-flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold",
                      isToday && "bg-primary text-primary-foreground"
                    )}
                  >
                    {day.getDate()}
                  </div>
                </div>

                <div className="space-y-1">
                  {dayEvents.map((event) => (
                    <div
                      key={event.id}
                      className="group relative rounded-md p-2 text-xs transition-colors hover:bg-muted"
                      style={{
                        borderLeft: `3px solid ${getCalendarColor(event.calendar)}`,
                        backgroundColor: `${getCalendarColor(event.calendar)}10`,
                      }}
                    >
                      <div className="font-medium truncate">{event.title}</div>
                      {!event.all_day && (
                        <div className="text-muted-foreground flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          {formatEventTime(event.start_time)}
                        </div>
                      )}
                      {event.location && (
                        <div className="text-muted-foreground flex items-center gap-1 truncate">
                          <MapPin className="h-3 w-3 shrink-0" />
                          {event.location}
                        </div>
                      )}
                      <button
                        onClick={() => deleteEvent(event.id)}
                        className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded hover:bg-destructive/10 text-destructive"
                      >
                        <Trash2 className="h-3 w-3" />
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>

        {/* Calendars Legend */}
        <Card className="mt-6">
          <CardHeader className="py-3">
            <CardTitle className="text-sm font-medium">Calendars</CardTitle>
          </CardHeader>
          <CardContent className="py-2">
            <div className="flex flex-wrap gap-4">
              {calendars.map((cal) => (
                <div key={cal.id} className="flex items-center gap-2 text-sm">
                  <div
                    className="h-3 w-3 rounded-full"
                    style={{ backgroundColor: cal.color }}
                  />
                  <span>{cal.title}</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
