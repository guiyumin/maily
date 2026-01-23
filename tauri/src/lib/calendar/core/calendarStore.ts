/**
 * Zustand store for calendar state management
 * Replaces the manual forceUpdate pattern with selective subscriptions
 */
import { create, StoreApi } from "zustand";
import { subscribeWithSelector } from "zustand/middleware";
import {
  CalendarAppConfig,
  CalendarPlugin,
  CalendarView,
  ViewType,
  CalendarCallbacks,
  SidebarConfig,
  CalendarType,
} from "../types";
import { CalendarEvent } from "../types";
import {
  CalendarRegistry,
  setDefaultCalendarRegistry,
} from "./calendarRegistry";
import { normalizeCssWidth } from "../utils/styleUtils";
import { ThemeMode } from "../types/calendarTypes";
import { isValidLocale } from "../locale/utils";
import { Locale } from "../locale/types";
import { ViewSwitcherMode } from "../components/common/ViewHeader";

const DEFAULT_SIDEBAR_WIDTH = "240px";

// ============ Singleton Store ============

export class CalendarStoreSingleton {
  private static instance: CalendarStoreApi | null = null;

  static set(store: CalendarStoreApi): void {
    this.instance = store;
  }

  static get(): CalendarStoreApi {
    if (!this.instance) {
      throw new Error(
        "Calendar store not initialized. Make sure useCalendarApp is called first.",
      );
    }
    return this.instance;
  }
}

// ============ Helper Functions ============

const resolveSidebarConfig = (
  input?: boolean | SidebarConfig,
): SidebarConfig => {
  if (!input) {
    return {
      enabled: false,
      width: DEFAULT_SIDEBAR_WIDTH,
      initialCollapsed: false,
    };
  }

  if (input === true) {
    return {
      enabled: true,
      width: DEFAULT_SIDEBAR_WIDTH,
      initialCollapsed: false,
    };
  }

  const {
    enabled = true,
    width,
    initialCollapsed = false,
    render,
    createCalendarMode,
    renderCalendarContextMenu,
    renderCreateCalendarDialog,
  } = input;

  return {
    enabled,
    width: normalizeCssWidth(width, DEFAULT_SIDEBAR_WIDTH),
    initialCollapsed,
    render,
    createCalendarMode,
    renderCalendarContextMenu,
    renderCreateCalendarDialog,
  };
};

const resolveLocale = (locale?: string | Locale): string | Locale => {
  if (!locale) {
    return "en-US";
  }

  if (typeof locale === "string") {
    return isValidLocale(locale) ? locale : "en-US";
  }

  if (locale && typeof locale === "object" && !isValidLocale(locale.code)) {
    return { ...locale, code: "en-US" };
  }

  return locale;
};

// ============ Store Types ============

export interface CalendarStoreState {
  // Reactive state (triggers re-renders when changed)
  currentView: ViewType;
  currentDate: Date;
  calendarEvents: CalendarEvent[];
  highlightedEventId: string | null;
  visibleMonth: Date;

  // Theme state
  theme: ThemeMode;
  effectiveTheme: "light" | "dark";
  _systemTheme: "light" | "dark";

  // Semi-reactive (needed for some components)
  switcherMode: ViewSwitcherMode;
  locale: string | Locale;

  // Internal state (accessed via getters, not subscribed directly)
  _views: Map<ViewType, CalendarView>;
  _plugins: Map<string, CalendarPlugin>;
  _calendarRegistry: CalendarRegistry;
  _callbacks: CalendarCallbacks;
  _sidebarConfig: SidebarConfig;
  _useEventDetailDialog: boolean;
  _themeChangeListeners: Set<(theme: ThemeMode) => void>;
  _systemThemeCleanup: (() => void) | null;

  // Version counter to force updates when calendar registry changes
  _calendarVersion: number;
}

export interface CalendarStoreActions {
  // View management
  changeView: (view: ViewType) => void;
  getCurrentView: () => CalendarView;

  // Date management
  setCurrentDate: (date: Date) => void;
  getCurrentDate: () => Date;
  goToToday: () => void;
  goToPrevious: () => void;
  goToNext: () => void;
  selectDate: (date: Date) => void;
  setVisibleMonth: (date: Date) => void;
  getVisibleMonth: () => Date;

  // Event management
  addEvent: (event: CalendarEvent) => void;
  updateEvent: (
    id: string,
    eventUpdate: Partial<CalendarEvent>,
    isPending?: boolean,
  ) => void;
  deleteEvent: (id: string) => void;
  setEvents: (events: CalendarEvent[]) => void;
  getEvents: () => CalendarEvent[];
  getAllEvents: () => CalendarEvent[];
  highlightEvent: (eventId: string | null) => void;

  // Calendar management
  getCalendars: () => CalendarType[];
  setCalendars: (calendars: CalendarType[]) => void;
  createCalendar: (calendar: CalendarType) => void;
  updateCalendar: (id: string, updates: Partial<CalendarType>) => void;
  deleteCalendar: (id: string) => void;
  mergeCalendars: (sourceId: string, targetId: string) => void;
  setCalendarVisibility: (calendarId: string, visible: boolean) => void;
  setAllCalendarsVisibility: (visible: boolean) => void;
  reorderCalendars: (fromIndex: number, toIndex: number) => void;

  // Theme management
  setTheme: (mode: ThemeMode) => void;
  getTheme: () => ThemeMode;
  subscribeThemeChange: (callback: (theme: ThemeMode) => void) => () => void;
  unsubscribeThemeChange: (callback: (theme: ThemeMode) => void) => void;

  // Locale management
  setLocale: (locale: string | Locale) => void;

  // Plugin management
  getPlugin: <T = unknown>(name: string) => T | undefined;
  hasPlugin: (name: string) => boolean;
  getPluginConfig: (pluginName: string) => Record<string, unknown>;
  updatePluginConfig: (
    pluginName: string,
    config: Record<string, unknown>,
  ) => void;

  // View config
  getViewConfig: (viewType: ViewType) => Record<string, unknown>;

  // Getters for internal state
  getCalendarRegistry: () => CalendarRegistry;
  getSidebarConfig: () => SidebarConfig;
  getUseEventDetailDialog: () => boolean;

  // Legacy compatibility
  triggerRender: () => void;
}

export type CalendarStore = CalendarStoreState & CalendarStoreActions;

// ============ Theme Helpers ============

const getSystemTheme = (): "light" | "dark" => {
  if (typeof window === "undefined") return "light";
  if (window.matchMedia?.("(prefers-color-scheme: dark)").matches)
    return "dark";
  return "light";
};

const computeEffectiveTheme = (
  theme: ThemeMode,
  systemTheme: "light" | "dark",
): "light" | "dark" => {
  return theme === "auto" ? systemTheme : theme;
};

const applyThemeToDocument = (
  theme: ThemeMode,
  effectiveTheme: "light" | "dark",
) => {
  if (typeof document === "undefined") return;

  const root = document.documentElement;
  root.classList.remove("light", "dark");
  root.classList.add(effectiveTheme);

  if (theme === "auto") {
    root.removeAttribute("data-maily-theme-override");
  } else {
    root.setAttribute("data-maily-theme-override", effectiveTheme);
  }
  root.setAttribute("data-theme", effectiveTheme);
};

// ============ Store Factory ============

export type CalendarStoreApi = StoreApi<CalendarStore>;

export function createCalendarStore(
  config: CalendarAppConfig,
): CalendarStoreApi {
  // Initialize views map
  const viewsMap = new Map<ViewType, CalendarView>();
  config.views.forEach((view) => {
    viewsMap.set(view.type, view);
  });

  // Initialize calendar registry
  const calendarRegistry = new CalendarRegistry(
    config.calendars,
    config.defaultCalendar,
    config.theme?.mode || "light",
  );
  setDefaultCalendarRegistry(calendarRegistry);

  // Resolve sidebar config
  const sidebarConfig = resolveSidebarConfig(config.useSidebar);

  // Initial date
  const initialDate = config.initialDate || new Date();
  const initialVisibleMonth = new Date(
    initialDate.getFullYear(),
    initialDate.getMonth(),
    1,
  );

  // Initial theme
  const initialTheme = config.theme?.mode || "light";
  const initialSystemTheme = getSystemTheme();
  const initialEffectiveTheme = computeEffectiveTheme(
    initialTheme,
    initialSystemTheme,
  );

  return create<CalendarStore>()(
    subscribeWithSelector((set, get) => {
      // Helper to increment calendar version (forces re-render for calendar-dependent components)
      const bumpCalendarVersion = () => {
        set((state) => ({ _calendarVersion: state._calendarVersion + 1 }));
      };

      const store: CalendarStore = {
        // ============ Initial State ============
        currentView: config.defaultView || ViewType.WEEK,
        currentDate: initialDate,
        calendarEvents: config.calendarEvents || [],
        highlightedEventId: null,
        visibleMonth: initialVisibleMonth,
        switcherMode: config.switcherMode || "buttons",
        locale: resolveLocale(config.locale),

        // Theme state
        theme: initialTheme,
        effectiveTheme: initialEffectiveTheme,
        _systemTheme: initialSystemTheme,

        // Internal state
        _views: viewsMap,
        _plugins: new Map(),
        _calendarRegistry: calendarRegistry,
        _callbacks: config.callbacks || {},
        _sidebarConfig: sidebarConfig,
        _useEventDetailDialog: config.useEventDetailDialog ?? false,
        _themeChangeListeners: new Set(),
        _systemThemeCleanup: null,
        _calendarVersion: 0,

        // ============ View Management ============
        changeView: (view: ViewType) => {
          const state = get();
          if (!state._views.has(view)) {
            throw new Error(`View ${view} is not registered`);
          }
          set({ currentView: view });
          state._callbacks.onViewChange?.(view);
        },

        getCurrentView: () => {
          const state = get();
          const view = state._views.get(state.currentView);
          if (!view) {
            throw new Error(
              `Current view ${state.currentView} is not registered`,
            );
          }
          return view;
        },

        // ============ Date Management ============
        setCurrentDate: (date: Date) => {
          const newDate = new Date(date);
          set({ currentDate: newDate });
          const state = get();
          state._callbacks.onDateChange?.(newDate);
          // Also update visible month
          state.setVisibleMonth(newDate);
        },

        getCurrentDate: () => new Date(get().currentDate),

        goToToday: () => {
          get().setCurrentDate(new Date());
        },

        goToPrevious: () => {
          const state = get();
          const newDate = new Date(state.currentDate);
          switch (state.currentView) {
            case ViewType.DAY:
              newDate.setDate(newDate.getDate() - 1);
              break;
            case ViewType.WEEK:
              newDate.setDate(newDate.getDate() - 7);
              break;
            case ViewType.MONTH:
              newDate.setMonth(newDate.getMonth() - 1);
              break;
          }
          state.setCurrentDate(newDate);
        },

        goToNext: () => {
          const state = get();
          const newDate = new Date(state.currentDate);
          switch (state.currentView) {
            case ViewType.DAY:
              newDate.setDate(newDate.getDate() + 1);
              break;
            case ViewType.WEEK:
              newDate.setDate(newDate.getDate() + 7);
              break;
            case ViewType.MONTH:
              newDate.setMonth(newDate.getMonth() + 1);
              break;
          }
          state.setCurrentDate(newDate);
        },

        selectDate: (date: Date) => {
          const state = get();
          state.setCurrentDate(date);
          state._callbacks.onDateChange?.(new Date(date));
        },

        setVisibleMonth: (date: Date) => {
          const next = new Date(date.getFullYear(), date.getMonth(), 1);
          const state = get();
          if (
            state.visibleMonth.getFullYear() === next.getFullYear() &&
            state.visibleMonth.getMonth() === next.getMonth()
          ) {
            return;
          }
          set({ visibleMonth: next });
          state._callbacks.onVisibleMonthChange?.(new Date(next));
        },

        getVisibleMonth: () => new Date(get().visibleMonth),

        // ============ Event Management ============
        addEvent: (event: CalendarEvent) => {
          set((state) => ({ calendarEvents: [...state.calendarEvents, event] }));
          get()._callbacks.onEventCreate?.(event);
        },

        updateEvent: (
          id: string,
          eventUpdate: Partial<CalendarEvent>,
          isPending?: boolean,
        ) => {
          set((state) => {
            const eventIndex = state.calendarEvents.findIndex((e) => e.id === id);
            if (eventIndex === -1) {
              throw new Error(`Event with id ${id} not found`);
            }
            const updatedEvent = {
              ...state.calendarEvents[eventIndex],
              ...eventUpdate,
            };
            const newEvents = [...state.calendarEvents];
            newEvents[eventIndex] = updatedEvent;
            return { calendarEvents: newEvents };
          });

          if (!isPending) {
            const state = get();
            const updatedEvent = state.calendarEvents.find((e) => e.id === id);
            if (updatedEvent) {
              state._callbacks.onEventUpdate?.(updatedEvent);
            }
          }
        },

        deleteEvent: (id: string) => {
          const state = get();
          const eventIndex = state.calendarEvents.findIndex((e) => e.id === id);
          if (eventIndex === -1) {
            throw new Error(`Event with id ${id} not found`);
          }
          set((s) => ({
            calendarEvents: s.calendarEvents.filter((e) => e.id !== id),
          }));
          state._callbacks.onEventDelete?.(id);
        },

        setEvents: (events: CalendarEvent[]) => {
          set({ calendarEvents: [...events] });
        },

        getEvents: () => {
          const state = get();
          const allEvents = state.calendarEvents;
          const visibleCalendars = new Set(
            state._calendarRegistry
              .getAll()
              .filter((calendar) => calendar.isVisible !== false)
              .map((calendar) => calendar.id),
          );

          return allEvents.filter((event) => {
            if (!event.calendarId) {
              return true;
            }
            if (!state._calendarRegistry.has(event.calendarId)) {
              return true;
            }
            return visibleCalendars.has(event.calendarId);
          });
        },

        getAllEvents: () => [...get().calendarEvents],

        highlightEvent: (eventId: string | null) => {
          set({ highlightedEventId: eventId });
          get()._callbacks.onRender?.();
        },

        // ============ Calendar Management ============
        getCalendars: () => get()._calendarRegistry.getAll(),

        setCalendars: (calendars: CalendarType[]) => {
          get()._calendarRegistry.setAll(calendars);
          bumpCalendarVersion();
        },

        createCalendar: (calendar: CalendarType) => {
          const state = get();
          state._calendarRegistry.register(calendar);
          state._callbacks.onCalendarCreate?.(calendar);
          bumpCalendarVersion();
        },

        updateCalendar: (id: string, updates: Partial<CalendarType>) => {
          const state = get();
          state._calendarRegistry.updateCalendar(id, updates);
          const updatedCalendar = state._calendarRegistry.get(id);
          if (updatedCalendar) {
            state._callbacks.onCalendarUpdate?.(updatedCalendar);
          }
          bumpCalendarVersion();
        },

        deleteCalendar: (id: string) => {
          const state = get();
          state._calendarRegistry.unregister(id);
          state._callbacks.onCalendarDelete?.(id);
          bumpCalendarVersion();
        },

        mergeCalendars: (sourceId: string, targetId: string) => {
          const state = get();

          // Update all events from source calendar to target calendar
          set((s) => ({
            calendarEvents: s.calendarEvents.map((e) =>
              e.calendarId === sourceId ? { ...e, calendarId: targetId } : e,
            ),
          }));

          // Delete source calendar
          state._calendarRegistry.unregister(sourceId);
          state._callbacks.onCalendarMerge?.(sourceId, targetId);
          bumpCalendarVersion();
        },

        setCalendarVisibility: (calendarId: string, visible: boolean) => {
          get()._calendarRegistry.setVisibility(calendarId, visible);
          bumpCalendarVersion();
        },

        setAllCalendarsVisibility: (visible: boolean) => {
          get()._calendarRegistry.setAllVisibility(visible);
          bumpCalendarVersion();
        },

        reorderCalendars: (fromIndex: number, toIndex: number) => {
          get()._calendarRegistry.reorder(fromIndex, toIndex);
          bumpCalendarVersion();
        },

        // ============ Theme Management ============
        setTheme: (mode: ThemeMode) => {
          const state = get();
          const newEffectiveTheme = computeEffectiveTheme(
            mode,
            state._systemTheme,
          );

          state._calendarRegistry.setTheme(mode);
          set({ theme: mode, effectiveTheme: newEffectiveTheme });

          // Apply to document
          applyThemeToDocument(mode, newEffectiveTheme);

          // Notify listeners
          state._themeChangeListeners.forEach((listener) => listener(mode));
          bumpCalendarVersion();
        },

        getTheme: () => get().theme,

        subscribeThemeChange: (callback: (theme: ThemeMode) => void) => {
          const state = get();
          state._themeChangeListeners.add(callback);
          return () => {
            state._themeChangeListeners.delete(callback);
          };
        },

        unsubscribeThemeChange: (callback: (theme: ThemeMode) => void) => {
          get()._themeChangeListeners.delete(callback);
        },

        // ============ Locale Management ============
        setLocale: (locale: string | Locale) => {
          set({ locale: resolveLocale(locale) });
        },

        // ============ Plugin Management ============
        getPlugin: <T = unknown>(name: string): T | undefined => {
          const plugin = get()._plugins.get(name);
          return plugin?.api as T;
        },

        hasPlugin: (name: string) => get()._plugins.has(name),

        getPluginConfig: (pluginName: string) => {
          const plugin = get()._plugins.get(pluginName);
          return plugin?.config || {};
        },

        updatePluginConfig: (
          pluginName: string,
          config: Record<string, unknown>,
        ) => {
          const plugin = get()._plugins.get(pluginName);
          if (plugin) {
            plugin.config = { ...plugin.config, ...config };
          }
        },

        // ============ View Config ============
        getViewConfig: (viewType: ViewType) => {
          const view = get()._views.get(viewType);
          return view?.config || {};
        },

        // ============ Getters ============
        getCalendarRegistry: () => get()._calendarRegistry,
        getSidebarConfig: () => get()._sidebarConfig,
        getUseEventDetailDialog: () => get()._useEventDetailDialog,

        // ============ Legacy Compatibility ============
        triggerRender: () => {
          get()._callbacks.onRender?.();
        },
      };

      // Install plugins after store is created
      config.plugins?.forEach((plugin) => {
        if (!store._plugins.has(plugin.name)) {
          store._plugins.set(plugin.name, plugin);
          // Create a minimal CalendarApp-like interface for plugin installation
          // This maintains backward compatibility with existing plugins
          plugin.install(createPluginAppInterface(get, set));
        }
      });

      // Setup system theme listener for 'auto' mode
      if (typeof window !== "undefined" && window.matchMedia) {
        const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");

        const handleSystemThemeChange = (
          e: MediaQueryListEvent | MediaQueryList,
        ) => {
          const newSystemTheme = e.matches ? "dark" : "light";
          const state = get();

          if (state._systemTheme !== newSystemTheme) {
            const newEffectiveTheme = computeEffectiveTheme(
              state.theme,
              newSystemTheme,
            );
            set({
              _systemTheme: newSystemTheme,
              effectiveTheme: newEffectiveTheme,
            });

            // Apply to document if in auto mode
            if (state.theme === "auto") {
              applyThemeToDocument(state.theme, newEffectiveTheme);
            }
          }
        };

        if (mediaQuery.addEventListener) {
          mediaQuery.addEventListener("change", handleSystemThemeChange);
          store._systemThemeCleanup = () =>
            mediaQuery.removeEventListener("change", handleSystemThemeChange);
        }
      }

      // Apply initial theme to document
      applyThemeToDocument(initialTheme, initialEffectiveTheme);

      return store;
    }),
  );
}

// Helper to create CalendarApp-like interface for plugins
function createPluginAppInterface(
  get: () => CalendarStore,
  _set: (
    partial:
      | Partial<CalendarStoreState>
      | ((state: CalendarStoreState) => Partial<CalendarStoreState>),
  ) => void,
): any {
  return {
    get state() {
      const s = get();
      return {
        currentView: s.currentView,
        currentDate: s.currentDate,
        calendarEvents: s.calendarEvents,
        plugins: s._plugins,
        views: s._views,
        switcherMode: s.switcherMode,
        sidebar: s._sidebarConfig,
        locale: s.locale,
        highlightedEventId: s.highlightedEventId,
      };
    },
    // Use arrow functions to defer get() calls until method is invoked
    changeView: (view: ViewType) => get().changeView(view),
    getCurrentView: () => get().getCurrentView(),
    setCurrentDate: (date: Date) => get().setCurrentDate(date),
    getCurrentDate: () => get().getCurrentDate(),
    goToToday: () => get().goToToday(),
    goToPrevious: () => get().goToPrevious(),
    goToNext: () => get().goToNext(),
    selectDate: (date: Date) => get().selectDate(date),
    addEvent: (event: CalendarEvent) => get().addEvent(event),
    updateEvent: (id: string, event: Partial<CalendarEvent>, isPending?: boolean) =>
      get().updateEvent(id, event, isPending),
    deleteEvent: (id: string) => get().deleteEvent(id),
    getEvents: () => get().getEvents(),
    getAllEvents: () => get().getAllEvents(),
    highlightEvent: (eventId: string | null) => get().highlightEvent(eventId),
    getCalendars: () => get().getCalendars(),
    reorderCalendars: (fromIndex: number, toIndex: number) =>
      get().reorderCalendars(fromIndex, toIndex),
    setCalendarVisibility: (calendarId: string, visible: boolean) =>
      get().setCalendarVisibility(calendarId, visible),
    setAllCalendarsVisibility: (visible: boolean) =>
      get().setAllCalendarsVisibility(visible),
    updateCalendar: (id: string, updates: Partial<CalendarType>) =>
      get().updateCalendar(id, updates),
    createCalendar: (calendar: CalendarType) => get().createCalendar(calendar),
    deleteCalendar: (id: string) => get().deleteCalendar(id),
    mergeCalendars: (sourceId: string, targetId: string) =>
      get().mergeCalendars(sourceId, targetId),
    setVisibleMonth: (date: Date) => get().setVisibleMonth(date),
    getVisibleMonth: () => get().getVisibleMonth(),
    getPlugin: <T = unknown>(name: string) => get().getPlugin<T>(name),
    hasPlugin: (name: string) => get().hasPlugin(name),
    getSidebarConfig: () => get().getSidebarConfig(),
    triggerRender: () => get().triggerRender(),
    getCalendarRegistry: () => get().getCalendarRegistry(),
    getUseEventDetailDialog: () => get().getUseEventDetailDialog(),
    setTheme: (mode: ThemeMode) => get().setTheme(mode),
    getTheme: () => get().getTheme(),
    subscribeThemeChange: (callback: (theme: ThemeMode) => void) =>
      get().subscribeThemeChange(callback),
    unsubscribeThemeChange: (callback: (theme: ThemeMode) => void) =>
      get().unsubscribeThemeChange(callback),
  };
}
