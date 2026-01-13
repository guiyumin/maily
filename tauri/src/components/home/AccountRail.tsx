import { Plus, Settings } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface Account {
  name: string;
  provider: string;
}

interface AccountRailProps {
  accounts: Account[];
  selectedAccount: string;
  onSelectAccount: (name: string) => void;
}

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
}: AccountRailProps) {
  return (
    <aside className="flex w-[72px] shrink-0 flex-col items-center border-r bg-muted/30 py-4">
      <div className="flex flex-col items-center gap-3">
        {accounts.map((account, index) => (
          <Tooltip key={account.name}>
            <TooltipTrigger asChild>
              <button
                onClick={() => onSelectAccount(account.name)}
                className={`size-10 rounded-full transition-all ${
                  selectedAccount === account.name
                    ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
                    : "opacity-60 hover:opacity-100"
                }`}
              >
                <Avatar className="size-10">
                  <AvatarFallback className={getAccountColor(index)}>
                    {getInitials(account.name)}
                  </AvatarFallback>
                </Avatar>
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">{account.name}</TooltipContent>
          </Tooltip>
        ))}

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className="size-10 rounded-full"
              asChild
            >
              <Link to="/settings">
                <Plus className="h-4 w-4" />
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Add account</TooltipContent>
        </Tooltip>
      </div>

      <div className="mt-auto">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="rounded-full" asChild>
              <Link to="/settings">
                <Settings className="h-4 w-4" />
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">Settings</TooltipContent>
        </Tooltip>
      </div>
    </aside>
  );
}
