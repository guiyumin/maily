import React, { useMemo, useState, useEffect, useRef } from 'react';
import ReactDOM from 'react-dom';
import { Temporal } from 'temporal-polyfill';
import { EventDetailPanelProps } from '../../types/eventDetail';
import { isPlainDate } from '../../utils/temporal';
import { getDefaultCalendarRegistry } from '../../core/calendarRegistry';
import ColorPicker, { ColorOption } from './ColorPicker';
import RangePicker from './RangePicker';
import { useCalendarTheme } from '../../core/calendarStoreHooks';
import { resolveAppliedTheme } from '../../utils/themeUtils';
import { CalendarApp } from '@calendar/core';
import { useLocale } from '@calendar/locale';
import { Button } from '@/components/ui/button';

interface DefaultEventDetailPanelProps extends EventDetailPanelProps {
  app?: CalendarApp;
}

/**
 * Default event detail panel component
 */
const DefaultEventDetailPanel: React.FC<DefaultEventDetailPanelProps> = ({
  calendarEvent,
  position,
  panelRef,
  isAllDay,
  eventVisibility,
  calendarRef,
  selectedEventElementRef,
  onEventUpdate,
  onEventDelete,
  onClose,
  app,
}) => {
  const { effectiveTheme } = useCalendarTheme();
  const appliedTheme = resolveAppliedTheme(effectiveTheme);
  const { t } = useLocale();
  const arrowBgColor = appliedTheme === 'dark' ? '#1f2937' : 'white';
  const arrowBorderColor = appliedTheme === 'dark' ? 'rgb(55, 65, 81)' : 'rgb(229, 231, 235)';

  // Use local state for editing to prevent stale closure issues
  const [editedEvent, setEditedEvent] = useState(calendarEvent);

  // Track if user is currently editing to avoid overwriting their changes
  const isEditingRef = useRef(false);

  // Sync state only when the event ID changes (not on every prop update)
  useEffect(() => {
    if (!isEditingRef.current) {
      setEditedEvent(calendarEvent);
    }
  }, [calendarEvent.id]);

  // Debounced save - update parent after user stops typing
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingUpdateRef = useRef<typeof editedEvent | null>(null);

  const handleFieldChange = <K extends keyof typeof editedEvent>(
    field: K,
    value: typeof editedEvent[K]
  ) => {
    isEditingRef.current = true;

    // Update local state immediately for responsive UI
    setEditedEvent(prev => {
      const updated = { ...prev, [field]: value };
      pendingUpdateRef.current = updated;
      return updated;
    });

    // Clear existing timeout
    if (saveTimeoutRef.current) {
      clearTimeout(saveTimeoutRef.current);
    }

    // Debounce the save to parent
    saveTimeoutRef.current = setTimeout(() => {
      if (pendingUpdateRef.current) {
        onEventUpdate(pendingUpdateRef.current);
      }
      isEditingRef.current = false;
    }, 300);
  };

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (saveTimeoutRef.current) {
        clearTimeout(saveTimeoutRef.current);
      }
    };
  }, []);

  // Get visible calendar type options
  const colorOptions: ColorOption[] = useMemo(() => {
    const registry = app ? app.getCalendarRegistry() : getDefaultCalendarRegistry();
    return registry.getVisible().map(cal => ({
      label: cal.name,
      value: cal.id,
    }));
  }, [app, app?.getCalendars()]); // Depend on app.getCalendars() to update when calendars change

  const convertToAllDay = () => {
    const plainDate = isPlainDate(editedEvent.start)
      ? editedEvent.start
      : editedEvent.start.toPlainDate();
    const updated = {
      ...editedEvent,
      allDay: true,
      start: plainDate,
      end: plainDate,
    };
    setEditedEvent(updated);
    onEventUpdate(updated);
  };

  const convertToRegular = () => {
    const plainDate = isPlainDate(editedEvent.start)
      ? editedEvent.start
      : editedEvent.start.toPlainDate();
    const start = Temporal.ZonedDateTime.from({
      year: plainDate.year,
      month: plainDate.month,
      day: plainDate.day,
      hour: 9,
      minute: 0,
      timeZone: Temporal.Now.timeZoneId(),
    });
    const end = Temporal.ZonedDateTime.from({
      year: plainDate.year,
      month: plainDate.month,
      day: plainDate.day,
      hour: 10,
      minute: 0,
      timeZone: Temporal.Now.timeZoneId(),
    });
    const updated = {
      ...editedEvent,
      allDay: false,
      start,
      end,
    };
    setEditedEvent(updated);
    onEventUpdate(updated);
  };

  const eventTimeZone = useMemo(() => {
    if (!isPlainDate(editedEvent.start)) {
      return (
        (editedEvent.start as any).timeZoneId ||
        (editedEvent.start as Temporal.ZonedDateTime).timeZoneId ||
        Temporal.Now.timeZoneId()
      );
    }

    if (editedEvent.end && !isPlainDate(editedEvent.end)) {
      return (
        (editedEvent.end as any).timeZoneId ||
        (editedEvent.end as Temporal.ZonedDateTime).timeZoneId ||
        Temporal.Now.timeZoneId()
      );
    }

    return Temporal.Now.timeZoneId();
  }, [editedEvent.end, editedEvent.start]);

  const handleAllDayRangeChange = (
    nextRange: [Temporal.ZonedDateTime, Temporal.ZonedDateTime]
  ) => {
    const [start, end] = nextRange;
    const updated = {
      ...editedEvent,
      start: start.toPlainDate(),
      end: end.toPlainDate(),
    };
    setEditedEvent(updated);
    onEventUpdate(updated);
  };

  // Calculate arrow style
  const calculateArrowStyle = (): React.CSSProperties => {
    let arrowStyle: React.CSSProperties = {};

    if (eventVisibility === 'sticky-top') {
      const calendarContent =
        calendarRef.current?.querySelector('.calendar-content');
      if (calendarContent) {
        const contentRect = calendarContent.getBoundingClientRect();
        const stickyEventCenterY = contentRect.top + 3;
        const arrowRelativeY = stickyEventCenterY - position.top;

        arrowStyle = {
          position: 'absolute',
          width: '12px',
          height: '12px',
          backgroundColor: arrowBgColor,
          transform: 'rotate(45deg)',
          transformOrigin: 'center',
          top: `${arrowRelativeY - 6}px`,
          borderRight: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
          borderTop: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
          borderLeft: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
          borderBottom: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
          ...(position.isSunday ? { right: '-6px' } : { left: '-6px' }),
        };
      }
    } else if (eventVisibility === 'sticky-bottom') {
      const panelElement = panelRef.current;
      let arrowTop = 200;

      if (panelElement) {
        const panelRect = panelElement.getBoundingClientRect();
        const computedStyle = window.getComputedStyle(panelElement);
        const paddingBottom = parseInt(computedStyle.paddingBottom, 10) || 0;
        const borderBottom = parseInt(computedStyle.borderBottomWidth, 10) || 0;

        arrowTop = panelRect.height - paddingBottom - borderBottom - 6 + 11;
      }

      arrowStyle = {
        position: 'absolute',
        width: '12px',
        height: '12px',
        backgroundColor: arrowBgColor,
        transform: 'rotate(45deg)',
        transformOrigin: 'center',
        top: `${arrowTop}px`,
        left: position.isSunday ? undefined : '-6px',
        right: position.isSunday ? '-6px' : undefined,
        borderRight: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
        borderTop: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
        borderLeft: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
        borderBottom: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
      };
    } else {
      if (position && selectedEventElementRef.current && calendarRef.current) {
        const eventRect =
          selectedEventElementRef.current.getBoundingClientRect();
        const calendarContent =
          calendarRef.current.querySelector('.calendar-content');

        if (calendarContent) {
          const viewportRect = calendarContent.getBoundingClientRect();

          const visibleTop = Math.max(eventRect.top, viewportRect.top);
          const visibleBottom = Math.min(eventRect.bottom, viewportRect.bottom);
          const visibleHeight = Math.max(0, visibleBottom - visibleTop);

          let targetY;
          if (visibleHeight === eventRect.height) {
            targetY = eventRect.top + eventRect.height / 2;
          } else if (visibleHeight > 0) {
            targetY = visibleTop + visibleHeight / 2;
          } else {
            targetY = eventRect.top + eventRect.height / 2;
          }

          const arrowRelativeY = targetY - position.top;

          const panelElement = panelRef.current;
          let maxArrowY = 240 - 12;

          if (panelElement) {
            const panelRect = panelElement.getBoundingClientRect();
            const computedStyle = window.getComputedStyle(panelElement);
            const paddingBottom =
              parseInt(computedStyle.paddingBottom, 10) || 0;
            const borderBottom =
              parseInt(computedStyle.borderBottomWidth, 10) || 0;

            maxArrowY = panelRect.height - paddingBottom - borderBottom + 11;
          }

          const minArrowY = 12;
          const finalArrowY = Math.max(
            minArrowY,
            Math.min(maxArrowY, arrowRelativeY)
          );

          arrowStyle = {
            position: 'absolute',
            width: '12px',
            height: '12px',
            backgroundColor: arrowBgColor,
            transform: 'rotate(45deg)',
            transformOrigin: 'center',
            top: `${finalArrowY - 6}px`,
            borderRight: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
            borderTop: `${position.isSunday ? `1px solid ${arrowBorderColor}` : 'none'}`,
            borderLeft: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
            borderBottom: `${position.isSunday ? 'none' : `1px solid ${arrowBorderColor}`}`,
            ...(position.isSunday ? { right: '-6px' } : { left: '-6px' }),
          };
        }
      }
    }

    return arrowStyle;
  };

  const arrowStyle = calculateArrowStyle();

  const panelContent = (
    <div
      ref={panelRef}
      className="fixed bg-white dark:bg-gray-800 shadow-lg border border-gray-200 dark:border-gray-700 rounded-lg p-4 "
      data-event-detail-panel="true"
      style={{
        top: `${position.top}px`,
        left: `${position.left}px`,
        zIndex: 9999,
        pointerEvents: 'auto',
      }}
    >
      <div style={arrowStyle}></div>
      <span className="block text-xs text-gray-600 dark:text-gray-300 mb-1">{t('eventTitle')}</span>
      <div className="flex items-center justify-between gap-3 mb-3">
        <div className="flex-1">
          <input
            type="text"
            value={editedEvent.title}
            onChange={e => {
              handleFieldChange('title', e.target.value);
            }}
            autoFocus
            className="w-full border border-slate-200 dark:border-gray-600 rounded-lg px-3 py-1.5 text-sm text-gray-900 dark:text-gray-100 dark:bg-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition"
          />
        </div>
        <ColorPicker
          options={colorOptions}
          value={editedEvent.calendarId || 'blue'}
          onChange={value => {
            handleFieldChange('calendarId', value);
          }}
          registry={app?.getCalendarRegistry()}
        />
      </div>

      {isAllDay ? (
        <div className="mb-3">
          <div className="text-xs text-gray-600 dark:text-gray-300 mb-1">{t('dateRange')}</div>
          <RangePicker
            value={[editedEvent.start, editedEvent.end]}
            format="YYYY-MM-DD"
            showTime={false}
            timeZone={eventTimeZone}
            matchTriggerWidth
            onChange={handleAllDayRangeChange}
            onOk={handleAllDayRangeChange}
            locale={app?.state.locale}
          />
        </div>
      ) : (
        <div className="mb-3">
          <div className="text-xs text-gray-600 dark:text-gray-300 mb-1">{t('timeRange')}</div>
          <RangePicker
            value={[editedEvent.start, editedEvent.end]}
            timeZone={eventTimeZone}
            onChange={(nextRange) => {
              const [start, end] = nextRange;
              const updated = { ...editedEvent, start, end };
              setEditedEvent(updated);
              onEventUpdate(updated);
            }}
            onOk={(nextRange) => {
              const [start, end] = nextRange;
              const updated = { ...editedEvent, start, end };
              setEditedEvent(updated);
              onEventUpdate(updated);
            }}
            locale={app?.state.locale}
          />
        </div>
      )}

      <div className="mb-3">
        <span className="block text-xs text-gray-600 dark:text-gray-300 mb-1">{t('note')}</span>
        <textarea
          value={editedEvent.description ?? ''}
          onChange={e => handleFieldChange('description', e.target.value)}
          rows={3}
          className="w-full border border-slate-200 dark:border-gray-600 rounded-lg px-3 py-2 text-sm text-gray-900 dark:text-gray-100 dark:bg-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition resize-none"
          placeholder={t('addNotePlaceholder')}
        />
      </div>

      <div className="flex space-x-2">
        {!isAllDay ? (
          <Button
            variant="secondary"
            size="sm"
            onClick={convertToAllDay}
          >
            {t('setAsAllDay')}
          </Button>
        ) : (
          <Button
            variant="secondary"
            size="sm"
            onClick={convertToRegular}
          >
            {t('setAsTimed')}
          </Button>
        )}

        <Button
          variant="destructive"
          size="sm"
          onClick={() => onEventDelete(calendarEvent.id)}
        >
          {t('delete')}
        </Button>

        {onClose && (
          <Button
            size="sm"
            className="ml-auto"
            onClick={() => {
              // Save any pending changes before closing
              if (saveTimeoutRef.current) {
                clearTimeout(saveTimeoutRef.current);
              }
              if (pendingUpdateRef.current) {
                onEventUpdate(pendingUpdateRef.current);
              }
              onClose();
            }}
          >
            OK
          </Button>
        )}
      </div>
    </div>
  );

  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return null;
  }

  const portalTarget = document.body;
  if (!portalTarget) return null;

  return ReactDOM.createPortal(panelContent, portalTarget);
};

export default DefaultEventDetailPanel;
