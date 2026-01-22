import React from "react";
import NavBox from "./NavBox";
import { Pin } from "lucide-react";
import { useLocale } from "@calendar/locale";

export type ViewHeaderType = "day" | "week" | "month" | "year";
export type ViewSwitcherMode = "buttons" | "select";

interface ViewHeaderProps {
  /** View type */
  viewType: ViewHeaderType;
  /** Current date */
  currentDate: Date;
  /** Previous period */
  onPrevious?: () => void;
  /** Next period */
  onNext?: () => void;
  /** Go to today */
  onToday?: () => void;
}

const ViewHeader: React.FC<ViewHeaderProps> = ({
  viewType,
  currentDate,
  onPrevious,
  onNext,
  onToday,
}) => {
  const { t } = useLocale();
  if (viewType === "day") {
    return <div className="p-2 h-7 w-full" />;
  }

  return (
    <div className="m-2 flex justify-between items-center">
      {/* Left side: Today button + Title */}
      <div className="flex items-center gap-3">
        {/* Today Button - always visible */}
        {onToday && (
          <button
            onClick={onToday}
            className="flex h-7 items-center gap-1.5 px-3 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-700 hover:border-gray-300 dark:hover:border-gray-500 transition-colors shadow-sm"
          >
            <Pin className="h-4 w-4" />
            <span>{t("today")}</span>
          </button>
        )}
      </div>

      {/* Right side: Navigation */}
      {onPrevious && onNext && (
        <div className="flex items-center gap-2">
          <NavBox
            handlePreviousMonth={onPrevious}
            handleNextMonth={onNext}
            viewType={viewType}
            currentDate={currentDate}
          />
        </div>
      )}
    </div>
  );
};

export default ViewHeader;
