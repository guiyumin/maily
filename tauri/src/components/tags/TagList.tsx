import { cn } from "@/lib/utils";
import type { EmailTag } from "@/types/tags";

interface TagListProps {
  tags: EmailTag[];
  maxDisplay?: number;
  className?: string;
}

export function TagList({ tags, maxDisplay = 3, className }: TagListProps) {
  if (!tags || tags.length === 0) {
    return null;
  }

  const displayTags = tags.slice(0, maxDisplay);
  const remainingCount = tags.length - maxDisplay;

  return (
    <div className={cn("flex flex-wrap items-center gap-1", className)}>
      {displayTags.map((tag) => (
        <span
          key={tag.tag_id}
          className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium text-white"
          style={{ backgroundColor: tag.tag_color }}
        >
          {tag.tag_name}
        </span>
      ))}
      {remainingCount > 0 && (
        <span className="text-[10px] text-muted-foreground">
          +{remainingCount}
        </span>
      )}
    </div>
  );
}
