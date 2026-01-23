export interface Tag {
  id: number;
  name: string;
  color: string;
  created_at: number;
}

export interface EmailTag {
  tag_id: number;
  tag_name: string;
  tag_color: string;
  auto_generated: boolean;
}

export const TAG_COLORS = [
  "#7C3AED", // Purple (primary)
  "#EF4444", // Red
  "#F59E0B", // Amber
  "#10B981", // Emerald
  "#3B82F6", // Blue
  "#EC4899", // Pink
  "#6366F1", // Indigo
  "#14B8A6", // Teal
];
