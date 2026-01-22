import React, { useMemo, useEffect, useRef } from 'react';
import { Loader2, X, Search } from 'lucide-react';
import { CalendarSearchEvent } from '../../types/search';
import { useLocale } from '../../locale/useLocale';
import {
  groupSearchResults,
  getSearchHeaderInfo,
  getDateObj,
  normalizeDate
} from '../../utils/searchUtils';

interface SearchDialogProps {
  isOpen: boolean;
  onClose: () => void;
  loading: boolean;
  results: CalendarSearchEvent[];
  keyword: string;
  onKeywordChange: (value: string) => void;
  onResultClick?: (event: CalendarSearchEvent) => void;
  emptyText?: string | Record<string, string>;
}

const SearchDialog: React.FC<SearchDialogProps> = ({
  isOpen,
  onClose,
  loading,
  results,
  keyword,
  onKeywordChange,
  onResultClick,
  emptyText,
}) => {
  const { t, locale } = useLocale();
  const inputRef = useRef<HTMLInputElement>(null);
  const dialogRef = useRef<HTMLDivElement>(null);

  const today = useMemo(() => normalizeDate(new Date()), []);

  // Group events by date (sorted)
  const groupedEvents = useMemo(() => {
    return groupSearchResults(results, today);
  }, [results, today]);

  // Helper to get time string
  const getTime = (d: any) => {
    return getDateObj(d);
  };

  const getEmptyText = () => {
    if (typeof emptyText === 'string') return emptyText;
    if (emptyText && typeof emptyText === 'object') {
      return emptyText[locale] || emptyText['en'] || 'No results found';
    }
    return t('noResults') || 'No results found';
  };

  // Focus input when dialog opens
  useEffect(() => {
    if (isOpen && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isOpen]);

  // Handle escape key to close
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  // Handle click outside to close
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dialogRef.current && !dialogRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] bg-black/30 dark:bg-black/50">
      <div
        ref={dialogRef}
        className="w-full max-w-lg bg-white dark:bg-gray-900 rounded-xl shadow-2xl dark:shadow-gray-900/50 overflow-hidden animate-in fade-in zoom-in-95 duration-150"
      >
        {/* Search Input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <Search className="w-5 h-5 text-gray-400" />
          <input
            ref={inputRef}
            type="text"
            placeholder={t('search') || 'Search events...'}
            value={keyword}
            onChange={(e) => onKeywordChange(e.target.value)}
            className="flex-1 text-base bg-transparent border-none outline-none text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400"
          />
          {keyword && (
            <button
              onClick={() => onKeywordChange('')}
              className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 rounded"
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <button
            onClick={onClose}
            className="p-1.5 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Results */}
        <div className="max-h-[60vh] overflow-y-auto">
          {loading ? (
            <div className="flex flex-col items-center justify-center h-40 text-gray-500">
              <Loader2 className="w-8 h-8 animate-spin mb-2" />
              <span className="text-sm">Searching...</span>
            </div>
          ) : results.length === 0 ? (
            keyword ? (
              <div className="flex flex-col items-center justify-center h-40 text-gray-500">
                <Search className="w-12 h-12 text-gray-300 dark:text-gray-600 mb-2" />
                <span className="text-sm">{getEmptyText()}</span>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-40 text-gray-400">
                <span className="text-sm">Type to search events</span>
              </div>
            )
          ) : (
            <div className="divide-y divide-gray-100 dark:divide-gray-800">
              {groupedEvents.map((group) => {
                const { title, colorClass } = getSearchHeaderInfo(group.date, today, locale, t);

                return (
                  <div key={group.date.getTime()}>
                    <h3 className={`px-4 py-2 text-xs font-semibold uppercase tracking-wide bg-gray-50 dark:bg-gray-800/50 ${colorClass}`}>
                      {title}
                    </h3>
                    <div className="divide-y divide-gray-100 dark:divide-gray-800">
                      {group.events.map((event) => {
                        const start = getTime(event.start);
                        const end = getTime(event.end);

                        const timeOpt: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' };
                        const startTimeStr = event.allDay
                          ? (t('allDay') || 'All Day')
                          : start.toLocaleTimeString(locale, timeOpt);
                        const endTimeStr = event.allDay
                          ? ''
                          : end.toLocaleTimeString(locale, timeOpt);

                        return (
                          <div
                            key={event.id}
                            className="flex items-center gap-3 px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer transition-colors"
                            onClick={() => {
                              onResultClick?.(event);
                              onClose();
                            }}
                          >
                            <div
                              className="w-1 h-10 rounded-full shrink-0"
                              style={{ backgroundColor: event.color || '#3b82f6' }}
                            />
                            <div className="flex-1 min-w-0">
                              <div className="font-medium text-gray-900 dark:text-gray-100 truncate">
                                {event.title}
                              </div>
                              <div className="text-sm text-gray-500 dark:text-gray-400">
                                {startTimeStr}
                                {endTimeStr && ` - ${endTimeStr}`}
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Footer hint */}
        <div className="px-4 py-2 text-xs text-gray-400 dark:text-gray-500 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700">
          Searching events within 1 year
        </div>
      </div>

      <style>{`
        @keyframes fadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        @keyframes zoomIn {
          from { transform: scale(0.95); opacity: 0; }
          to { transform: scale(1); opacity: 1; }
        }
        .animate-in {
          animation: fadeIn 0.15s ease-out, zoomIn 0.15s ease-out;
        }
      `}</style>
    </div>
  );
};

export default SearchDialog;
