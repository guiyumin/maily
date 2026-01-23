import React, { useMemo } from 'react';
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
  app,
}) => {
  const { effectiveTheme } = useCalendarTheme();
  const appliedTheme = resolveAppliedTheme(effectiveTheme);
  const { t } = useLocale();
  const arrowBgColor = appliedTheme === 'dark' ? '#1f2937' : 'white';
  const arrowBorderColor = appliedTheme === 'dark' ? 'rgb(55, 65, 81)' : 'rgb(229, 231, 235)';

  // Get visible calendar type options
  const colorOptions: ColorOption[] = useMemo(() => {
    const registry = app ? app.getCalendarRegistry() : getDefaultCalendarRegistry();
    return registry.getVisible().map(cal => ({
      label: cal.name,
      value: cal.id,
    }));
  }, [app, app?.getCalendars()]); // Depend on app.getCalendars() to update when calendars change

  const convertToAllDay = () => {
    const plainDate = isPlainDate(calendarEvent.start)
      ? calendarEvent.start
      : calendarEvent.start.toPlainDate();
    onEventUpdate({
      ...calendarEvent,
      allDay: true,
      start: plainDate,
      end: plainDate,
    });
  };

  const convertToRegular = () => {
    const plainDate = isPlainDate(calendarEvent.start)
      ? calendarEvent.start
      : calendarEvent.start.toPlainDate();
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
    onEventUpdate({
      ...calendarEvent,
      allDay: false,
      start,
      end,
    });
  };

  const eventTimeZone = useMemo(() => {
    if (!isPlainDate(calendarEvent.start)) {
      return (
        (calendarEvent.start as any).timeZoneId ||
        (calendarEvent.start as Temporal.ZonedDateTime).timeZoneId ||
        Temporal.Now.timeZoneId()
      );
    }

    if (calendarEvent.end && !isPlainDate(calendarEvent.end)) {
      return (
        (calendarEvent.end as any).timeZoneId ||
        (calendarEvent.end as Temporal.ZonedDateTime).timeZoneId ||
        Temporal.Now.timeZoneId()
      );
    }

    return Temporal.Now.timeZoneId();
  }, [calendarEvent.end, calendarEvent.start]);

  const handleAllDayRangeChange = (
    nextRange: [Temporal.ZonedDateTime, Temporal.ZonedDateTime]
  ) => {
    const [start, end] = nextRange;
    onEventUpdate({
      ...calendarEvent,
      start: start.toPlainDate(),
      end: end.toPlainDate(),
    });
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
            value={calendarEvent.title}
            onChange={e => {
              onEventUpdate({
                ...calendarEvent,
                title: e.target.value,
              });
            }}
            className="w-full border border-slate-200 dark:border-gray-600 rounded-lg px-3 py-1.5 text-sm text-gray-900 dark:text-gray-100 dark:bg-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition"
          />
        </div>
        <ColorPicker
          options={colorOptions}
          value={calendarEvent.calendarId || 'blue'}
          onChange={value => {
            onEventUpdate({
              ...calendarEvent,
              calendarId: value,
            });
          }}
          registry={app?.getCalendarRegistry()}
        />
      </div>

      {isAllDay ? (
        <div className="mb-3">
          <div className="text-xs text-gray-600 dark:text-gray-300 mb-1">{t('dateRange')}</div>
          <RangePicker
            value={[calendarEvent.start, calendarEvent.end]}
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
            value={[calendarEvent.start, calendarEvent.end]}
            timeZone={
              eventTimeZone
            }
            onChange={(nextRange) => {
              const [start, end] = nextRange;
              onEventUpdate({
                ...calendarEvent,
                start,
                end,
              });
            }}
            onOk={(nextRange) => {
              const [start, end] = nextRange;
              onEventUpdate({
                ...calendarEvent,
                start,
                end,
              });
            }}
            locale={app?.state.locale}
          />
        </div>
      )}

      <div className="mb-3">
        <span className="block text-xs text-gray-600 dark:text-gray-300 mb-1">{t('note')}</span>
        <textarea
          value={calendarEvent.description ?? ''}
          onChange={e =>
            onEventUpdate({
              ...calendarEvent,
              description: e.target.value,
            })
          }
          rows={3}
          className="w-full border border-slate-200 dark:border-gray-600 rounded-lg px-3 py-2 text-sm text-gray-900 dark:text-gray-100 dark:bg-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary transition resize-none"
          placeholder={t('addNotePlaceholder')}
        />
      </div>

      <div className="flex space-x-2">
        {!isAllDay ? (
          <button
            className="px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary text-xs font-medium transition"
            onClick={convertToAllDay}
          >
            {t('setAsAllDay')}
          </button>
        ) : (
          <button
            className="px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary text-xs font-medium transition"
            onClick={convertToRegular}
          >
            {t('setAsTimed')}
          </button>
        )}

        <button
          className="px-2 py-1 bg-destructive text-destructive-foreground rounded hover:bg-destructive/90 text-xs font-medium transition"
          onClick={() => onEventDelete(calendarEvent.id)}
        >
          {t('delete')}
        </button>
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
