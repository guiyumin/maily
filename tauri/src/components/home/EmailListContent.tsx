import { Loader2, Search } from "lucide-react";
import { format, isToday, isYesterday, differenceInDays } from "date-fns";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { TagList } from "@/components/tags/TagList";
import type { TranslationKey } from "@/lib/i18n";
import type { Email } from "./EmailList";

function formatDate(dateString: string): string {
  const date = new Date(dateString);

  if (isToday(date)) {
    return format(date, "p"); // e.g., "2:30 PM"
  }
  if (isYesterday(date)) {
    return "Yesterday";
  }
  if (differenceInDays(new Date(), date) < 7) {
    return format(date, "EEE"); // e.g., "Mon"
  }
  return format(date, "MMM d"); // e.g., "Jan 26"
}

function extractName(from: string): string {
  // Extract name from "Name <email>" format
  const match = from.match(/^([^<]+)</);
  if (match) {
    return match[1].trim();
  }
  // Extract email username if no name
  const emailMatch = from.match(/([^@]+)@/);
  if (emailMatch) {
    return emailMatch[1];
  }
  return from;
}

function sanitizeSnippet(snippet: string | undefined): string {
  if (!snippet) return "";

  // Decode HTML entities (handles &#8199; &#847; etc.)
  // Note: Using dynamic import to avoid issues with 'he' module
  let text = snippet;

  // Remove invisible/zero-width/formatting characters
  text = text.replace(
    /[\u00AD\u034F\u061C\u115F\u1160\u17B4\u17B5\u180B-\u180E\u200B-\u200F\u202A-\u202E\u2060-\u206F\u2800\u3164\uFE00-\uFE0F\uFEFF\uFFA0\uFFF0-\uFFFF]/g,
    ""
  );

  // Collapse whitespace and trim
  return text.replace(/\s+/g, " ").trim();
}

interface EmailListContentProps {
  loading: boolean;
  isSearching: boolean;
  isSearchMode: boolean;
  searchResults: Email[];
  searchQuery: string;
  displayEmails: Email[];
  selectedEmail: Email | null;
  onSelectEmail: (email: Email) => void;
  hasMore: boolean;
  loadingMore: boolean;
  loadMoreRef: React.RefObject<HTMLDivElement | null>;
  total: number;
  emails: Email[];
  t: (key: TranslationKey) => string;
}

export function EmailListContent({
  loading,
  isSearching,
  isSearchMode,
  searchResults,
  searchQuery,
  displayEmails,
  selectedEmail,
  onSelectEmail,
  hasMore,
  loadingMore,
  loadMoreRef,
  total,
  emails,
  t,
}: EmailListContentProps) {
  if (loading) {
    return (
      <div className="space-y-4 p-4">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="space-y-2">
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ))}
      </div>
    );
  }

  if (isSearching && searchResults.length === 0) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
        <Loader2 className="h-6 w-6 animate-spin" />
        <p>Searching...</p>
      </div>
    );
  }

  if (isSearchMode && displayEmails.length === 0) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
        <Search className="h-8 w-8 opacity-50" />
        <p>No results found</p>
      </div>
    );
  }

  if (displayEmails.length === 0) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
        <p>{t("mail.noEmails")}</p>
      </div>
    );
  }

  return (
    <div className="EmailList flex min-w-0 flex-col">
      {isSearchMode && (
        <div className="px-3 py-2 text-xs text-muted-foreground border-b">
          {displayEmails.length} {displayEmails.length === 1 ? "result" : "results"} for "{searchQuery}"
        </div>
      )}
      {displayEmails.map((email) => {
        const isSelected = selectedEmail?.uid === email.uid;

        return (
          <button
            key={email.uid}
            onClick={() => onSelectEmail(email)}
            className={cn(
              "flex w-full flex-col gap-1 border-b border-l-4 p-3 text-left transition-colors",
              isSelected
                ? "border-l-primary bg-accent"
                : "border-l-transparent hover:bg-accent/50",
              email.unread && !isSelected && "bg-accent/30"
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <span
                className={cn(
                  "truncate text-sm",
                  email.unread ? "font-semibold" : "text-muted-foreground"
                )}
              >
                {extractName(email.from)}
              </span>
              <span className="shrink-0 text-xs text-muted-foreground">
                {formatDate(email.date)}
              </span>
            </div>

            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "truncate text-sm",
                  email.unread && "font-medium"
                )}
              >
                {email.subject || "(No subject)"}
              </span>
              {email.unread && (
                <span className="h-2 w-2 shrink-0 rounded-full bg-primary" />
              )}
            </div>

            <span className="line-clamp-2 block text-xs text-muted-foreground">
              {sanitizeSnippet(email.snippet) || "\u00A0"}
            </span>

            {email.tags && email.tags.length > 0 && (
              <TagList tags={email.tags} maxDisplay={4} />
            )}
          </button>
        );
      })}

      {/* Infinite scroll sentinel (only when not searching) */}
      {hasMore && !isSearchMode && (
        <div
          ref={loadMoreRef}
          className="flex items-center justify-center p-4 text-sm text-muted-foreground"
        >
          {loadingMore ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t("common.loading")}
            </>
          ) : (
            <span>{total - emails.length} {t("mail.moreEmails")}</span>
          )}
        </div>
      )}
    </div>
  );
}
