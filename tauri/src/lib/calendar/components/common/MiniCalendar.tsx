import React, { useMemo } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useLocale, getWeekDaysLabels } from '@calendar/locale';

interface MiniCalendarProps {
  visibleMonth: Date;
  currentDate: Date;
  showHeader?: boolean;
  onMonthChange: (offset: number) => void;
  onDateSelect: (date: Date) => void;
  onDateDoubleClick?: (date: Date) => void;
  locale?: string;
}

export const MiniCalendar: React.FC<MiniCalendarProps> = ({
  visibleMonth,
  currentDate,
  showHeader = false,
  onMonthChange,
  onDateSelect,
  onDateDoubleClick,
}) => {
  const { locale } = useLocale();
  const todayKey = useMemo(() => new Date().toDateString(), []);
  const currentDateKey = currentDate.toDateString();

  const weekdayLabels = useMemo(() => {
    return getWeekDaysLabels(locale, 'narrow');
  }, [locale]);

  const monthLabel = useMemo(
    () =>
      visibleMonth.toLocaleDateString(locale, {
        month: 'long',
        year: 'numeric',
      }),
    [visibleMonth, locale]
  );

  const miniCalendarDays = useMemo(() => {
    const year = visibleMonth.getFullYear();
    const month = visibleMonth.getMonth();
    const firstDay = new Date(year, month, 1);
    const startOffset = (firstDay.getDay() + 6) % 7; // Monday as first day
    const totalCells = 42;
    const days: Array<{
      date: number;
      fullDate: Date;
      isCurrentMonth: boolean;
      isToday: boolean;
      isSelected: boolean;
    }> = [];

    for (let cell = 0; cell < totalCells; cell++) {
      const cellDate = new Date(year, month, cell - startOffset + 1);
      const cellDateString = cellDate.toDateString();
      days.push({
        date: cellDate.getDate(),
        fullDate: cellDate,
        isCurrentMonth: cellDate.getMonth() === month,
        isToday: cellDateString === todayKey,
        isSelected: cellDateString === currentDateKey,
      });
    }

    return days;
  }, [visibleMonth, currentDateKey, todayKey]);

  return (
    <div className="px-3 py-3">
      {
        showHeader ? <div className="mb-3 flex items-center justify-between">
          <button
            type="button"
            className="flex h-7 w-7 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-slate-800"
            onClick={() => onMonthChange(-1)}
            aria-label="Previous month"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {monthLabel}
          </span>
          <button
            type="button"
            className="flex h-7 w-7 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-slate-800"
            onClick={() => onMonthChange(1)}
            aria-label="Next month"
          >
            <ChevronRight className="h-4 w-4" />
          </button>
        </div> : ''
      }
      <div className="grid grid-cols-7 gap-1 text-xs justify-items-center">
        {weekdayLabels.map((label, index) => (
          <div key={`weekday-${index}`} className="text-center text-gray-500 dark:text-gray-400 font-medium py-1 h-6 w-6">
            {label}
          </div>
        ))}
        {miniCalendarDays.map(day => (
          <button
            type="button"
            key={day.fullDate.getTime()}
            className={`
              text-center py-1 rounded text-xs h-6 w-6
              ${day.isToday
                ? 'bg-primary rounded-full text-primary-foreground'
                : day.isCurrentMonth
                  ? 'text-gray-900 dark:text-gray-100'
                  : 'text-gray-400 dark:text-gray-600'
              }
              ${day.isSelected && !day.isToday ? 'bg-secondary text-secondary-foreground rounded-full font-medium' : ''}
            `}
            onClick={() => onDateSelect(day.fullDate)}
            onDoubleClick={() => onDateDoubleClick?.(day.fullDate)}
          >
            {day.date}
          </button>
        ))}
      </div>
    </div>
  );
};
