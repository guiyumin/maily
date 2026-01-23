import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import type { EmailTag } from "@/types/tags";

interface TagBadgeProps {
  tag: EmailTag;
  onRemove?: () => void;
  size?: "sm" | "md";
  className?: string;
}

export function TagBadge({
  tag,
  onRemove,
  size = "sm",
  className,
}: TagBadgeProps) {
  const sizeClasses = {
    sm: "text-[10px] px-1.5 py-0.5 gap-0.5",
    md: "text-xs px-2 py-1 gap-1",
  };

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full font-medium text-white",
        sizeClasses[size],
        className
      )}
      style={{ backgroundColor: tag.tag_color }}
    >
      {tag.tag_name}
      {onRemove && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="hover:bg-white/20 rounded-full p-0.5 transition-colors"
        >
          <X className={size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3"} />
        </button>
      )}
    </span>
  );
}
