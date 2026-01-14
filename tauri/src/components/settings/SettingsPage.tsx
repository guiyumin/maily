import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ArrowLeft, Settings, Bot, Users, Bell, Plug } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { GeneralSettings } from "./GeneralSettings";
import { AIProvidersSettings } from "./AIProvidersSettings";
import { AccountsSettings } from "./AccountsSettings";
import { NotificationSettings } from "./NotificationSettings";
import { IntegrationsSettings } from "./IntegrationsSettings";
import type { Account, Config } from "./types";
import { cn } from "@/lib/utils";

type SettingsSection = "general" | "ai" | "accounts" | "notifications" | "integrations";

const sections: { id: SettingsSection; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "general", label: "General", icon: Settings },
  { id: "ai", label: "AI Providers", icon: Bot },
  { id: "accounts", label: "Accounts", icon: Users },
  { id: "notifications", label: "Notifications", icon: Bell },
  { id: "integrations", label: "Integrations", icon: Plug },
];

export function SettingsPage() {
  const [config, setConfig] = useState<Config | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [activeSection, setActiveSection] = useState<SettingsSection>("general");

  useEffect(() => {
    Promise.all([
      invoke<Account[]>("list_accounts"),
      invoke<Config>("get_config"),
    ])
      .then(([accountsData, configData]) => {
        setAccounts(accountsData ?? []);
        // Ensure ai_providers is always an array (backend skips serializing empty arrays)
        setConfig({
          ...configData,
          ai_providers: configData.ai_providers ?? [],
          notifications: configData.notifications ?? {
            native: { enabled: true, new_email: true, calendar_reminder: true },
          },
        });
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const updateConfig = (updates: Partial<Config>) => {
    if (!config) return;
    setConfig({ ...config, ...updates });
    setDirty(true);
  };

  const saveConfig = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await invoke("save_config", { config });
      setDirty(false);
    } catch (err) {
      console.error("Failed to save config:", err);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <p className="text-muted-foreground">Loading settings...</p>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="flex h-screen items-center justify-center">
        <p className="text-destructive">Failed to load settings</p>
      </div>
    );
  }

  const renderSection = () => {
    switch (activeSection) {
      case "general":
        return <GeneralSettings config={config} onUpdate={updateConfig} />;
      case "ai":
        return <AIProvidersSettings config={config} onConfigChange={setConfig} />;
      case "accounts":
        return <AccountsSettings accounts={accounts} onAccountsChange={setAccounts} />;
      case "notifications":
        return <NotificationSettings config={config} onUpdate={updateConfig} />;
      case "integrations":
        return <IntegrationsSettings config={config} onUpdate={updateConfig} />;
      default:
        return null;
    }
  };

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-56 border-r bg-muted/30 flex flex-col">
        <div className="p-4 border-b">
          <Link
            to="/"
            className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            <span className="text-sm font-medium">Back</span>
          </Link>
        </div>

        <nav className="flex-1 p-2">
          <ul className="space-y-1">
            {sections.map((section) => {
              const Icon = section.icon;
              return (
                <li key={section.id}>
                  <button
                    onClick={() => setActiveSection(section.id)}
                    className={cn(
                      "w-full flex items-center gap-3 px-3 py-2 text-sm rounded-md transition-colors",
                      activeSection === section.id
                        ? "bg-primary text-primary-foreground"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    )}
                  >
                    <Icon className="h-4 w-4" />
                    {section.label}
                  </button>
                </li>
              );
            })}
          </ul>
        </nav>

        <div className="p-4 border-t">
          <p className="text-xs text-muted-foreground">
            Maily Desktop
          </p>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0">
        <header className="h-14 border-b flex items-center justify-between px-6 shrink-0">
          <h1 className="text-lg font-semibold">
            {sections.find((s) => s.id === activeSection)?.label}
          </h1>
          <div className="flex items-center gap-3">
            {dirty && (
              <span className="text-sm text-muted-foreground">Unsaved changes</span>
            )}
            <Button onClick={saveConfig} disabled={!dirty || saving} size="sm">
              {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        </header>

        <ScrollArea className="flex-1">
          <div className="max-w-2xl p-6">
            {renderSection()}
          </div>
        </ScrollArea>
      </main>
    </div>
  );
}
