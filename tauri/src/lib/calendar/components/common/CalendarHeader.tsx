import React from "react";
import { CalendarApp, ViewType } from "../../types";
import ViewSwitcher from "./ViewSwitcher";
import { Plus, Search } from "lucide-react";
import { ViewSwitcherMode } from "./ViewHeader";
import { useLocale } from "@calendar/locale";

interface CalendarHeaderProps {
  calendar: CalendarApp;
  switcherMode?: ViewSwitcherMode;
  onCreateEvent?: () => void;
  onSearchClick?: () => void;
}

const CalendarHeader: React.FC<CalendarHeaderProps> = ({
  calendar,
  switcherMode = "buttons",
  onCreateEvent,
  onSearchClick,
}) => {
  const { t } = useLocale();
  const isSwitcherCentered = switcherMode === "buttons";
  const isDayView = calendar.state.currentView === ViewType.DAY;

  return (
    <div className="flex items-center justify-between p-2 bg-white dark:bg-gray-900 transition-colors duration-200 shrink-0">
      {/* Left Section: New Event Button */}
      <div className="flex items-center min-w-50">
        {onCreateEvent && (
          <button
            onClick={onCreateEvent}
            className="flex h-7 items-center gap-1.5 px-3 rounded-md bg-primary text-white text-sm font-medium hover:bg-primary/90 transition-colors shadow-sm"
          >
            <Plus className="h-4 w-4" />
            <span>{t("newEvent")}</span>
          </button>
        )}
      </div>

      {/* Middle Section: ViewSwitcher (if mode is buttons) */}
      <div className="flex-1 flex justify-center">
        {isSwitcherCentered && (
          <ViewSwitcher mode={switcherMode} calendar={calendar} />
        )}
      </div>

      {/* Right Section: Search Button, ViewSwitcher (if select) */}
      <div className="flex items-center justify-end gap-3 min-w-50 pb-1 h-6">
        {onSearchClick && (
          <button
            onClick={onSearchClick}
            className="hidden md:flex items-center gap-2 h-7 px-3 text-sm text-gray-500 dark:text-gray-400 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
          >
            <Search className="h-4 w-4" />
            <span>{t("search")}</span>
            <kbd className="hidden lg:inline-flex h-5 items-center gap-1 rounded border border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 px-1.5 text-[10px] font-medium text-gray-500 dark:text-gray-400">
              ⌘K
            </kbd>
          </button>
        )}

        {!isSwitcherCentered && (
          <ViewSwitcher mode={switcherMode} calendar={calendar} />
        )}
      </div>
    </div>
  );
};

export default CalendarHeader;
