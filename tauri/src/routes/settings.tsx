import { createFileRoute } from "@tanstack/react-router";
import { SettingsPage } from "@/components/settings";

type SettingsSearch = {
  section?: "general" | "ai" | "accounts" | "notifications" | "integrations" | "tags";
};

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
  validateSearch: (search: Record<string, unknown>): SettingsSearch => {
    const validSections = ["general", "ai", "accounts", "notifications", "integrations", "tags"];
    const section = search.section as string | undefined;
    return {
      section: section && validSections.includes(section)
        ? (section as SettingsSearch["section"])
        : undefined,
    };
  },
});
