import { useState, useCallback } from "react";
import { Plus, Settings, MoreHorizontal, GripVertical, Calendar } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { useLocale } from "@/lib/i18n";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
  DragStartEvent,
  DragOverlay,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
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

// Sortable account item for visible accounts
interface SortableAccountProps {
  account: Account;
  originalIndex: number;
  isSelected: boolean;
  unread: number;
  onSelect: (name: string) => void;
  isDragging?: boolean;
  canDrag: boolean;
}

function SortableVisibleAccount({
  account,
  originalIndex,
  isSelected,
  unread,
  onSelect,
  isDragging: isCurrentlyDragging,
  canDrag,
}: SortableAccountProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: account.name, disabled: !canDrag });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          ref={setNodeRef}
          style={style}
          {...attributes}
          {...(canDrag ? listeners : {})}
          onClick={() => onSelect(account.name)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              onSelect(account.name);
            }
          }}
          className={cn(
            "relative group select-none",
            canDrag && "cursor-grab active:cursor-grabbing",
            "relative size-8 rounded-full transition-all",
            isSelected
              ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
              : "opacity-60 hover:opacity-100",
            (isDragging || isCurrentlyDragging) && "opacity-50 z-50"
          )}
        >
          <Avatar className="size-8 pointer-events-none">
            <AvatarFallback className={cn(getAccountColor(originalIndex), "text-xs")}>
              {getInitials(account.name)}
            </AvatarFallback>
          </Avatar>
          {unread > 0 && (
            <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-0.5 text-[9px] font-medium text-destructive-foreground pointer-events-none">
              {unread > 99 ? "99+" : unread}
            </span>
          )}
          {/* Drag handle indicator on hover */}
          {canDrag && (
            <div className="absolute -left-1 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-60 transition-opacity pointer-events-none">
              <GripVertical className="h-3 w-3 text-muted-foreground" />
            </div>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent side="right">
        {account.name}
        {unread > 0 && ` (${unread} unread)`}
        {canDrag && <span className="block text-xs text-muted-foreground">Drag to reorder</span>}
      </TooltipContent>
    </Tooltip>
  );
}

// Sortable account item for overflow menu
function SortableOverflowAccount({
  account,
  originalIndex,
  unread,
  onSelect,
  isDragging: isCurrentlyDragging,
  canDrag,
}: SortableAccountProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: account.name, disabled: !canDrag });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...(canDrag ? listeners : {})}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          onSelect(account.name);
        }
      }}
      className={cn(
        "flex w-full items-center gap-3 rounded-md px-2 py-2 text-left hover:bg-muted transition-all select-none",
        canDrag && "cursor-grab active:cursor-grabbing",
        (isDragging || isCurrentlyDragging) && "opacity-50"
      )}
      onClick={() => {
        if (!isDragging && !isCurrentlyDragging) {
          onSelect(account.name);
        }
      }}
    >
      {canDrag && (
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
}

// Drag overlay content
function DragOverlayContent({
  account,
  originalIndex,
  unread,
  variant,
}: {
  account: Account;
  originalIndex: number;
  unread: number;
  variant: "visible" | "overflow";
}) {
  if (variant === "visible") {
    return (
      <div className="relative size-8 rounded-full ring-2 ring-primary ring-offset-2 ring-offset-background cursor-grabbing">
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
      </div>
    );
  }

  return (
    <div className="flex w-56 items-center gap-3 rounded-md bg-background px-2 py-2 shadow-lg border cursor-grabbing">
      <GripVertical className="h-4 w-4 text-muted-foreground/50 shrink-0" />
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
      </div>
    </div>
  );
}

export function AccountRail({
  accounts,
  selectedAccount,
  onSelectAccount,
  unreadCounts = {},
  accountOrder = [],
  onOrderChange,
}: AccountRailProps) {
  const { t } = useLocale();
  const [overflowOpen, setOverflowOpen] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8, // 8px movement required before drag starts
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  // Sort accounts based on accountOrder (if provided), keeping unordered accounts at the end
  const sortedAccounts = [...accounts].sort((a, b) => {
    const aIndex = accountOrder.indexOf(a.name);
    const bIndex = accountOrder.indexOf(b.name);

    if (aIndex !== -1 && bIndex !== -1) {
      return aIndex - bIndex;
    }
    if (aIndex !== -1) return -1;
    if (bIndex !== -1) return 1;
    return accounts.indexOf(a) - accounts.indexOf(b);
  });

  // Check if selected account is in overflow
  const selectedIndex = sortedAccounts.findIndex((a) => a.name === selectedAccount);
  const selectedInOverflow = selectedIndex >= MAX_VISIBLE_ACCOUNTS;

  // Determine visible accounts: first 3, or swap in selected if it's in overflow
  let visibleAccounts: Account[];
  let overflowAccounts: Account[];

  if (selectedInOverflow && sortedAccounts.length > MAX_VISIBLE_ACCOUNTS) {
    visibleAccounts = [
      ...sortedAccounts.slice(0, MAX_VISIBLE_ACCOUNTS - 1),
      sortedAccounts[selectedIndex],
    ];
    overflowAccounts = sortedAccounts.filter(
      (_, i) => i >= MAX_VISIBLE_ACCOUNTS - 1 && i !== selectedIndex
    );
  } else {
    visibleAccounts = sortedAccounts.slice(0, MAX_VISIBLE_ACCOUNTS);
    overflowAccounts = sortedAccounts.slice(MAX_VISIBLE_ACCOUNTS);
  }

  const handleOverflowSelect = useCallback((name: string) => {
    onSelectAccount(name);
    setOverflowOpen(false);
  }, [onSelectAccount]);

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveId(event.active.id as string);
  }, []);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    setActiveId(null);

    if (!over || active.id === over.id || !onOrderChange) {
      return;
    }

    const currentOrder = sortedAccounts.map((a) => a.name);
    const oldIndex = currentOrder.indexOf(active.id as string);
    const newIndex = currentOrder.indexOf(over.id as string);

    if (oldIndex === -1 || newIndex === -1) return;

    const newOrder = arrayMove(currentOrder, oldIndex, newIndex);
    onOrderChange(newOrder);
    setOverflowOpen(false);
  }, [sortedAccounts, onOrderChange]);

  const activeAccount = activeId
    ? sortedAccounts.find((a) => a.name === activeId)
    : null;
  const activeOriginalIndex = activeAccount
    ? sortedAccounts.findIndex((a) => a.name === activeAccount.name)
    : 0;
  const activeInOverflow = activeAccount
    ? overflowAccounts.some((a) => a.name === activeAccount.name)
    : false;

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <aside className="flex w-14 shrink-0 flex-col items-center border-r bg-muted/30 py-3">
        <div className="flex flex-col items-center gap-2">
          <SortableContext
            items={sortedAccounts.map((a) => a.name)}
            strategy={verticalListSortingStrategy}
          >
            {visibleAccounts.map((account) => {
              const originalIndex = sortedAccounts.findIndex((a) => a.name === account.name);
              const unread = unreadCounts[account.name] || 0;
              return (
                <SortableVisibleAccount
                  key={account.name}
                  account={account}
                  originalIndex={originalIndex}
                  isSelected={selectedAccount === account.name}
                  unread={unread}
                  onSelect={onSelectAccount}
                  isDragging={activeId === account.name}
                  canDrag={!!onOrderChange}
                />
              );
            })}

            {/* Overflow menu for additional accounts */}
            {overflowAccounts.length > 0 && (
              <Popover open={overflowOpen} onOpenChange={setOverflowOpen} modal={false}>
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
                <PopoverContent
                  side="right"
                  className="w-64 p-2"
                  onPointerDownOutside={(e) => {
                    if (activeId) {
                      e.preventDefault();
                    }
                  }}
                  onInteractOutside={(e) => {
                    if (activeId) {
                      e.preventDefault();
                    }
                  }}
                >
                  <div className="space-y-1">
                    <p className="px-2 py-1 text-xs font-medium text-muted-foreground">
                      More accounts {onOrderChange && <span className="text-muted-foreground/60">· Drag to reorder</span>}
                    </p>
                    {overflowAccounts.map((account) => {
                      const originalIndex = sortedAccounts.findIndex((a) => a.name === account.name);
                      const unread = unreadCounts[account.name] || 0;
                      return (
                        <SortableOverflowAccount
                          key={account.name}
                          account={account}
                          originalIndex={originalIndex}
                          isSelected={selectedAccount === account.name}
                          unread={unread}
                          onSelect={handleOverflowSelect}
                          isDragging={activeId === account.name}
                          canDrag={!!onOrderChange}
                        />
                      );
                    })}
                  </div>
                </PopoverContent>
              </Popover>
            )}
          </SortableContext>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                className="size-8 rounded-full"
                asChild
              >
                <Link to="/settings" search={{ section: "accounts" }}>
                  <Plus className="h-3.5 w-3.5" />
                </Link>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">{t("settings.accounts.addAccount")}</TooltipContent>
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
            <TooltipContent side="right">{t("nav.calendar")}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="size-8 rounded-full" asChild>
                <Link to="/settings">
                  <Settings className="h-3.5 w-3.5" />
                </Link>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">{t("nav.settings")}</TooltipContent>
          </Tooltip>
        </div>
      </aside>

      <DragOverlay>
        {activeAccount && (
          <DragOverlayContent
            account={activeAccount}
            originalIndex={activeOriginalIndex}
            unread={unreadCounts[activeAccount.name] || 0}
            variant={activeInOverflow ? "overflow" : "visible"}
          />
        )}
      </DragOverlay>
    </DndContext>
  );
}
