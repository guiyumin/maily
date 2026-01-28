import React, { useState } from 'react';
import { CalendarType } from '../../../types';
import { useLocale } from '@calendar/locale';
import { Button } from '@/components/ui/button';

interface DeleteCalendarDialogProps {
  calendarId: string;
  calendarName: string;
  calendars: CalendarType[];
  step: 'initial' | 'confirm_delete';
  onStepChange: (step: 'initial' | 'confirm_delete') => void;
  onConfirmDelete: () => void;
  onCancel: () => void;
  onMergeSelect: (targetId: string) => void;
}

export const DeleteCalendarDialog: React.FC<DeleteCalendarDialogProps> = ({
  calendarId,
  calendarName,
  calendars,
  step,
  onStepChange,
  onConfirmDelete,
  onCancel,
  onMergeSelect,
}) => {
  const [showMergeDropdown, setShowMergeDropdown] = useState(false);
  const { t } = useLocale();

  return (
    <div className="fixed inset-0 z-100 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg p-6 shadow-xl bg-background">
        {step === 'initial' ? (
          <>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              {t('deleteCalendar', { calendarName })}
            </h2>
            <p className="mt-3 text-sm text-gray-600 dark:text-gray-300">
              {t('deleteCalendarMessage', { calendarName })}
            </p>
            <div className="mt-6 flex justify-between items-center">
              <div className="relative">
                <Button
                  variant="outline"
                  onClick={() => setShowMergeDropdown(!showMergeDropdown)}
                >
                  {t('merge')}
                </Button>
                {showMergeDropdown && (
                  <div className="absolute left-0 top-full mt-1 min-w-full w-max rounded-md border border-gray-200 bg-background shadow-lg dark:border-slate-700 z-10 max-h-60 overflow-y-auto">
                    {calendars
                      .filter(c => c.id !== calendarId)
                      .map(calendar => (
                        <div
                          key={calendar.id}
                          className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-slate-700 cursor-pointer"
                          onClick={() => {
                            onMergeSelect(calendar.id);
                            setShowMergeDropdown(false);
                          }}
                        >
                          <div
                            className="mr-2 h-3 w-3 rounded-sm shrink-0"
                            style={{ backgroundColor: calendar.colors.lineColor }}
                          />
                          <span className="whitespace-nowrap">{calendar.name || calendar.id}</span>
                        </div>
                      ))}
                  </div>
                )}
              </div>
              <div className="flex gap-3">
                <Button
                  variant="outline"
                  onClick={onCancel}
                >
                  {t('cancel')}
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => onStepChange('confirm_delete')}
                >
                  {t('delete')}
                </Button>
              </div>
            </div>
          </>
        ) : (
          <>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              {t('confirmDeleteTitle', { calendarName })}
            </h2>
            <p className="mt-3 text-sm text-gray-600 dark:text-gray-300">
              {t('confirmDeleteMessage')}
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <Button
                variant="ghost"
                onClick={onCancel}
              >
                {t('cancel')}
              </Button>
              <Button
                variant="destructive"
                onClick={onConfirmDelete}
              >
                {t('delete')}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};
