import React, { useMemo, useCallback } from 'react';
import {
  ViewType,
  ViewAdapterProps,
  BaseViewProps,
  EventsService,
  DragService,
} from '../types';
import { CalendarEvent } from '../types';

export const ViewAdapter: React.FC<ViewAdapterProps> = ({
  originalComponent: OriginalComponent,
  app,
  config,
  customDetailPanelContent,
  customEventDetailDialog,
  calendarRef,
  switcherMode,
  meta,
}) => {
  // Get plugin services
  const eventsService = app.getPlugin<EventsService>('events');
  const dragService = config.enableDrag
    ? app.getPlugin<DragService>('drag')
    : undefined;

  // Basic state
  const currentDate = app.getCurrentDate();
  const currentView = app.state.currentView;
  const calendarEvents = app.getEvents();

  // Event handlers
  const handleEventUpdate = useCallback(
    (event: CalendarEvent) => {
      if (eventsService) {
        eventsService.update(event.id, event);
      } else {
        app.updateEvent(event.id, event);
      }
    },
    [eventsService, app]
  );

  const handleEventDelete = useCallback(
    (eventId: string) => {
      if (eventsService) {
        eventsService.delete(eventId);
      } else {
        app.deleteEvent(eventId);
      }
    },
    [eventsService, app]
  );

  const handleEventCreate = useCallback(
    (event: CalendarEvent) => {
      if (eventsService) {
        eventsService.add(event);
      } else {
        app.addEvent(event);
      }
    },
    [eventsService, app]
  );

  const handleDateChange = useCallback(
    (date: Date) => {
      app.setCurrentDate(date);
    },
    [app]
  );

  const handleViewChange = useCallback(
    (view: ViewType) => {
      app.changeView(view);
    },
    [app]
  );

  // Merge configuration
  const mergedConfig = useMemo(() => {
    const baseConfig = config.viewConfig || {};

    // Add plugin configuration
    if (dragService) {
      Object.assign(baseConfig, config.dragConfig || {});
    }

    if (eventsService) {
      Object.assign(baseConfig, config.eventsConfig || {});
    }

    return baseConfig;
  }, [config, dragService, eventsService]);

  // Prepare props to pass to original component
  const viewProps: BaseViewProps = useMemo(
    () => ({
      app,
      currentDate,
      currentView,
      calendarEvents,
      onEventUpdate: handleEventUpdate,
      onEventDelete: handleEventDelete,
      onEventCreate: handleEventCreate,
      onDateChange: handleDateChange,
      onViewChange: handleViewChange,
      config: mergedConfig,
    }),
    [
      app,
      currentDate,
      currentView,
      calendarEvents,
      handleEventUpdate,
      handleEventDelete,
      handleEventCreate,
      handleDateChange,
      handleViewChange,
      mergedConfig,
    ]
  );

  // Special handling: prepare compatible props for existing components
  const compatProps = useMemo(() => {
    // Create a calendar object compatible with the existing API
    const calendarCompat = {
      currentDate,
      calendarEvents,
      setEvents: (newEvents: CalendarEvent[]) => {
        // Clear existing events and add new events
        calendarEvents.forEach((event: CalendarEvent) => handleEventDelete(event.id));
        newEvents.forEach((event: CalendarEvent) => handleEventCreate(event));
      },
      updateEvent: handleEventUpdate,
      deleteEvent: handleEventDelete,
      addEvent: handleEventCreate,
      goToPrevious: () => {
        const newDate = new Date(currentDate);
        switch (currentView) {
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
        handleDateChange(newDate);
      },
      goToNext: () => {
        const newDate = new Date(currentDate);
        switch (currentView) {
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
        handleDateChange(newDate);
      },
      goToToday: () => {
        handleDateChange(new Date());
      },
      changeView: handleViewChange,
      selectDate: handleDateChange,
    };

    return {
      calendar: calendarCompat,
      ...viewProps,
      // Pass plugin services (if original component needs them)
      eventsService,
      dragService,
      // Pass custom detail panel
      customDetailPanelContent,
      customEventDetailDialog,
      // Pass calendarRef and meta
      calendarRef,
      switcherMode,
      meta,
    };
  }, [
    currentDate,
    calendarEvents,
    currentView,
    handleEventUpdate,
    handleEventDelete,
    handleEventCreate,
    handleDateChange,
    handleViewChange,
    viewProps,
    eventsService,
    dragService,
    customDetailPanelContent,
    customEventDetailDialog,
    calendarRef,
    switcherMode,
    meta,
  ]);

  return <OriginalComponent {...compatProps} />;
};

export default ViewAdapter;
