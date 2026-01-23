import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState, useCallback, useMemo } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ArrowLeft,
  Calendar as CalendarIcon,
  Loader2,
  AlertCircle,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  MailyCalendar,
  createMonthView,
  createWeekView,
  createDayView,
  createDragPlugin,
  createEvent,
  ViewType,
  type CalendarEvent as MailyEvent,
  type CalendarType,
  type CalendarColors,
} from "@/lib/calendar";
import { Temporal } from "temporal-polyfill";

// Predefined calendar colors for visual consistency
const CALENDAR_COLORS = [
  "#ea426b", // red/pink
  "#f19a38", // orange
  "#f7cf46", // yellow
  "#83d754", // green
  "#51aaf2", // blue
  "#b672d0", // purple
  "#957e5e", // brown
];

// Generate calendar colors from a hex color
function getCalendarColors(hex: string): {
  colors: CalendarColors;
  darkColors: CalendarColors;
} {
  return {
    colors: {
      eventColor: `${hex}1A`, // 10% opacity for background
      eventSelectedColor: hex,
      lineColor: hex,
      textColor: "#1f2937",
    },
    darkColors: {
      eventColor: `${hex}CC`, // 80% opacity for dark mode
      eventSelectedColor: hex,
      lineColor: hex,
      textColor: "#f9fafb",
    },
  };
}

interface CalendarInfo {
  id: string;
  title: string;
  color: string;
  source_title: string;
  source_type: string;
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

interface Config {
  language?: string;
}

type AuthStatus = "not_determined" | "restricted" | "denied" | "authorized";

export const Route = createFileRoute("/calendar")({
  component: CalendarPage,
});

// Convert Tauri calendar to Maily calendar type
function toMailyCalendar(cal: CalendarInfo, index: number): CalendarType {
  const colorHex = CALENDAR_COLORS[index % CALENDAR_COLORS.length];
  const { colors, darkColors } = getCalendarColors(colorHex);
  return {
    id: cal.id,
    name: cal.title,
    isVisible: true,
    colors,
    darkColors,
    sourceTitle: cal.source_title,
    sourceType: cal.source_type,
  };
}

// Convert Tauri event to Maily event
function toMailyEvent(event: CalendarEvent): MailyEvent {
  return createEvent({
    id: event.id,
    title: event.title,
    start: new Date(event.start_time * 1000),
    end: new Date(event.end_time * 1000),
    calendarId: event.calendar,
    allDay: event.all_day,
    meta: {
      location: event.location,
      notes: event.notes,
      alarmMinutesBefore: event.alarm_minutes_before,
    },
  });
}

// Helper to convert Temporal types to Unix timestamp
function temporalToTimestamp(
  temporal:
    | Temporal.PlainDate
    | Temporal.PlainDateTime
    | Temporal.ZonedDateTime,
): number {
  if (temporal instanceof Temporal.ZonedDateTime) {
    return Math.floor(temporal.epochMilliseconds / 1000);
  } else if (temporal instanceof Temporal.PlainDateTime) {
    const zoned = temporal.toZonedDateTime(Temporal.Now.timeZoneId());
    return Math.floor(zoned.epochMilliseconds / 1000);
  } else {
    const dateTime = temporal.toPlainDateTime({ hour: 0, minute: 0 });
    const zoned = dateTime.toZonedDateTime(Temporal.Now.timeZoneId());
    return Math.floor(zoned.epochMilliseconds / 1000);
  }
}

// Convert Maily event back to Tauri format for creating/updating
function toTauriEvent(event: MailyEvent): NewEvent {
  return {
    title: event.title,
    start_time: temporalToTimestamp(event.start),
    end_time: temporalToTimestamp(event.end),
    location: event.meta?.location || "",
    notes: event.meta?.notes || "",
    calendar_id: event.calendarId || "",
    all_day: event.meta?.allDay || false,
    alarm_minutes_before: event.meta?.alarmMinutesBefore || 15,
  };
}

// Map Go CLI language codes to BCP 47 locale codes
const LOCALE_MAP: Record<string, string> = {
  en: "en-US",
  ko: "ko-KR",
  ja: "ja-JP",
  "zh-Hans": "zh-CN",
  "zh-Hant": "zh-TW",
  zh: "zh-CN",
  es: "es-ES",
  de: "de-DE",
  fr: "fr-FR",
  "pt-BR": "pt-BR",
  pl: "pl-PL",
  nl: "nl-NL",
  it: "it-IT",
  ru: "ru-RU",
};

function CalendarPage() {
  const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null);
  const [calendars, setCalendars] = useState<CalendarInfo[]>([]);
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [configLoaded, setConfigLoaded] = useState(false);
  const [locale, setLocale] = useState<string | null>(null);

  // Load config on mount
  useEffect(() => {
    invoke<Config>("get_config")
      .then((config) => {
        if (config.language) {
          setLocale(LOCALE_MAP[config.language] || config.language);
        } else {
          setLocale(navigator.language || "en-US");
        }
      })
      .catch((err) => {
        console.error("Failed to load config:", err);
        setLocale(navigator.language || "en-US");
      })
      .finally(() => setConfigLoaded(true));

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
      const cals = await invoke<CalendarInfo[]>("calendar_list_calendars");
      setCalendars(cals);
      await loadEvents();
    } catch (err) {
      console.error("Failed to load calendar data:", err);
      toast.error("Failed to load calendar data");
    } finally {
      setLoading(false);
    }
  };

  const loadEvents = useCallback(async () => {
    const start = new Date();
    start.setMonth(start.getMonth() - 1);
    start.setDate(1);
    start.setHours(0, 0, 0, 0);

    const end = new Date();
    end.setMonth(end.getMonth() + 2);
    end.setDate(0);
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
  }, []);

  // Convert to Maily formats
  const mailyCalendars = useMemo(
    () => calendars.map((cal, index) => toMailyCalendar(cal, index)),
    [calendars],
  );
  const mailyEvents = useMemo(() => events.map(toMailyEvent), [events]);

  // Backend search function
  const handleSearch = useCallback(
    async (keyword: string) => {
      const results = await invoke<CalendarEvent[]>("calendar_search_events", {
        keyword,
      });
      return results.map((event) => {
        const calendarIndex = calendars.findIndex((c) => c.id === event.calendar);
        const colorHex =
          CALENDAR_COLORS[calendarIndex >= 0 ? calendarIndex % CALENDAR_COLORS.length : 0];
        const mailyEvent = createEvent({
          id: event.id,
          title: event.title,
          start: new Date(event.start_time * 1000),
          end: new Date(event.end_time * 1000),
          calendarId: event.calendar,
          allDay: event.all_day,
          meta: {
            location: event.location,
            notes: event.notes,
          },
        });
        return { ...mailyEvent, color: colorHex };
      });
    },
    [calendars],
  );

  // Event handlers
  const handleEventCreate = useCallback(
    async (event: MailyEvent) => {
      try {
        const newEvent = toTauriEvent(event);
        await invoke("calendar_create_event", { event: newEvent });
        toast.success("Event created");
        loadEvents();
      } catch (err) {
        toast.error(`Failed to create event: ${err}`);
      }
    },
    [loadEvents],
  );

  const handleEventUpdate = useCallback(
    async (event: MailyEvent) => {
      try {
        const updatedEvent = toTauriEvent(event);
        await invoke("calendar_update_event", {
          eventId: event.id,
          event: updatedEvent,
        });
        toast.success("Event updated");
        loadEvents();
      } catch (err) {
        toast.error(`Failed to update event: ${err}`);
      }
    },
    [loadEvents],
  );

  const handleEventDelete = useCallback(
    async (eventId: string) => {
      try {
        await invoke("calendar_delete_event", { eventId });
        toast.success("Event deleted");
        loadEvents();
      } catch (err) {
        toast.error(`Failed to delete event: ${err}`);
      }
    },
    [loadEvents],
  );

  // Not authorized view
  if (authStatus !== "authorized" && !loading && configLoaded) {
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

  // Loading state
  if (loading || !configLoaded || !locale) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Main calendar view - simplified API!
  return (
    <div className="flex h-screen flex-col bg-background">
      <header className="shrink-0 border-b bg-background/95 backdrop-blur">
        <div className="flex h-12 items-center gap-4 px-4">
          <Link
            to="/"
            className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
            <span className="text-sm">Back</span>
          </Link>
        </div>
      </header>

      <main className="flex-1 overflow-hidden">
        <MailyCalendar
          events={mailyEvents}
          calendars={mailyCalendars}
          views={[createMonthView(), createWeekView(), createDayView()]}
          plugins={[createDragPlugin()]}
          defaultView={ViewType.WEEK}
          locale={locale}
          theme={{ mode: "auto" }}
          sidebar={{ enabled: true, width: 240, initialCollapsed: false }}
          onEventCreate={handleEventCreate}
          onEventUpdate={handleEventUpdate}
          onEventDelete={handleEventDelete}
          onVisibleMonthChange={loadEvents}
          search={{ onSearch: handleSearch }}
          className="maily-calendar"
        />
      </main>
    </div>
  );
}
