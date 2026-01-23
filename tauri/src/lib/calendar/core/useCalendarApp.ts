/**
 * useCalendarApp hook - Zustand-based implementation
 * Maintains backward compatibility with the previous CalendarApp class-based approach
 */
import { useMemo, useEffect, useRef } from 'react';
import { useStore } from 'zustand';
import { createCalendarStore, CalendarStoreApi, CalendarStoreSingleton } from './calendarStore';
import {
  CalendarAppConfig,
  UseCalendarAppReturn,
  CalendarApp,
  CalendarAppState,
  ViewType,
  CalendarType,
} from '../types';
import { CalendarEvent } from '../types';
import React from 'react';

/**
 * Create a CalendarApp-compatible interface from the store
 * This provides backward compatibility for components expecting the CalendarApp interface
 */
function createAppFromStore(store: CalendarStoreApi): CalendarApp {
  const getState = () => store.getState();

  // Create a proxy state object that reads from the store
  const stateProxy: CalendarAppState = {
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

    // View management
    changeView: (view: ViewType) => getState().changeView(view),
    getCurrentView: () => getState().getCurrentView(),

    // Date management
    setCurrentDate: (date: Date) => getState().setCurrentDate(date),
    getCurrentDate: () => getState().getCurrentDate(),
    goToToday: () => getState().goToToday(),
    goToPrevious: () => getState().goToPrevious(),
    goToNext: () => getState().goToNext(),
    selectDate: (date: Date) => getState().selectDate(date),
    setVisibleMonth: (date: Date) => getState().setVisibleMonth(date),
    getVisibleMonth: () => getState().getVisibleMonth(),

    // Event management
    addEvent: (event: CalendarEvent) => getState().addEvent(event),
    updateEvent: (id: string, event: Partial<CalendarEvent>, isPending?: boolean) =>
      getState().updateEvent(id, event, isPending),
    deleteEvent: (id: string) => getState().deleteEvent(id),
    getEvents: () => getState().getEvents(),
    getAllEvents: () => getState().getAllEvents(),
    highlightEvent: (eventId: string | null) => getState().highlightEvent(eventId),

    // Calendar management
    getCalendars: () => getState().getCalendars(),
    reorderCalendars: (fromIndex: number, toIndex: number) =>
      getState().reorderCalendars(fromIndex, toIndex),
    setCalendarVisibility: (calendarId: string, visible: boolean) =>
      getState().setCalendarVisibility(calendarId, visible),
    setAllCalendarsVisibility: (visible: boolean) =>
      getState().setAllCalendarsVisibility(visible),
    updateCalendar: (id: string, updates: Partial<CalendarType>) =>
      getState().updateCalendar(id, updates),
    createCalendar: (calendar: CalendarType) => getState().createCalendar(calendar),
    deleteCalendar: (id: string) => getState().deleteCalendar(id),
    mergeCalendars: (sourceId: string, targetId: string) =>
      getState().mergeCalendars(sourceId, targetId),

    // Plugin management
    getPlugin: <T = unknown>(name: string) => getState().getPlugin<T>(name),
    hasPlugin: (name: string) => getState().hasPlugin(name),

    // Rendering (legacy - no-op in new implementation)
    render: () => React.createElement('div', { className: 'calendar-app' }, 'Calendar App'),

    // Config getters
    getSidebarConfig: () => getState().getSidebarConfig(),
    triggerRender: () => getState().triggerRender(),
    getCalendarRegistry: () => getState().getCalendarRegistry(),
    getUseEventDetailDialog: () => getState().getUseEventDetailDialog(),

    // Theme management
    setTheme: (mode) => getState().setTheme(mode),
    getTheme: () => getState().getTheme(),
    subscribeThemeChange: (callback) => getState().subscribeThemeChange(callback),
    unsubscribeThemeChange: (callback) => getState().unsubscribeThemeChange(callback),
  };
}

/**
 * Hook to use the calendar application
 * Provides a Zustand-based state management with selective subscriptions
 */
export function useCalendarApp(config: CalendarAppConfig): UseCalendarAppReturn {
  // Create store once per component instance and set as singleton
  const store = useMemo(() => {
    const newStore = createCalendarStore(config);
    CalendarStoreSingleton.set(newStore);
    return newStore;
  }, []);

  // Subscribe to reactive state with selective subscriptions
  // This is the key improvement - only re-render when specific state changes
  const currentView = useStore(store, state => state.currentView);
  const currentDate = useStore(store, state => state.currentDate);

  // Subscribe to raw events and calendar version
  const rawEvents = useStore(store, state => state.calendarEvents);
  const calendarVersion = useStore(store, state => state._calendarVersion);

  // Compute visible events with useMemo to avoid creating new arrays on every render
  const events = useMemo(() => {
    const registry = store.getState()._calendarRegistry;
    const visibleCalendars = new Set(
      registry
        .getAll()
        .filter(calendar => calendar.isVisible !== false)
        .map(calendar => calendar.id)
    );

    return rawEvents.filter((event: CalendarEvent) => {
      if (!event.calendarId) return true;
      if (!registry.has(event.calendarId)) return true;
      return visibleCalendars.has(event.calendarId);
    });
  }, [rawEvents, calendarVersion, store]);

  // Create the CalendarApp-compatible interface
  const app = useMemo(() => createAppFromStore(store), [store]);

  // Track previous values to avoid unnecessary updates
  const prevEventsRef = useRef<CalendarEvent[] | undefined>(undefined);
  const prevCalendarsRef = useRef<CalendarType[] | undefined>(undefined);

  // Sync events from config when they change
  useEffect(() => {
    const configEvents = config.calendarEvents;
    // Only update if events array reference changed and has content
    if (configEvents && configEvents !== prevEventsRef.current) {
      prevEventsRef.current = configEvents;
      store.getState().setEvents(configEvents);
    }
  }, [config.calendarEvents, store]);

  // Sync calendars from config when they change
  useEffect(() => {
    const configCalendars = config.calendars;
    // Only update if calendars array reference changed and has content
    if (configCalendars && configCalendars !== prevCalendarsRef.current) {
      prevCalendarsRef.current = configCalendars;
      store.getState().setCalendars(configCalendars);
    }
  }, [config.calendars, store]);

  return {
    app,
    currentView,
    currentDate,
    calendarEvents: events,

    // Expose actions directly for convenience
    changeView: store.getState().changeView,
    setCurrentDate: store.getState().setCurrentDate,
    addEvent: store.getState().addEvent,
    updateEvent: store.getState().updateEvent,
    deleteEvent: store.getState().deleteEvent,
    goToToday: store.getState().goToToday,
    goToPrevious: store.getState().goToPrevious,
    goToNext: store.getState().goToNext,
    selectDate: store.getState().selectDate,
    getCalendars: store.getState().getCalendars,
    createCalendar: store.getState().createCalendar,
    mergeCalendars: store.getState().mergeCalendars,
    setCalendarVisibility: store.getState().setCalendarVisibility,
    setAllCalendarsVisibility: store.getState().setAllCalendarsVisibility,
    getAllEvents: store.getState().getAllEvents,
    highlightEvent: store.getState().highlightEvent,
    setVisibleMonth: store.getState().setVisibleMonth,
    getVisibleMonth: store.getState().getVisibleMonth,
    sidebarConfig: store.getState().getSidebarConfig(),
  };
}

// Re-export for direct store access in advanced use cases
export { createCalendarStore } from './calendarStore';
export type { CalendarStoreApi, CalendarStore } from './calendarStore';
