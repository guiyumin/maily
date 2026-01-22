import React, { useState, useEffect, useMemo } from "react";
import clsx from "clsx";
import { CalendarApp } from "@calendar/core";
import {
  formatTime,
  weekDays,
  extractHourFromDate,
  createDateWithHour,
  getLineColor,
  getEventEndHour,
} from "@calendar/utils";
import { useLocale } from "@calendar/locale";
import {
  EventLayout,
  Event,
  EventDetailContentRenderer,
  EventDetailDialogRenderer,
  ViewType,
} from "@calendar/types";
import CalendarEvent from "@calendar/components/weekView/CalendarEvent";
import { EventLayoutCalculator } from "@calendar/components/EventLayout";
import { useDragForView } from "@calendar/plugins/dragPlugin";
import { ViewType as DragViewType, WeekDayDragState } from "@calendar/types";
import { defaultDragConfig } from "@calendar/core/config";
import ViewHeader, {
  ViewSwitcherMode,
} from "@calendar/components/common/ViewHeader";
import { temporalToDate, dateToZonedDateTime } from "@calendar/utils/temporal";
import { useCalendarDrop } from "@calendar/hooks/useCalendarDrop";

interface DayViewProps {
  app: CalendarApp;
  customDetailPanelContent?: EventDetailContentRenderer;
  customEventDetailDialog?: EventDetailDialogRenderer;
  calendarRef: React.RefObject<HTMLDivElement>;
  switcherMode?: ViewSwitcherMode;
}

const DayView: React.FC<DayViewProps> = ({
  app,
  customDetailPanelContent,
  customEventDetailDialog,
  calendarRef,
  switcherMode = "buttons",
}) => {
  const events = app.getEvents();
  const { t, locale } = useLocale();

  const [currentTime, setCurrentTime] = useState<Date | null>(null);
  const [selectedEvent, setSelectedEvent] = useState<Event | null>(null);
  const [detailPanelEventId, setDetailPanelEventId] = useState<string | null>(
    null,
  );

  // Sync highlighted event from app state
  useEffect(() => {
    if (app.state.highlightedEventId) {
      const event = events.find((e) => e.id === app.state.highlightedEventId);
      if (event) {
        setSelectedEvent(event);
      }
    }
  }, [app.state.highlightedEventId, events]);

  const [newlyCreatedEventId, setNewlyCreatedEventId] = useState<string | null>(
    null,
  );

  const currentDate = app.getCurrentDate();

  // Get configuration constants
  const {
    HOUR_HEIGHT,
    FIRST_HOUR,
    LAST_HOUR,
    TIME_COLUMN_WIDTH,
    ALL_DAY_HEIGHT,
  } = defaultDragConfig;

  // References
  const allDayRowRef = React.useRef<HTMLDivElement>(null);

  // Utility function: Get week start time
  const getWeekStart = (date: Date): Date => {
    const day = date.getDay();
    const diff = date.getDate() - day + (day === 0 ? -6 : 1);
    const monday = new Date(date);
    monday.setDate(diff);
    monday.setHours(0, 0, 0, 0);
    return monday;
  };

  // Calculate the week start time for the current date
  const currentWeekStart = useMemo(
    () => getWeekStart(currentDate),
    [currentDate],
  );

  // Events for the current date
  const currentDayEvents = useMemo(() => {
    const filtered = events.filter((event) => {
      const eventDate = temporalToDate(event.start);
      eventDate.setHours(0, 0, 0, 0);
      const targetDate = new Date(currentDate);
      targetDate.setHours(0, 0, 0, 0);
      return eventDate.getTime() === targetDate.getTime();
    });

    // Recalculate the day field to fit the current week start time
    return filtered.map((event) => {
      const eventDate = temporalToDate(event.start);
      const dayDiff = Math.floor(
        (eventDate.getTime() - currentWeekStart.getTime()) /
          (24 * 60 * 60 * 1000),
      );
      const correctDay = Math.max(0, Math.min(6, dayDiff)); // Ensure within 0-6 range

      return {
        ...event,
        day: correctDay,
      };
    });
  }, [events, currentDate, currentWeekStart]);

  // Calculate event layouts
  const eventLayouts = useMemo(() => {
    return EventLayoutCalculator.calculateDayEventLayouts(currentDayEvents, {
      viewType: "day",
    });
  }, [currentDayEvents]);

  // Calculate layout for newly created events
  const calculateNewEventLayout = (
    targetDay: number,
    startHour: number,
    endHour: number,
  ): EventLayout | null => {
    const startDate = new Date(currentDate);
    const endDate = new Date(currentDate);
    startDate.setHours(Math.floor(startHour), (startHour % 1) * 60, 0, 0);
    endDate.setHours(Math.floor(endHour), (endHour % 1) * 60, 0, 0);

    const tempEvent: Event = {
      id: "-1",
      title: "Temp",
      day: targetDay,
      start: dateToZonedDateTime(startDate),
      end: dateToZonedDateTime(endDate),
      calendarId: "blue",
      allDay: false,
    };

    const dayEvents = [...currentDayEvents.filter((e) => !e.allDay), tempEvent];
    const tempLayouts = EventLayoutCalculator.calculateDayEventLayouts(
      dayEvents,
      { viewType: "day" },
    );
    return tempLayouts.get("-1") || null;
  };

  const calculateDragLayout = (
    draggedEvent: Event,
    targetDay: number,
    targetStartHour: number,
    targetEndHour: number,
  ): EventLayout | null => {
    // Create temporary event list, including the dragged event in the new position
    const tempEvents = currentDayEvents.map((e) => {
      if (e.id !== draggedEvent.id) return e;

      const eventDateForCalc = temporalToDate(e.start);
      const newStartDate = createDateWithHour(
        eventDateForCalc,
        targetStartHour,
      ) as Date;
      const newEndDate = createDateWithHour(
        eventDateForCalc,
        targetEndHour,
      ) as Date;
      const newStart = dateToZonedDateTime(newStartDate);
      const newEnd = dateToZonedDateTime(newEndDate);

      return { ...e, day: targetDay, start: newStart, end: newEnd };
    });

    const dayEvents = tempEvents.filter((e) => !e.allDay);

    if (dayEvents.length === 0) return null;

    // Use layout calculator to calculate temporary layout
    const tempLayouts = EventLayoutCalculator.calculateDayEventLayouts(
      dayEvents,
      { viewType: "day" },
    );
    return tempLayouts.get(draggedEvent.id) || null;
  };

  // Use drag functionality provided by the plugin
  const {
    handleMoveStart,
    handleCreateStart,
    handleResizeStart,
    handleCreateAllDayEvent,
    dragState,
    isDragging,
  } = useDragForView(app, {
    calendarRef,
    allDayRowRef,
    viewType: DragViewType.DAY,
    onEventsUpdate: (updateFunc: (events: Event[]) => Event[]) => {
      const newEvents = updateFunc(currentDayEvents);

      // Find events that need to be deleted (in old list but not in new list)
      const newEventIds = new Set(newEvents.map((e) => e.id));
      const eventsToDelete = currentDayEvents.filter(
        (e) => !newEventIds.has(e.id),
      );

      // Find events that need to be added (in new list but not in old list)
      const oldEventIds = new Set(currentDayEvents.map((e) => e.id));
      const eventsToAdd = newEvents.filter((e) => !oldEventIds.has(e.id));

      // Find events that need to be updated (exist in both lists but content may differ)
      const eventsToUpdate = newEvents.filter((e) => {
        if (!oldEventIds.has(e.id)) return false;
        const oldEvent = currentDayEvents.find((old) => old.id === e.id);
        // Check if there are real changes
        return (
          oldEvent &&
          (temporalToDate(oldEvent.start).getTime() !==
            temporalToDate(e.start).getTime() ||
            temporalToDate(oldEvent.end).getTime() !==
              temporalToDate(e.end).getTime() ||
            oldEvent.day !== e.day ||
            extractHourFromDate(oldEvent.start) !==
              extractHourFromDate(e.start) ||
            extractHourFromDate(oldEvent.end) !== extractHourFromDate(e.end) ||
            oldEvent.title !== e.title)
        );
      });

      // Perform operations - updateEvent will automatically trigger onEventUpdate callback
      eventsToDelete.forEach((event) => app.deleteEvent(event.id));
      eventsToAdd.forEach((event) => app.addEvent(event));
      eventsToUpdate.forEach((event) => app.updateEvent(event.id, event));
    },
    onEventCreate: (event: Event) => {
      app.addEvent(event);
    },
    onEventEdit: () => {
      // Event edit handling (add logic here if needed)
    },
    currentWeekStart,
    events: currentDayEvents,
    calculateNewEventLayout,
    calculateDragLayout,
  });

  // Use calendar drop functionality
  const { handleDrop, handleDragOver } = useCalendarDrop({
    app,
    onEventCreated: (event: Event) => {
      setNewlyCreatedEventId(event.id);
    },
  });

  // Event handling functions
  const handleEventUpdate = (updatedEvent: Event) => {
    app.updateEvent(updatedEvent.id, updatedEvent);
  };

  const handleEventDelete = (eventId: string) => {
    app.deleteEvent(eventId);
  };

  const timeSlots = Array.from({ length: 24 }, (_, i) => ({
    hour: i + FIRST_HOUR,
    label: formatTime(i + FIRST_HOUR),
  }));

  // Check if it is today
  const isToday = useMemo(() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const current = new Date(currentDate);
    current.setHours(0, 0, 0, 0);
    return current.getTime() === today.getTime();
  }, [currentDate]);

  // Timer
  useEffect(() => {
    setCurrentTime(new Date());
    const timer = setInterval(() => setCurrentTime(new Date()), 60_000);
    return () => clearInterval(timer);
  }, []);

  return (
    <div className={clsx("flex h-full", "bg-gray-50 dark:bg-gray-800")}>
      {/* Main calendar area */}
      <div className="w-full bg-white dark:bg-gray-900">
        <div className={clsx("relative", "flex flex-col", "h-full")}>
          {/* Fixed navigation bar */}
          <ViewHeader
            viewType={ViewType.DAY}
            currentDate={currentDate}
            onPrevious={() => app.goToPrevious()}
            onNext={() => app.goToNext()}
            onToday={() => app.goToToday()}
          />
          {/* All-day event area */}
          <div
            className="flex items-center border-b border-gray-200 dark:border-gray-700 sticky pr-[10px] pt-px"
            ref={allDayRowRef}
          >
            <div className="w-20 flex-shrink-0 p-1 text-xs font-medium text-gray-500 dark:text-gray-400 flex justify-end">
              {t("allDay")}
            </div>
            <div className="flex flex-1 relative">
              <div
                className="w-full relative"
                style={{ minHeight: `${ALL_DAY_HEIGHT}px` }}
                onDoubleClick={(e) => {
                  const currentDayIndex = Math.floor(
                    (currentDate.getTime() - currentWeekStart.getTime()) /
                      (24 * 60 * 60 * 1000),
                  );
                  handleCreateAllDayEvent?.(e, currentDayIndex);
                }}
                onDragOver={handleDragOver}
                onDrop={(e) => {
                  handleDrop(e, currentDate, undefined, true);
                }}
              >
                {currentDayEvents
                  .filter((event) => event.allDay)
                  .map((event) => (
                    <CalendarEvent
                      key={event.id}
                      event={event}
                      isAllDay={true}
                      isDayView={true}
                      allDayHeight={ALL_DAY_HEIGHT}
                      calendarRef={calendarRef}
                      isBeingDragged={
                        isDragging &&
                        (dragState as WeekDayDragState)?.eventId === event.id &&
                        (dragState as WeekDayDragState)?.mode === "move"
                      }
                      hourHeight={HOUR_HEIGHT}
                      firstHour={FIRST_HOUR}
                      onMoveStart={handleMoveStart}
                      onEventUpdate={handleEventUpdate}
                      onEventDelete={handleEventDelete}
                      newlyCreatedEventId={newlyCreatedEventId}
                      onDetailPanelOpen={() => setNewlyCreatedEventId(null)}
                      detailPanelEventId={detailPanelEventId}
                      onDetailPanelToggle={(eventId: string | null) =>
                        setDetailPanelEventId(eventId)
                      }
                      selectedEventId={selectedEvent?.id ?? null}
                      onEventSelect={(eventId: string | null) => {
                        const evt = events.find((e) => e.id === eventId);
                        setSelectedEvent(evt || null);
                      }}
                      customDetailPanelContent={customDetailPanelContent}
                      customEventDetailDialog={customEventDetailDialog}
                      app={app}
                    />
                  ))}
              </div>
            </div>
          </div>

          {/* Time grid and event area */}
          <div
            className="relative overflow-y-scroll calendar-content"
            style={{ position: "relative" }}
          >
            <div className="relative flex">
              {/* Current time line */}
              {isToday &&
                currentTime &&
                (() => {
                  const now = currentTime;
                  const hours = now.getHours() + now.getMinutes() / 60;
                  if (hours < FIRST_HOUR || hours > LAST_HOUR) return null;

                  const topPx = (hours - FIRST_HOUR) * HOUR_HEIGHT;

                  return (
                    <div
                      className="absolute left-0 top-0 flex pointer-events-none"
                      style={{
                        top: `${topPx}px`,
                        width: "100%",
                        height: 0,
                        zIndex: 20,
                      }}
                    >
                      <div
                        className="flex items-center"
                        style={{ width: `${TIME_COLUMN_WIDTH}px` }}
                      >
                        <div className="relative w-full flex items-center"></div>
                        <div className="ml-2 text-primary-foreground text-xs font-bold px-1.5 bg-primary rounded-sm">
                          {formatTime(hours)}
                        </div>
                      </div>

                      <div className="flex-1 flex items-center">
                        <div className="h-0.5 w-full bg-primary relative" />
                      </div>
                    </div>
                  );
                })()}

              {/* Time column */}
              <div className="w-20 flex-shrink-0 border-gray-200 dark:border-gray-700">
                {timeSlots.map((slot, slotIndex) => (
                  <div key={slotIndex} className="relative h-[4.5rem] flex">
                    <div className="absolute -top-2.5 right-2 text-[12px] text-gray-500 dark:text-gray-400">
                      {slotIndex === 0 ? "" : slot.label}
                    </div>
                  </div>
                ))}
              </div>

              {/* Time grid */}
              <div className="grow relative">
                {timeSlots.map((slot, slotIndex) => (
                  <div
                    key={slotIndex}
                    className="h-[4.5rem] border-t first:border-none border-gray-200 dark:border-gray-700 flex"
                    onDoubleClick={(e) => {
                      const currentDayIndex = Math.floor(
                        (currentDate.getTime() - currentWeekStart.getTime()) /
                          (24 * 60 * 60 * 1000),
                      );
                      const rect = calendarRef.current
                        ?.querySelector(".calendar-content")
                        ?.getBoundingClientRect();
                      if (!rect) return;
                      const relativeY =
                        e.clientY -
                          rect.top +
                          (
                            calendarRef.current?.querySelector(
                              ".calendar-content",
                            ) as HTMLElement
                          )?.scrollTop || 0;
                      const clickedHour = FIRST_HOUR + relativeY / HOUR_HEIGHT;
                      handleCreateStart(e, currentDayIndex, clickedHour);
                    }}
                    onDragOver={handleDragOver}
                    onDrop={(e) => {
                      const rect = calendarRef.current
                        ?.querySelector(".calendar-content")
                        ?.getBoundingClientRect();
                      if (!rect) return;
                      const relativeY =
                        e.clientY -
                          rect.top +
                          (
                            calendarRef.current?.querySelector(
                              ".calendar-content",
                            ) as HTMLElement
                          )?.scrollTop || 0;
                      const dropHour = Math.floor(
                        FIRST_HOUR + relativeY / HOUR_HEIGHT,
                      );
                      handleDrop(e, currentDate, dropHour);
                    }}
                  />
                ))}

                {/* Bottom boundary */}
                <div className="h-3 border-t border-gray-200 dark:border-gray-700 relative">
                  <div className="absolute -top-2.5 -left-9 text-[12px] text-gray-500 dark:text-gray-400">
                    00.00
                  </div>
                </div>

                {/* Event layer */}
                <div className="absolute top-0 left-0 right-0 bottom-0 pointer-events-none">
                  {currentDayEvents
                    .filter((event) => !event.allDay)
                    .map((event) => {
                      const eventLayout = eventLayouts.get(event.id);
                      return (
                        <CalendarEvent
                          key={event.id}
                          event={event}
                          layout={eventLayout}
                          isDayView={true}
                          calendarRef={calendarRef}
                          isBeingDragged={
                            isDragging &&
                            (dragState as WeekDayDragState)?.eventId ===
                              event.id &&
                            (dragState as WeekDayDragState)?.mode === "move"
                          }
                          hourHeight={HOUR_HEIGHT}
                          firstHour={FIRST_HOUR}
                          onMoveStart={handleMoveStart}
                          onResizeStart={handleResizeStart}
                          onEventUpdate={handleEventUpdate}
                          onEventDelete={handleEventDelete}
                          newlyCreatedEventId={newlyCreatedEventId}
                          onDetailPanelOpen={() => setNewlyCreatedEventId(null)}
                          detailPanelEventId={detailPanelEventId}
                          onDetailPanelToggle={(eventId: string | null) =>
                            setDetailPanelEventId(eventId)
                          }
                          selectedEventId={selectedEvent?.id ?? null}
                          onEventSelect={(eventId: string | null) => {
                            const evt = events.find((e) => e.id === eventId);
                            setSelectedEvent(evt || null);
                          }}
                          customDetailPanelContent={customDetailPanelContent}
                          customEventDetailDialog={customEventDetailDialog}
                          app={app}
                        />
                      );
                    })}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default DayView;
