import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import clsx from "clsx";
import { useStore } from "zustand";
import {
  EventDetailContentRenderer,
  EventDetailDialogRenderer,
  CalendarSidebarRenderProps,
  CalendarType,
  CalendarEvent,
  CalendarPlugin,
  CalendarView,
  CalendarCallbacks,
  SidebarConfig,
  ViewType,
  CalendarApp,
} from "../types";
import { ThemeConfig } from "../types/calendarTypes";
import { ViewSwitcherMode } from "../components/common/ViewHeader";
import DefaultCalendarSidebar from "../components/sidebar/DefaultCalendarSidebar";
import DefaultEventDetailDialog from "../components/common/DefaultEventDetailDialog";
import CalendarHeader from "../components/common/CalendarHeader";
import { CreateCalendarDialog } from "../components/common/CreateCalendarDialog";
import SearchDialog from "../components/common/SearchDialog";
import { CalendarSearchProps, CalendarSearchEvent } from "../types/search";
import { normalizeCssWidth } from "../utils/styleUtils";
import { LocaleProvider } from "../locale/LocaleProvider";
import { useLocale } from "../locale/useLocale";
import { LocaleMessages, LocaleCode, Locale } from "../locale/types";
import { getCalendarColorsForHex } from "./calendarRegistry";
import { generateUniKey } from "../utils/helpers";
import { temporalToDate } from "../utils/temporal";
import { createEvent } from "../utils/eventHelpers";
import {
  createCalendarStore,
  CalendarStoreApi,
  CalendarStoreSingleton,
} from "./calendarStore";

const DEFAULT_SIDEBAR_WIDTH = "240px";

const COLORS = [
  "#ea426b",
  "#f19a38",
  "#f7cf46",
  "#83d754",
  "#51aaf2",
  "#b672d0",
  "#957e5e",
];

/**
 * MailyCalendar Props - simplified API
 * Users can pass all configuration as props directly
 */
export interface MailyCalendarProps {
  /** Calendar events to display */
  events?: CalendarEvent[];
  /** Calendar definitions (for multi-calendar support) */
  calendars?: CalendarType[];
  /** Default calendar ID for new events */
  defaultCalendar?: string;
  /** Views to enable */
  views?: CalendarView[];
  /** Plugins to use */
  plugins?: CalendarPlugin[];
  /** Default view to show */
  defaultView?: ViewType;
  /** Initial date to display */
  initialDate?: Date;
  /** Locale for i18n (e.g., "en-US", "zh-CN", "ko-KR") */
  locale?: string | Locale;
  /** Theme configuration */
  theme?: ThemeConfig;
  /** View switcher mode */
  switcherMode?: ViewSwitcherMode;
  /** Sidebar configuration */
  sidebar?: boolean | SidebarConfig;
  /** Whether to use event detail dialog (vs panel) */
  useEventDetailDialog?: boolean;

  // Event callbacks
  /** Called when an event is created */
  onEventCreate?: (event: CalendarEvent) => void | Promise<void>;
  /** Called when an event is updated */
  onEventUpdate?: (event: CalendarEvent) => void | Promise<void>;
  /** Called when an event is deleted */
  onEventDelete?: (eventId: string) => void | Promise<void>;
  /** Called when the view changes */
  onViewChange?: (view: ViewType) => void | Promise<void>;
  /** Called when the date changes */
  onDateChange?: (date: Date) => void | Promise<void>;
  /** Called when the visible month changes */
  onVisibleMonthChange?: (date: Date) => void | Promise<void>;
  /** Called when a calendar is created */
  onCalendarCreate?: (calendar: CalendarType) => void | Promise<void>;
  /** Called when a calendar is updated */
  onCalendarUpdate?: (calendar: CalendarType) => void | Promise<void>;
  /** Called when a calendar is deleted */
  onCalendarDelete?: (calendarId: string) => void | Promise<void>;
  /** Called when calendars are merged */
  onCalendarMerge?: (
    sourceId: string,
    targetId: string,
  ) => void | Promise<void>;

  // UI customization
  className?: string;
  /** Custom event detail content component */
  customDetailPanelContent?: EventDetailContentRenderer;
  /** Custom event detail dialog component */
  customEventDetailDialog?: EventDetailDialogRenderer;
  /** Additional metadata */
  meta?: Record<string, any>;
  /** Custom localization messages */
  customMessages?: LocaleMessages;
  /** Search configuration */
  search?: CalendarSearchProps;
}

// Internal helper to create CalendarApp interface from store
function createAppFromStore(store: CalendarStoreApi): CalendarApp {
  const getState = () => store.getState();

  const stateProxy = {
    get currentView() {
      return getState().currentView;
    },
    get currentDate() {
      return getState().currentDate;
    },
    get calendarEvents() {
      return getState().calendarEvents;
    },
    get plugins() {
      return getState()._plugins;
    },
    get views() {
      return getState()._views;
    },
    get switcherMode() {
      return getState().switcherMode;
    },
    get sidebar() {
      return getState()._sidebarConfig;
    },
    get locale() {
      return getState().locale;
    },
    get highlightedEventId() {
      return getState().highlightedEventId;
    },
  };

  return {
    state: stateProxy,
    changeView: (view: ViewType) => getState().changeView(view),
    getCurrentView: () => getState().getCurrentView(),
    setCurrentDate: (date: Date) => getState().setCurrentDate(date),
    getCurrentDate: () => getState().getCurrentDate(),
    goToToday: () => getState().goToToday(),
    goToPrevious: () => getState().goToPrevious(),
    goToNext: () => getState().goToNext(),
    selectDate: (date: Date) => getState().selectDate(date),
    setVisibleMonth: (date: Date) => getState().setVisibleMonth(date),
    getVisibleMonth: () => getState().getVisibleMonth(),
    addEvent: (event: CalendarEvent) => getState().addEvent(event),
    updateEvent: (
      id: string,
      event: Partial<CalendarEvent>,
      isPending?: boolean,
    ) => getState().updateEvent(id, event, isPending),
    deleteEvent: (id: string) => getState().deleteEvent(id),
    getEvents: () => getState().getEvents(),
    getAllEvents: () => getState().getAllEvents(),
    highlightEvent: (eventId: string | null) =>
      getState().highlightEvent(eventId),
    getCalendars: () => getState().getCalendars(),
    reorderCalendars: (fromIndex: number, toIndex: number) =>
      getState().reorderCalendars(fromIndex, toIndex),
    setCalendarVisibility: (calendarId: string, visible: boolean) =>
      getState().setCalendarVisibility(calendarId, visible),
    setAllCalendarsVisibility: (visible: boolean) =>
      getState().setAllCalendarsVisibility(visible),
    updateCalendar: (id: string, updates: Partial<CalendarType>) =>
      getState().updateCalendar(id, updates),
    createCalendar: (calendar: CalendarType) =>
      getState().createCalendar(calendar),
    deleteCalendar: (id: string) => getState().deleteCalendar(id),
    mergeCalendars: (sourceId: string, targetId: string) =>
      getState().mergeCalendars(sourceId, targetId),
    getPlugin: <T = unknown,>(name: string) => getState().getPlugin<T>(name),
    hasPlugin: (name: string) => getState().hasPlugin(name),
    render: () => React.createElement("div"),
    getSidebarConfig: () => getState().getSidebarConfig(),
    triggerRender: () => getState().triggerRender(),
    getCalendarRegistry: () => getState().getCalendarRegistry(),
    getUseEventDetailDialog: () => getState().getUseEventDetailDialog(),
    setTheme: (mode) => getState().setTheme(mode),
    getTheme: () => getState().getTheme(),
    subscribeThemeChange: (callback) =>
      getState().subscribeThemeChange(callback),
    unsubscribeThemeChange: (callback) =>
      getState().unsubscribeThemeChange(callback),
  };
}

const CalendarInternalLocaleProvider: React.FC<{
  locale: LocaleCode | Locale;
  messages?: LocaleMessages;
  children: React.ReactNode;
}> = ({ locale, messages, children }) => {
  const context = useLocale();

  if (!context.isDefault) {
    return <>{children}</>;
  }

  return (
    <LocaleProvider locale={locale} messages={messages}>
      {children}
    </LocaleProvider>
  );
};

interface CalendarLayoutProps {
  store: CalendarStoreApi;
  app: CalendarApp;
  className?: string;
  customDetailPanelContent?: EventDetailContentRenderer;
  customEventDetailDialog?: EventDetailDialogRenderer;
  meta?: Record<string, any>;
  search?: CalendarSearchProps;
}

const CalendarLayout = ({
  store,
  app,
  className,
  customDetailPanelContent,
  customEventDetailDialog,
  meta,
  search: searchConfig,
}: CalendarLayoutProps) => {
  // Subscribe to view and date changes for re-renders
  useStore(store, (s) => s.currentView);
  useStore(store, (s) => s.currentDate);
  useStore(store, (s) => s.visibleMonth);
  const currentViewObj = app.getCurrentView();
  const ViewComponent = currentViewObj.component;
  const sidebarConfig = app.getSidebarConfig();
  const sidebarEnabled = sidebarConfig?.enabled ?? false;
  const [sidebarVersion, setSidebarVersion] = useState(0);
  const [isCollapsed, setIsCollapsed] = useState(
    sidebarConfig?.initialCollapsed ?? false,
  );
  const { t } = useLocale();

  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingCalendarId, setEditingCalendarId] = useState<string | null>(
    null,
  );
  const [searchKeyword, setSearchKeyword] = useState("");
  const [isSearchOpen, setIsSearchOpen] = useState(false);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchResults, setSearchResults] = useState<CalendarSearchEvent[]>([]);
  const [pendingNewEvent, setPendingNewEvent] = useState<CalendarEvent | null>(
    null,
  );

  // Keyboard shortcut for search (⌘K / Ctrl+K)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setIsSearchOpen(true);
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Search Logic
  useEffect(() => {
    if (!searchKeyword.trim()) {
      setSearchResults([]);
      app.highlightEvent(null);
      return;
    }

    const debounceDelay = searchConfig?.debounceDelay ?? 300;

    const performSearch = async () => {
      setSearchLoading(true);
      try {
        let results: CalendarSearchEvent[] = [];

        if (searchConfig?.customSearch) {
          const currentEvents = app.getEvents().map((e) => ({
            ...e,
            color:
              app.getCalendarRegistry().get(e.calendarId || "")?.colors
                .lineColor ||
              app.getCalendarRegistry().resolveColors().lineColor,
          }));
          results = searchConfig.customSearch({
            keyword: searchKeyword,
            events: currentEvents,
          });
        } else if (searchConfig?.onSearch) {
          results = await searchConfig.onSearch(searchKeyword);
        } else {
          const keywordLower = searchKeyword.toLowerCase();
          results = app
            .getEvents()
            .filter(
              (e) =>
                e.title.toLowerCase().includes(keywordLower) ||
                (e.description &&
                  e.description.toLowerCase().includes(keywordLower)),
            )
            .map((e) => ({
              ...e,
              color:
                app.getCalendarRegistry().get(e.calendarId || "")?.colors
                  .lineColor ||
                app.getCalendarRegistry().resolveColors().lineColor,
            }));
        }

        setSearchResults(results);
        searchConfig?.onSearchStateChange?.({
          keyword: searchKeyword,
          loading: false,
          results,
        });
      } catch (error) {
        console.error("Search failed", error);
        setSearchResults([]);
      } finally {
        setSearchLoading(false);
      }
    };

    const timer = setTimeout(performSearch, debounceDelay);
    return () => clearTimeout(timer);
  }, [searchKeyword, searchConfig, app]);

  useEffect(() => {
    if (!isSearchOpen) {
      app.highlightEvent(null);
    }
  }, [isSearchOpen, app]);

  const handleSearchResultClick = (event: CalendarSearchEvent) => {
    let date: Date;
    if (event.start instanceof Date) {
      date = event.start;
    } else if (typeof event.start === "string") {
      date = new Date(event.start);
    } else {
      date = temporalToDate(event.start as any);
    }
    app.setCurrentDate(date);
    app.highlightEvent(event.id);
  };

  useEffect(() => {
    setIsCollapsed(sidebarConfig?.initialCollapsed ?? false);
  }, [sidebarConfig?.initialCollapsed]);

  const refreshSidebar = useCallback(() => {
    setSidebarVersion((prev) => prev + 1);
  }, []);

  const calendarVersion = useStore(store, (s) => s._calendarVersion);
  const calendars = useMemo(
    () => app.getCalendars(),
    [app, sidebarVersion, calendarVersion],
  );

  const handleToggleCalendarVisibility = useCallback(
    (calendarId: string, visible: boolean) => {
      app.setCalendarVisibility(calendarId, visible);
      refreshSidebar();
    },
    [app, refreshSidebar],
  );

  const handleToggleAllCalendars = useCallback(
    (visible: boolean) => {
      app.setAllCalendarsVisibility(visible);
      refreshSidebar();
    },
    [app, refreshSidebar],
  );

  // Used by sidebar for creating new calendars
  const _handleCreateCalendar = useCallback(() => {
    const createMode = sidebarConfig.createCalendarMode || "inline";

    if (createMode === "modal") {
      setShowCreateDialog(true);
      return;
    }

    const randomColor = COLORS[Math.floor(Math.random() * COLORS.length)];
    const { colors, darkColors } = getCalendarColorsForHex(randomColor);
    const newId = generateUniKey();

    const newCalendar: CalendarType = {
      id: newId,
      name: t("untitled"),
      colors,
      darkColors,
      isVisible: true,
      isDefault: false,
    };

    app.createCalendar(newCalendar);
    setEditingCalendarId(newId);
    refreshSidebar();
  }, [app, sidebarConfig.createCalendarMode, t, refreshSidebar]);
  void _handleCreateCalendar; // Suppress unused warning - available for sidebar

  const handleCreateEvent = useCallback(() => {
    const currentDate = app.getCurrentDate();
    const now = new Date();
    const startHour =
      now.getMinutes() > 30 ? now.getHours() + 1 : now.getHours();
    const startDate = new Date(currentDate);
    startDate.setHours(startHour, 0, 0, 0);

    const endDate = new Date(startDate);
    endDate.setHours(startHour + 1);

    const calendars = app.getCalendars();
    const defaultCalendar = calendars.find((c) => c.isDefault) || calendars[0];

    const newEvent = createEvent({
      id: generateUniKey(),
      title: "",
      start: startDate,
      end: endDate,
      calendarId: defaultCalendar?.id,
    });

    setPendingNewEvent(newEvent);
  }, [app]);

  const handleSaveNewEvent = useCallback(
    (calendarEvent: CalendarEvent) => {
      app.addEvent(calendarEvent);
      setPendingNewEvent(null);
    },
    [app],
  );

  const handleCloseNewEventDialog = useCallback(() => {
    setPendingNewEvent(null);
  }, []);

  const calendarRef = useRef<HTMLDivElement>(null!);

  const effectiveEventDetailDialog: EventDetailDialogRenderer | undefined =
    customEventDetailDialog ||
    (app.getUseEventDetailDialog() ? DefaultEventDetailDialog : undefined);

  const viewProps = {
    app: app,
    config: currentViewObj.config || {},
    customDetailPanelContent,
    customEventDetailDialog: effectiveEventDetailDialog,
    switcherMode: app.state.switcherMode,
    calendarRef,
    meta,
  };

  const sidebarProps: CalendarSidebarRenderProps = {
    app: app,
    calendars,
    toggleCalendarVisibility: handleToggleCalendarVisibility,
    toggleAll: handleToggleAllCalendars,
    isCollapsed,
    setCollapsed: setIsCollapsed,
    renderCalendarContextMenu: sidebarConfig.renderCalendarContextMenu,
    createCalendarMode: sidebarConfig.createCalendarMode,
    renderCreateCalendarDialog: sidebarConfig.renderCreateCalendarDialog,
    editingCalendarId,
    setEditingCalendarId,
  };

  const renderSidebarContent = () => {
    if (!sidebarEnabled) return null;
    if (sidebarConfig.render) {
      return sidebarConfig.render(sidebarProps);
    }
    return <DefaultCalendarSidebar {...sidebarProps} />;
  };

  const sidebarWidth = normalizeCssWidth(
    sidebarConfig?.width,
    DEFAULT_SIDEBAR_WIDTH,
  );
  const miniSidebarWidth = "50px";

  return (
    <div
      className={clsx(
        "maily-calendar relative flex flex-row h-full overflow-hidden",
        className,
      )}
    >
      {sidebarEnabled && (
        <aside
          className="absolute top-0 bottom-0 left-0 z-0 h-full"
          style={{ width: sidebarWidth }}
        >
          {renderSidebarContent()}
        </aside>
      )}

      <div
        className={clsx(
          "flex flex-col flex-1 h-full overflow-hidden relative z-10 bg-white dark:bg-gray-900 transition-all duration-250 ease-in-out border-l",
          isCollapsed
            ? "border-gray-200 dark:border-gray-700 shadow-xl"
            : "border-transparent",
        )}
        style={{
          marginLeft: sidebarEnabled
            ? isCollapsed
              ? miniSidebarWidth
              : sidebarWidth
            : 0,
        }}
      >
        <CalendarHeader
          calendar={app}
          switcherMode={app.state.switcherMode}
          onCreateEvent={handleCreateEvent}
          onSearchClick={() => setIsSearchOpen(true)}
        />

        <div className="flex-1 overflow-hidden relative" ref={calendarRef}>
          <div className="calendar-renderer h-full relative">
            <div className="h-full overflow-hidden">
              <ViewComponent {...viewProps} />
            </div>
          </div>
        </div>
      </div>

      <SearchDialog
        isOpen={isSearchOpen}
        onClose={() => {
          setIsSearchOpen(false);
          setSearchKeyword("");
          app.highlightEvent(null);
        }}
        loading={searchLoading}
        results={searchResults}
        keyword={searchKeyword}
        onKeywordChange={setSearchKeyword}
        onResultClick={handleSearchResultClick}
        emptyText={searchConfig?.emptyText}
      />

      {showCreateDialog &&
        (sidebarConfig.renderCreateCalendarDialog ? (
          sidebarConfig.renderCreateCalendarDialog({
            onClose: () => setShowCreateDialog(false),
            onCreate: (newCalendar) => {
              app.createCalendar(newCalendar);
              setShowCreateDialog(false);
              refreshSidebar();
            },
          })
        ) : (
          <CreateCalendarDialog
            onClose={() => setShowCreateDialog(false)}
            onCreate={(newCalendar) => {
              app.createCalendar(newCalendar);
              setShowCreateDialog(false);
              refreshSidebar();
            }}
          />
        ))}

      {pendingNewEvent && (
        <DefaultEventDetailDialog
          calendarEvent={pendingNewEvent}
          isOpen={true}
          isAllDay={pendingNewEvent.allDay ?? false}
          onEventUpdate={handleSaveNewEvent}
          onEventDelete={() => setPendingNewEvent(null)}
          onClose={handleCloseNewEventDialog}
          app={app}
        />
      )}
    </div>
  );
};

/**
 * MailyCalendar - A full-featured calendar component
 *
 * Simple usage:
 * ```tsx
 * <MailyCalendar
 *   events={events}
 *   calendars={calendars}
 *   locale="zh-CN"
 *   onEventCreate={(event) => saveEvent(event)}
 *   onEventUpdate={(event) => updateEvent(event)}
 *   onEventDelete={(id) => deleteEvent(id)}
 * />
 * ```
 */
export const MailyCalendar: React.FC<MailyCalendarProps> = (props) => {
  console.log("[MailyCalendar] Received locale prop:", props.locale);

  const {
    events = [],
    calendars = [],
    defaultCalendar,
    views = [],
    plugins = [],
    defaultView = ViewType.WEEK,
    initialDate,
    locale = "en-US",
    theme,
    switcherMode,
    sidebar,
    useEventDetailDialog,
    onEventCreate,
    onEventUpdate,
    onEventDelete,
    onViewChange,
    onDateChange,
    onVisibleMonthChange,
    onCalendarCreate,
    onCalendarUpdate,
    onCalendarDelete,
    onCalendarMerge,
    className,
    customDetailPanelContent,
    customEventDetailDialog,
    meta,
    customMessages,
    search,
  } = props;

  // Build callbacks object
  const callbacks: CalendarCallbacks = useMemo(
    () => ({
      onEventCreate,
      onEventUpdate,
      onEventDelete,
      onViewChange,
      onDateChange,
      onVisibleMonthChange,
      onCalendarCreate,
      onCalendarUpdate,
      onCalendarDelete,
      onCalendarMerge,
    }),
    [
      onEventCreate,
      onEventUpdate,
      onEventDelete,
      onViewChange,
      onDateChange,
      onVisibleMonthChange,
      onCalendarCreate,
      onCalendarUpdate,
      onCalendarDelete,
      onCalendarMerge,
    ],
  );

  // Create store once
  const store = useMemo(() => {
    const newStore = createCalendarStore({
      views,
      plugins,
      calendarEvents: events,
      calendars,
      defaultCalendar,
      defaultView,
      initialDate,
      locale,
      theme,
      switcherMode,
      useSidebar: sidebar,
      useEventDetailDialog,
      callbacks,
    });
    CalendarStoreSingleton.set(newStore);
    return newStore;
  }, []); // Empty deps - store created once

  // Create app interface
  const app = useMemo(() => createAppFromStore(store), [store]);

  // Track previous values for sync
  const prevEventsRef = useRef<CalendarEvent[] | undefined>(undefined);
  const prevCalendarsRef = useRef<CalendarType[] | undefined>(undefined);

  // Sync events when they change
  useEffect(() => {
    if (events && events !== prevEventsRef.current) {
      prevEventsRef.current = events;
      store.getState().setEvents(events);
    }
  }, [events, store]);

  // Sync calendars when they change
  useEffect(() => {
    if (calendars && calendars !== prevCalendarsRef.current) {
      prevCalendarsRef.current = calendars;
      store.getState().setCalendars(calendars);
    }
  }, [calendars, store]);

  // Sync locale when it changes
  useEffect(() => {
    if (locale) {
      store.getState().setLocale(locale);
    }
  }, [locale, store]);

  // Subscribe to locale for re-renders
  const currentLocale = useStore(store, (s) => s.locale);
  console.log("[MailyCalendar] Store locale:", currentLocale);

  return (
    <CalendarInternalLocaleProvider
      locale={currentLocale}
      messages={customMessages}
    >
      <CalendarLayout
        store={store}
        app={app}
        className={className}
        customDetailPanelContent={customDetailPanelContent}
        customEventDetailDialog={customEventDetailDialog}
        meta={meta}
        search={search}
      />
    </CalendarInternalLocaleProvider>
  );
};

export default MailyCalendar;
