import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ArrowLeft } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { GeneralSettings } from "./GeneralSettings";
import { AIProvidersSettings } from "./AIProvidersSettings";
import { AccountsSettings } from "./AccountsSettings";
import type { Account, Config } from "./types";

export function SettingsPage() {
  const [config, setConfig] = useState<Config | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

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

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-10 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
        <div className="flex h-14 items-center gap-4 px-6">
          <Link
            to="/"
            className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Back</span>
          </Link>
          <Separator orientation="vertical" className="h-6" />
          <h1 className="text-lg font-semibold">Settings</h1>
          <div className="ml-auto flex items-center gap-2">
            {dirty && (
              <span className="text-sm text-muted-foreground">Unsaved changes</span>
            )}
            <Button onClick={saveConfig} disabled={!dirty || saving}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-2xl p-6 space-y-6">
        <GeneralSettings config={config} onUpdate={updateConfig} />
        <AIProvidersSettings config={config} onConfigChange={setConfig} />
        <AccountsSettings accounts={accounts} onAccountsChange={setAccounts} />
      </main>
    </div>
  );
}
