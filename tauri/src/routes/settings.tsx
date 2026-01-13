import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Plus, Trash2, ArrowLeft } from "lucide-react";
import { Link } from "@tanstack/react-router";

interface AIProvider {
  provider_type: "cli" | "api";
  name: string;
  model: string;
  base_url: string;
  api_key: string;
}

interface Config {
  max_emails: number;
  default_label: string;
  theme: string;
  language: string;
  ai_providers: AIProvider[];
}

export const Route = createFileRoute("/settings")({
  component: Settings,
});

function Settings() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  // Provider dialog state
  const [providerDialogOpen, setProviderDialogOpen] = useState(false);
  const [providerType, setProviderType] = useState<"cli" | "api">("cli");
  const [providerName, setProviderName] = useState("");
  const [providerModel, setProviderModel] = useState("");
  const [providerBaseUrl, setProviderBaseUrl] = useState("");
  const [providerApiKey, setProviderApiKey] = useState("");

  useEffect(() => {
    invoke<Config>("get_config")
      .then((cfg) => {
        setConfig(cfg);
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

  const resetProviderForm = () => {
    setProviderName("");
    setProviderModel("");
    setProviderBaseUrl("");
    setProviderApiKey("");
  };

  const addProvider = async () => {
    if (!providerName || !providerModel) return;

    const provider: AIProvider = {
      provider_type: providerType,
      name: providerName,
      model: providerModel,
      base_url: providerBaseUrl,
      api_key: providerApiKey,
    };

    try {
      const newConfig = await invoke<Config>("add_ai_provider", { provider });
      setConfig(newConfig);
      setProviderDialogOpen(false);
      resetProviderForm();
    } catch (err) {
      console.error("Failed to add provider:", err);
    }
  };

  const removeProvider = async (index: number) => {
    try {
      const newConfig = await invoke<Config>("remove_ai_provider", { index });
      setConfig(newConfig);
    } catch (err) {
      console.error("Failed to remove provider:", err);
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
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="flex h-14 items-center gap-4 px-6">
          <Link to="/" className="flex items-center gap-2 text-muted-foreground hover:text-foreground">
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

      {/* Content */}
      <main className="mx-auto max-w-2xl p-6 space-y-6">
        {/* General Settings */}
        <Card>
          <CardHeader>
            <CardTitle>General</CardTitle>
            <CardDescription>Basic application settings</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="max_emails">Max Emails</Label>
              <Input
                id="max_emails"
                type="number"
                value={config.max_emails}
                onChange={(e) =>
                  updateConfig({ max_emails: parseInt(e.target.value) || 50 })
                }
              />
              <p className="text-sm text-muted-foreground">
                Maximum number of emails to load per mailbox
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="default_label">Default Label</Label>
              <Input
                id="default_label"
                value={config.default_label}
                onChange={(e) => updateConfig({ default_label: e.target.value })}
              />
              <p className="text-sm text-muted-foreground">
                Default mailbox to open on startup
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="theme">Theme</Label>
              <Select
                value={config.theme}
                onValueChange={(value) => updateConfig({ theme: value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select theme" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default</SelectItem>
                  <SelectItem value="dark">Dark</SelectItem>
                  <SelectItem value="light">Light</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="language">Language</Label>
              <Select
                value={config.language || "auto"}
                onValueChange={(value) =>
                  updateConfig({ language: value === "auto" ? "" : value })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select language" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">Auto (detect from system)</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                  <SelectItem value="ko">한국어</SelectItem>
                  <SelectItem value="ja">日本語</SelectItem>
                  <SelectItem value="zh">中文</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>

        {/* AI Providers */}
        <Card>
          <CardHeader>
            <CardTitle>AI Providers</CardTitle>
            <CardDescription>
              Configure AI providers for email summarization and composition
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {config.ai_providers.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No AI providers configured
              </p>
            ) : (
              <div className="space-y-2">
                {config.ai_providers.map((provider, index) => (
                  <div
                    key={index}
                    className="flex items-center justify-between rounded-lg border p-3"
                  >
                    <div>
                      <p className="font-medium">
                        {provider.name}/{provider.model}
                      </p>
                      <p className="text-sm text-muted-foreground">
                        {provider.provider_type === "cli" ? "CLI Tool" : "API"}
                        {provider.base_url && ` • ${provider.base_url}`}
                      </p>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => removeProvider(index)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
              </div>
            )}

            <Dialog open={providerDialogOpen} onOpenChange={setProviderDialogOpen}>
              <DialogTrigger asChild>
                <Button variant="outline" className="w-full">
                  <Plus className="mr-2 h-4 w-4" />
                  Add Provider
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Add AI Provider</DialogTitle>
                  <DialogDescription>
                    Add a CLI tool or API provider for AI features
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-4 py-4">
                  <div className="grid gap-2">
                    <Label>Provider Type</Label>
                    <Select
                      value={providerType}
                      onValueChange={(value: "cli" | "api") => {
                        setProviderType(value);
                        resetProviderForm();
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="cli">CLI Tool (claude, codex, gemini...)</SelectItem>
                        <SelectItem value="api">API (OpenAI-compatible)</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="provider_name">Name</Label>
                    <Input
                      id="provider_name"
                      placeholder={providerType === "cli" ? "claude, codex, gemini..." : "openai, groq..."}
                      value={providerName}
                      onChange={(e) => setProviderName(e.target.value)}
                    />
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="provider_model">Model</Label>
                    <Input
                      id="provider_model"
                      placeholder={providerType === "cli" ? "haiku, o4-mini, flash..." : "gpt-4o-mini, llama-3.1-8b..."}
                      value={providerModel}
                      onChange={(e) => setProviderModel(e.target.value)}
                    />
                  </div>

                  {providerType === "api" && (
                    <>
                      <div className="grid gap-2">
                        <Label htmlFor="provider_base_url">Base URL</Label>
                        <Input
                          id="provider_base_url"
                          placeholder="https://api.openai.com/v1"
                          value={providerBaseUrl}
                          onChange={(e) => setProviderBaseUrl(e.target.value)}
                        />
                      </div>

                      <div className="grid gap-2">
                        <Label htmlFor="provider_api_key">API Key</Label>
                        <Input
                          id="provider_api_key"
                          type="password"
                          placeholder="sk-..."
                          value={providerApiKey}
                          onChange={(e) => setProviderApiKey(e.target.value)}
                        />
                      </div>
                    </>
                  )}
                </div>

                <DialogFooter>
                  <Button variant="outline" onClick={() => setProviderDialogOpen(false)}>
                    Cancel
                  </Button>
                  <Button onClick={addProvider} disabled={!providerName || !providerModel}>
                    Add Provider
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </CardContent>
        </Card>

        {/* Accounts section - placeholder for now */}
        <Card>
          <CardHeader>
            <CardTitle>Accounts</CardTitle>
            <CardDescription>
              Manage your email accounts
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Account management coming soon. For now, use the CLI:{" "}
              <code className="rounded bg-muted px-1 py-0.5">maily login gmail</code>
            </p>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
