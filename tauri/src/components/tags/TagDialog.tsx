import { useState, useEffect } from "react";
import { Sparkles, Plus, Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TagBadge } from "./TagBadge";
import {
  listTags,
  createTag,
  addEmailTag,
  removeEmailTag,
} from "@/lib/tags";
import type { Tag, EmailTag } from "@/types/tags";
import { TAG_COLORS } from "@/types/tags";
import { useLocale } from "@/lib/i18n";

interface TagDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account: string;
  mailbox: string;
  uid: number;
  tags: EmailTag[];
  onTagsChange: (tags: EmailTag[]) => void;
  emailContext?: {
    from: string;
    subject: string;
    bodyText: string;
  };
}

export function TagDialog({
  open,
  onOpenChange,
  account,
  mailbox,
  uid,
  tags,
  onTagsChange,
  emailContext,
}: TagDialogProps) {
  const { t } = useLocale();
  const [allTags, setAllTags] = useState<Tag[]>([]);
  const [newTagName, setNewTagName] = useState("");
  const [loading, setLoading] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);

  // Load all available tags when dialog opens
  useEffect(() => {
    if (open) {
      listTags().then(setAllTags).catch(console.error);
    }
  }, [open]);

  // Get tags not currently assigned to this email
  const availableTags = allTags.filter(
    (tag) => !tags.some((et) => et.tag_id === tag.id)
  );

  const handleAddExistingTag = async (tag: Tag) => {
    setLoading(true);
    try {
      await addEmailTag(account, mailbox, uid, tag.id, false);
      const newEmailTag: EmailTag = {
        tag_id: tag.id,
        tag_name: tag.name,
        tag_color: tag.color,
        auto_generated: false,
      };
      onTagsChange([...tags, newEmailTag]);
    } catch (error) {
      console.error("Failed to add tag:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateAndAddTag = async () => {
    if (!newTagName.trim()) return;

    setLoading(true);
    try {
      // Pick a random color from the palette
      const color = TAG_COLORS[Math.floor(Math.random() * TAG_COLORS.length)];
      const tag = await createTag(newTagName.trim(), color);

      // Add to email
      await addEmailTag(account, mailbox, uid, tag.id, false);

      const newEmailTag: EmailTag = {
        tag_id: tag.id,
        tag_name: tag.name,
        tag_color: tag.color,
        auto_generated: false,
      };
      onTagsChange([...tags, newEmailTag]);

      // Update all tags list
      setAllTags((prev) => {
        if (prev.some((t) => t.id === tag.id)) return prev;
        return [...prev, tag];
      });

      setNewTagName("");
    } catch (error) {
      console.error("Failed to create tag:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleRemoveTag = async (tagId: number) => {
    setLoading(true);
    try {
      await removeEmailTag(account, mailbox, uid, tagId);
      onTagsChange(tags.filter((t) => t.tag_id !== tagId));
    } catch (error) {
      console.error("Failed to remove tag:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleGenerateAITags = async () => {
    if (!emailContext) return;

    setAiLoading(true);
    try {
      // TODO: Call AI auto-tag endpoint when implemented
      // For now, this is a placeholder
      console.log("AI tagging not yet implemented");
    } catch (error) {
      console.error("Failed to generate AI tags:", error);
    } finally {
      setAiLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("tags.manageTags")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Current tags */}
          <div>
            <label className="text-sm font-medium text-muted-foreground">
              {t("tags.currentTags")}
            </label>
            <div className="mt-2 flex flex-wrap gap-2 min-h-[32px]">
              {tags.length === 0 ? (
                <span className="text-sm text-muted-foreground">
                  {t("tags.noTags")}
                </span>
              ) : (
                tags.map((tag) => (
                  <TagBadge
                    key={tag.tag_id}
                    tag={tag}
                    size="md"
                    onRemove={() => handleRemoveTag(tag.tag_id)}
                  />
                ))
              )}
            </div>
          </div>

          <hr className="border-border" />

          {/* Add new tag */}
          <div>
            <label className="text-sm font-medium text-muted-foreground">
              {t("tags.addTag")}
            </label>
            <div className="mt-2 flex gap-2">
              <Input
                value={newTagName}
                onChange={(e) => setNewTagName(e.target.value)}
                placeholder={t("tags.tagNamePlaceholder")}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleCreateAndAddTag();
                  }
                }}
              />
              <Button
                size="sm"
                onClick={handleCreateAndAddTag}
                disabled={!newTagName.trim() || loading}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Available tags to select */}
          {availableTags.length > 0 && (
            <div>
              <label className="text-sm font-medium text-muted-foreground">
                {t("tags.selectExisting")}
              </label>
              <div className="mt-2 flex flex-wrap gap-2">
                {availableTags.map((tag) => (
                  <button
                    key={tag.id}
                    onClick={() => handleAddExistingTag(tag)}
                    disabled={loading}
                    className="inline-flex items-center rounded-full px-2 py-1 text-xs font-medium text-white transition-opacity hover:opacity-80 disabled:opacity-50"
                    style={{ backgroundColor: tag.color }}
                  >
                    {tag.name}
                  </button>
                ))}
              </div>
            </div>
          )}

          <hr className="border-border" />

          {/* AI Generate */}
          <Button
            variant="outline"
            className="w-full"
            onClick={handleGenerateAITags}
            disabled={aiLoading || !emailContext}
          >
            {aiLoading ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="mr-2 h-4 w-4" />
            )}
            {t("tags.generateWithAI")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
