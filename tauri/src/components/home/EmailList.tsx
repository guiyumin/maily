import { useEffect, useRef, useState, useCallback } from "react";
import { decode } from "he";
import { invoke } from "@tauri-apps/api/core";
import { Loader2, RefreshCw, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useLocale } from "@/lib/i18n";
import { TagList } from "@/components/tags/TagList";
import type { EmailTag } from "@/types/tags";

export interface Email {
  uid: number;
  from: string;
  subject: string;
  snippet: string;
  date: string;
  unread: boolean;
  tags?: EmailTag[];
}

interface EmailListProps {
  emails: Email[];
  selectedEmail: Email | null;
  onSelectEmail: (email: Email) => void;
  loading: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  mailboxName: string;
  total: number;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  account: string;
  mailbox: string;
}

interface SearchResult {
  uid: number;
  message_id: string;
  internal_date: string;
  from: string;
  to: string;
  subject: string;
  date: string;
  snippet: string;
  unread: boolean;
  has_attachments: boolean;
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days === 0) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  } else if (days === 1) {
    return "Yesterday";
  } else if (days < 7) {
    return date.toLocaleDateString([], { weekday: "short" });
  } else {
    return date.toLocaleDateString([], { month: "short", day: "numeric" });
  }
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
  let text = decode(snippet);

  // Remove invisible/zero-width/formatting characters
  text = text.replace(
    /[\u00AD\u034F\u061C\u115F\u1160\u17B4\u17B5\u180B-\u180E\u200B-\u200F\u202A-\u202E\u2060-\u206F\u2800\u3164\uFE00-\uFE0F\uFEFF\uFFA0\uFFF0-\uFFFF]/g,
    ""
  );

  // Collapse whitespace and trim
  return text.replace(/\s+/g, " ").trim();
}

export function EmailList({
  emails,
  selectedEmail,
  onSelectEmail,
  loading,
  refreshing,
  onRefresh,
  mailboxName,
  total,
  hasMore,
  loadingMore,
  onLoadMore,
  account,
  mailbox,
}: EmailListProps) {
  const { t } = useLocale();
  const unreadCount = emails.filter((e) => e.unread).length;
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Search state
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<Email[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Debounced search
  const performSearch = useCallback(async (query: string) => {
    if (!query.trim() || !account) {
      setSearchResults([]);
      setIsSearching(false);
      return;
    }

    setIsSearching(true);
    try {
      const results = await invoke<SearchResult[]>("search_emails_cmd", {
        account,
        mailbox,
        query: query.trim(),
        limit: 100,
      });

      // Convert SearchResult to Email format
      setSearchResults(
        results.map((r) => ({
          uid: r.uid,
          from: r.from,
          subject: r.subject,
          snippet: r.snippet,
          date: r.date,
          unread: r.unread,
        }))
      );
    } catch (err) {
      console.error("Search error:", err);
      setSearchResults([]);
    } finally {
      setIsSearching(false);
    }
  }, [account, mailbox]);

  // Handle search input change with debounce
  const handleSearchChange = useCallback((value: string) => {
    setSearchQuery(value);

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    if (!value.trim()) {
      setSearchResults([]);
      setIsSearching(false);
      return;
    }

    setIsSearching(true);
    searchTimeoutRef.current = setTimeout(() => {
      performSearch(value);
    }, 300);
  }, [performSearch]);

  // Clear search
  const clearSearch = useCallback(() => {
    setSearchQuery("");
    setSearchResults([]);
    setIsSearching(false);
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }
  }, []);

  // Reset search when account/mailbox changes
  useEffect(() => {
    clearSearch();
  }, [account, mailbox, clearSearch]);

  // Determine which emails to display
  const displayEmails = searchQuery.trim() ? searchResults : emails;
  const isSearchMode = searchQuery.trim().length > 0;

  // IntersectionObserver for infinite scroll (only when not searching)
  useEffect(() => {
    if (!hasMore || loadingMore || isSearchMode) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          onLoadMore();
        }
      },
      {
        threshold: 0.1,
        root: scrollContainerRef.current, // Observe within scroll container, not viewport
      }
    );

    if (loadMoreRef.current) {
      observer.observe(loadMoreRef.current);
    }

    return () => observer.disconnect();
  }, [hasMore, loadingMore, onLoadMore]);

  return (
    <div className="flex w-95 shrink-0 flex-col border-r bg-background">
      {/* Header */}
      <div className="flex flex-col gap-3 border-b p-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold">{mailboxName}</h2>
            <span className="text-sm text-muted-foreground">
              {emails.length === total
                ? `${emails.length} ${t("mail.emails")}`
                : `${emails.length} ${t("mail.of")} ${total} ${t("mail.emails")}`}
              {unreadCount > 0 && ` · ${unreadCount} ${t("mail.unread")}`}
            </span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={onRefresh}
            disabled={refreshing}
          >
            <RefreshCw
              className={cn("h-4 w-4", refreshing && "animate-spin")}
            />
          </Button>
        </div>

        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t("mail.search")}
            className="pl-9 pr-9"
            value={searchQuery}
            onChange={(e) => handleSearchChange(e.target.value)}
          />
          {searchQuery && (
            <Button
              variant="ghost"
              size="icon"
              className="absolute right-1 top-1 h-7 w-7"
              onClick={clearSearch}
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Email list */}
      <div ref={scrollContainerRef} className="scrollbar-thin flex-1 overflow-y-auto">
        {loading ? (
          <div className="space-y-4 p-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-4 w-1/3" />
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            ))}
          </div>
        ) : isSearching ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
            <Loader2 className="h-6 w-6 animate-spin" />
            <p>Searching...</p>
          </div>
        ) : isSearchMode && displayEmails.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
            <Search className="h-8 w-8 opacity-50" />
            <p>No results found</p>
          </div>
        ) : displayEmails.length === 0 ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-muted-foreground">
            <p>{t("mail.noEmails")}</p>
          </div>
        ) : (
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
        )}
      </div>
    </div>
  );
}
