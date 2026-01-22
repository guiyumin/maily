/**
 * Zustand selectors for calendar store
 * Use these for selective subscriptions to minimize re-renders
 */
import { CalendarStore } from './calendarStore';

/**
 * Select visible events (filtered by calendar visibility)
 * Only re-computes when events or calendar version changes
 */
export const selectVisibleEvents = (state: CalendarStore) => {
  const visibleCalendars = new Set(
    state._calendarRegistry
      .getAll()
      .filter(calendar => calendar.isVisible !== false)
      .map(calendar => calendar.id)
  );

  return state.events.filter(event => {
    if (!event.calendarId) {
      return true;
    }
    if (!state._calendarRegistry.has(event.calendarId)) {
      return true;
    }
    return visibleCalendars.has(event.calendarId);
  });
};

/**
 * Select current date
 */
export const selectCurrentDate = (state: CalendarStore) => state.currentDate;

/**
 * Select current view type
 */
export const selectCurrentView = (state: CalendarStore) => state.currentView;

/**
 * Select highlighted event ID
 */
export const selectHighlightedEventId = (state: CalendarStore) => state.highlightedEventId;

/**
 * Select visible month
 */
export const selectVisibleMonth = (state: CalendarStore) => state.visibleMonth;

/**
 * Select locale
 */
export const selectLocale = (state: CalendarStore) => state.locale;

/**
 * Select switcher mode
 */
export const selectSwitcherMode = (state: CalendarStore) => state.switcherMode;

/**
 * Select all events (unfiltered)
 */
export const selectAllEvents = (state: CalendarStore) => state.events;

/**
 * Select calendar version (use to detect calendar registry changes)
 */
export const selectCalendarVersion = (state: CalendarStore) => state._calendarVersion;

/**
 * Create a selector for events on a specific date
 */
export const createSelectEventsForDate = (date: Date) => (state: CalendarStore) => {
  const dateStr = date.toISOString().split('T')[0];
  return selectVisibleEvents(state).filter(event => {
    // This is a simplified check - you may need more sophisticated date comparison
    // depending on how your events store dates
    const eventStart = event.start;
    if ('toPlainDate' in eventStart) {
      return eventStart.toPlainDate?.().toString() === dateStr;
    }
    return false;
  });
};

/**
 * Create a selector for events in a date range
 */
export const createSelectEventsInRange = (start: Date, end: Date) => (state: CalendarStore) => {
  const startTime = start.getTime();
  const endTime = end.getTime();

  return selectVisibleEvents(state).filter(event => {
    // Convert event dates to timestamps for comparison
    // This is simplified - you may need to handle Temporal types properly
    const eventStart = event.start;
    let eventStartTime: number;

    if (eventStart instanceof Date) {
      eventStartTime = eventStart.getTime();
    } else if ('epochMilliseconds' in eventStart) {
      eventStartTime = (eventStart as any).epochMilliseconds;
    } else {
      // PlainDate or PlainDateTime - approximate conversion
      return true; // Include by default
    }

    return eventStartTime >= startTime && eventStartTime <= endTime;
  });
};
