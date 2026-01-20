import { useState, useCallback, useRef } from "react";
import { Plus, Settings, MoreHorizontal, GripVertical, Calendar } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

interface Account {
  name: string;
  provider: string;
}

interface AccountRailProps {
  accounts: Account[];
  selectedAccount: string;
  onSelectAccount: (name: string) => void;
  unreadCounts?: Record<string, number>;
  accountOrder?: string[];
  onOrderChange?: (order: string[]) => void;
}

const MAX_VISIBLE_ACCOUNTS = 3;

function getInitials(name: string): string {
  const parts = name.split("@")[0].split(/[._-]/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}

function getAccountColor(index: number): string {
  const colors = [
    "bg-violet-600",
    "bg-blue-600",
    "bg-emerald-600",
    "bg-amber-600",
    "bg-rose-600",
  ];
  return colors[index % colors.length];
}

export function AccountRail({
  accounts,
  selectedAccount,
  onSelectAccount,
  unreadCounts = {},
  accountOrder = [],
  onOrderChange,
}: AccountRailProps) {
  const [overflowOpen, setOverflowOpen] = useState(false);
  const [draggedAccount, setDraggedAccount] = useState<string | null>(null);
  const [dragOverAccount, setDragOverAccount] = useState<string | null>(null);
  const dragCounter = useRef(0);

  // Sort accounts based on accountOrder (if provided), keeping unordered accounts at the end
  const sortedAccounts = [...accounts].sort((a, b) => {
    const aIndex = accountOrder.indexOf(a.name);
    const bIndex = accountOrder.indexOf(b.name);

    // If both are in order, sort by their order position
    if (aIndex !== -1 && bIndex !== -1) {
      return aIndex - bIndex;
    }
    // If only a is in order, a comes first
    if (aIndex !== -1) return -1;
    // If only b is in order, b comes first
    if (bIndex !== -1) return 1;
    // If neither is in order, maintain original order
    return accounts.indexOf(a) - accounts.indexOf(b);
  });

  // Check if selected account is in overflow
  const selectedIndex = sortedAccounts.findIndex((a) => a.name === selectedAccount);
  const selectedInOverflow = selectedIndex >= MAX_VISIBLE_ACCOUNTS;

  // Determine visible accounts: first 3, or swap in selected if it's in overflow
  let visibleAccounts: Account[];
  let overflowAccounts: Account[];

  if (selectedInOverflow && sortedAccounts.length > MAX_VISIBLE_ACCOUNTS) {
    // Show first 2 + selected account
    visibleAccounts = [
      ...sortedAccounts.slice(0, MAX_VISIBLE_ACCOUNTS - 1),
      sortedAccounts[selectedIndex],
    ];
    // Overflow contains everything else
    overflowAccounts = sortedAccounts.filter(
      (_, i) => i >= MAX_VISIBLE_ACCOUNTS - 1 && i !== selectedIndex
    );
  } else {
    visibleAccounts = sortedAccounts.slice(0, MAX_VISIBLE_ACCOUNTS);
    overflowAccounts = sortedAccounts.slice(MAX_VISIBLE_ACCOUNTS);
  }

  const handleOverflowSelect = (name: string) => {
    onSelectAccount(name);
    setOverflowOpen(false);
  };

  // Drag and drop handlers
  const handleDragStart = useCallback((e: React.DragEvent, accountName: string) => {
    setDraggedAccount(accountName);
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", accountName);
    // Add a slight delay to allow the drag image to be created
    requestAnimationFrame(() => {
      (e.target as HTMLElement).style.opacity = "0.5";
    });
  }, []);

  const handleDragEnd = useCallback((e: React.DragEvent) => {
    setDraggedAccount(null);
    setDragOverAccount(null);
    dragCounter.current = 0;
    (e.target as HTMLElement).style.opacity = "1";
  }, []);

  const handleDragEnter = useCallback((e: React.DragEvent, accountName: string) => {
    e.preventDefault();
    dragCounter.current++;
    if (accountName !== draggedAccount) {
      setDragOverAccount(accountName);
    }
  }, [draggedAccount]);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    dragCounter.current--;
    if (dragCounter.current === 0) {
      setDragOverAccount(null);
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  }, []);

  const handleDrop = useCallback((e: React.DragEvent, targetAccountName: string) => {
    e.preventDefault();
    setDragOverAccount(null);
    dragCounter.current = 0;

    if (!draggedAccount || draggedAccount === targetAccountName || !onOrderChange) {
      return;
    }

    // Create new order
    const currentOrder = sortedAccounts.map(a => a.name);
    const draggedIndex = currentOrder.indexOf(draggedAccount);
    const targetIndex = currentOrder.indexOf(targetAccountName);

    if (draggedIndex === -1 || targetIndex === -1) return;

    // Remove dragged item and insert at target position
    const newOrder = [...currentOrder];
    newOrder.splice(draggedIndex, 1);
    newOrder.splice(targetIndex, 0, draggedAccount);

    onOrderChange(newOrder);
    setDraggedAccount(null);
  }, [draggedAccount, sortedAccounts, onOrderChange]);

  return (
    <aside className="flex w-14 shrink-0 flex-col items-center border-r bg-muted/30 py-3">
      <div className="flex flex-col items-center gap-2">
        {visibleAccounts.map((account) => {
          const originalIndex = sortedAccounts.findIndex((a) => a.name === account.name);
          const unread = unreadCounts[account.name] || 0;
          const isDragging = draggedAccount === account.name;
          const isDragOver = dragOverAccount === account.name;
          return (
            <Tooltip key={account.name}>
              <TooltipTrigger asChild>
                <div
                  draggable={!!onOrderChange}
                  onDragStart={(e) => handleDragStart(e, account.name)}
                  onDragEnd={handleDragEnd}
                  onDragEnter={(e) => handleDragEnter(e, account.name)}
                  onDragLeave={handleDragLeave}
                  onDragOver={handleDragOver}
                  onDrop={(e) => handleDrop(e, account.name)}
                  className={cn(
                    "relative group",
                    onOrderChange && "cursor-grab active:cursor-grabbing"
                  )}
                >
                  <button
                    onClick={() => onSelectAccount(account.name)}
                    className={cn(
                      "relative size-8 rounded-full transition-all",
                      selectedAccount === account.name
                        ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
                        : "opacity-60 hover:opacity-100",
                      isDragging && "opacity-50",
                      isDragOver && "ring-2 ring-primary ring-offset-2 ring-offset-background scale-110"
                    )}
                  >
                    <Avatar className="size-8">
                      <AvatarFallback className={cn(getAccountColor(originalIndex), "text-xs")}>
                        {getInitials(account.name)}
                      </AvatarFallback>
                    </Avatar>
                    {unread > 0 && (
                      <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-medium text-destructive-foreground">
                        {unread > 99 ? "99+" : unread}
                      </span>
                    )}
                  </button>
                  {/* Drag handle indicator on hover */}
                  {onOrderChange && (
                    <div className="absolute -left-1 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-60 transition-opacity pointer-events-none">
                      <GripVertical className="h-3 w-3 text-muted-foreground" />
                    </div>
                  )}
                </div>
              </TooltipTrigger>
              <TooltipContent side="right">
                {account.name}
                {unread > 0 && ` (${unread} unread)`}
                {onOrderChange && <span className="block text-xs text-muted-foreground">Drag to reorder</span>}
              </TooltipContent>
            </Tooltip>
          );
        })}

        {/* Overflow menu for additional accounts */}
        {overflowAccounts.length > 0 && (
          <Popover open={overflowOpen} onOpenChange={setOverflowOpen}>
            <Tooltip>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <button className="flex size-8 items-center justify-center rounded-full bg-muted/50 text-muted-foreground transition-all hover:bg-muted hover:text-foreground">
                    <MoreHorizontal className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="right">
                {overflowAccounts.length} more account{overflowAccounts.length > 1 ? "s" : ""}
              </TooltipContent>
            </Tooltip>
            <PopoverContent side="right" className="w-64 p-2">
              <div className="space-y-1">
                <p className="px-2 py-1 text-xs font-medium text-muted-foreground">
                  More accounts {onOrderChange && <span className="text-muted-foreground/60">· Drag to reorder</span>}
                </p>
                {overflowAccounts.map((account) => {
                  const originalIndex = sortedAccounts.findIndex((a) => a.name === account.name);
                  const unread = unreadCounts[account.name] || 0;
                  const isDragging = draggedAccount === account.name;
                  const isDragOver = dragOverAccount === account.name;
                  return (
                    <div
                      key={account.name}
                      draggable={!!onOrderChange}
                      onDragStart={(e) => handleDragStart(e, account.name)}
                      onDragEnd={handleDragEnd}
                      onDragEnter={(e) => handleDragEnter(e, account.name)}
                      onDragLeave={handleDragLeave}
                      onDragOver={handleDragOver}
                      onDrop={(e) => handleDrop(e, account.name)}
                      className={cn(
                        "flex w-full items-center gap-3 rounded-md px-2 py-2 text-left hover:bg-muted transition-all",
                        onOrderChange && "cursor-grab active:cursor-grabbing",
                        isDragging && "opacity-50",
                        isDragOver && "bg-muted ring-1 ring-primary"
                      )}
                      onClick={() => handleOverflowSelect(account.name)}
                    >
                      {onOrderChange && (
                        <GripVertical className="h-4 w-4 text-muted-foreground/50 shrink-0" />
                      )}
                      <div className="relative">
                        <Avatar className="size-8">
                          <AvatarFallback className={getAccountColor(originalIndex)}>
                            {getInitials(account.name)}
                          </AvatarFallback>
                        </Avatar>
                        {unread > 0 && (
                          <span className="absolute -top-1 -right-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-medium text-destructive-foreground">
                            {unread > 99 ? "99+" : unread}
                          </span>
                        )}
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">
                          {account.name}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {account.provider}
                          {unread > 0 && ` · ${unread} unread`}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </PopoverContent>
          </Popover>
        )}

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className="size-8 rounded-full"
              asChild
            >
              <Link to="/settings">
                <Plus className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Add account</TooltipContent>
        </Tooltip>
      </div>

      <div className="mt-auto flex flex-col gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="size-8 rounded-full" asChild>
              <Link to="/calendar">
                <Calendar className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Calendar</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="size-8 rounded-full" asChild>
              <Link to="/settings">
                <Settings className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Settings</TooltipContent>
        </Tooltip>
      </div>
    </aside>
  );
}
