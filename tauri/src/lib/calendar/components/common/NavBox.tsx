import { ChevronLeft, ChevronRight } from "lucide-react";
import { useLocale } from "@calendar/locale";
import { ViewHeaderType } from "./ViewHeader";

interface Props {
  handlePreviousMonth: () => void;
  handleNextMonth: () => void;
  viewType: ViewHeaderType;
  currentDate: Date;
}

// Get ISO week number
function getWeekNumber(date: Date): number {
  const d = new Date(
    Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()),
  );
  const dayNum = d.getUTCDay() || 7;
  d.setUTCDate(d.getUTCDate() + 4 - dayNum);
  const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
  return Math.ceil(((d.getTime() - yearStart.getTime()) / 86400000 + 1) / 7);
}

const NavBox: React.FC<Props> = ({
  handlePreviousMonth,
  handleNextMonth,
  viewType,
  currentDate,
}) => {
  const { locale } = useLocale();

  // Return empty placeholder for day view to maintain layout
  // Width: 2 buttons (w-7 each) + text (min-w-28) + gaps (gap-1 * 2) ≈ 168px
  if (viewType === "day") {
    return null;
  }

  // Get display text based on view type
  const getDisplayText = (): string => {
    const monthYear = currentDate.toLocaleDateString(locale, {
      month: "short",
      year: "numeric",
    });

    if (viewType === "week") {
      const weekNum = getWeekNumber(currentDate);
      return `${monthYear} · W${weekNum}`;
    }

    // Month or Year view - show month and year
    return monthYear;
  };

  return (
    <div className="flex items-center gap-1">
      <button
        className="calendar-nav-button group relative inline-flex items-center justify-center w-7 h-7 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 hover:border-gray-300 dark:hover:border-gray-500 active:bg-gray-100 dark:active:bg-gray-600 transition-all duration-150 shadow-sm hover:shadow focus:outline-none"
        onClick={handlePreviousMonth}
        aria-label="Previous"
      >
        <ChevronLeft className="h-4 w-4 transition-transform group-hover:scale-110" />
      </button>
      <div className="inline-flex items-center justify-center px-3 h-7 text-sm font-medium text-gray-700 dark:text-gray-300 min-w-28">
        {getDisplayText()}
      </div>
      <button
        className="calendar-nav-button group relative inline-flex items-center justify-center w-7 h-7 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 hover:border-gray-300 dark:hover:border-gray-500 active:bg-gray-100 dark:active:bg-gray-600 transition-all duration-150 shadow-sm hover:shadow focus:outline-none"
        onClick={handleNextMonth}
        aria-label="Next"
      >
        <ChevronRight className="h-4 w-4 transition-transform group-hover:scale-110" />
      </button>
    </div>
  );
};

export default NavBox;
