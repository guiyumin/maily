import { useEffect, useRef, useState, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { RefreshCw, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useLocale } from "@/lib/i18n";
import { EmailListContent } from "./EmailListContent";
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
              {`${emails.length} ${t("mail.of")} ${total} ${t("mail.emails")}`}
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
        <EmailListContent
          loading={loading}
          isSearching={isSearching}
          isSearchMode={isSearchMode}
          searchResults={searchResults}
          searchQuery={searchQuery}
          displayEmails={displayEmails}
          selectedEmail={selectedEmail}
          onSelectEmail={onSelectEmail}
          hasMore={hasMore}
          loadingMore={loadingMore}
          loadMoreRef={loadMoreRef}
          total={total}
          emails={emails}
          t={t}
        />
      </div>
    </div>
  );
}
