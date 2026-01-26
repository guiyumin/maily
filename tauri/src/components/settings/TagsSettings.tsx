import { useState, useEffect } from "react";
import { Trash2, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { toast } from "sonner";
import { listTags, updateTag, deleteTag } from "@/lib/tags";
import type { Tag } from "@/types/tags";
import { TAG_COLORS } from "@/types/tags";
import { useLocale } from "@/lib/i18n";

export function TagsSettings() {
  const { t } = useLocale();
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editName, setEditName] = useState("");

  useEffect(() => {
    loadTags();
  }, []);

  const loadTags = async () => {
    try {
      const allTags = await listTags();
      setTags(allTags);
    } catch (err) {
      console.error("Failed to load tags:", err);
      toast.error("Failed to load tags");
    } finally {
      setLoading(false);
    }
  };

  const handleColorChange = async (tag: Tag, newColor: string) => {
    try {
      const updated = await updateTag(tag.id, tag.name, newColor);
      setTags((prev) => prev.map((t) => (t.id === tag.id ? updated : t)));
      toast.success("Tag color updated");
    } catch (err) {
      console.error("Failed to update tag:", err);
      toast.error("Failed to update tag");
    }
  };

  const handleNameChange = async (tag: Tag) => {
    if (!editName.trim() || editName === tag.name) {
      setEditingId(null);
      return;
    }

    try {
      const updated = await updateTag(tag.id, editName.trim(), tag.color);
      setTags((prev) => prev.map((t) => (t.id === tag.id ? updated : t)));
      toast.success("Tag name updated");
    } catch (err) {
      console.error("Failed to update tag:", err);
      toast.error("Failed to update tag");
    } finally {
      setEditingId(null);
    }
  };

  const handleDelete = async (tag: Tag) => {
    if (!confirm(`Delete tag "${tag.name}"? This will remove it from all emails.`)) {
      return;
    }

    try {
      await deleteTag(tag.id);
      setTags((prev) => prev.filter((t) => t.id !== tag.id));
      toast.success("Tag deleted");
    } catch (err) {
      console.error("Failed to delete tag:", err);
      toast.error("Failed to delete tag");
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.tags.title")}</CardTitle>
        <CardDescription>{t("settings.tags.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {tags.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("settings.tags.noTags")}</p>
        ) : (
          <div className="space-y-1.5">
            {tags.map((tag) => (
              <div
                key={tag.id}
                className="flex items-center gap-2 rounded-md border p-2"
              >
                {/* Color picker */}
                <div className="flex gap-0.5">
                  {TAG_COLORS.map((color) => (
                    <button
                      key={color}
                      onClick={() => handleColorChange(tag, color)}
                      className={`h-4 w-4 rounded-full transition-transform hover:scale-110 ${
                        tag.color === color ? "ring-1 ring-offset-1 ring-primary" : ""
                      }`}
                      style={{ backgroundColor: color }}
                      title={color}
                    />
                  ))}
                </div>

                {/* Tag name */}
                <div className="flex-1">
                  {editingId === tag.id ? (
                    <Input
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      onBlur={() => handleNameChange(tag)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleNameChange(tag);
                        if (e.key === "Escape") setEditingId(null);
                      }}
                      className="h-6 text-xs"
                      autoFocus
                    />
                  ) : (
                    <button
                      onClick={() => {
                        setEditingId(tag.id);
                        setEditName(tag.name);
                      }}
                      className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium text-white hover:opacity-80"
                      style={{ backgroundColor: tag.color }}
                    >
                      {tag.name}
                    </button>
                  )}
                </div>

                {/* Delete button */}
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(tag)}
                  className="h-6 w-6 text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
