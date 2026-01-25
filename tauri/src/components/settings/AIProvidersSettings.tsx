import { useState } from "react";
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
import { Switch } from "@/components/ui/switch";
import { Plus, Trash2, Zap, CheckCircle, XCircle, Loader2, Pencil, Eye, EyeOff, Sparkles } from "lucide-react";
import { toast } from "sonner";
import type { AIProvider, Config, TestResult, AutoTaggingConfig } from "./types";
import { useLocale } from "@/lib/i18n";

interface AIProvidersSettingsProps {
  config: Config;
  onConfigChange: (config: Config) => void;
}

type DialogMode = "view" | "edit" | "new";

export function AIProvidersSettings({ config, onConfigChange }: AIProvidersSettingsProps) {
  const { t } = useLocale();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<DialogMode>("new");
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [providerType, setProviderType] = useState<"cli" | "api">("cli");
  const [name, setName] = useState("");
  const [model, setModel] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [testingProvider, setTestingProvider] = useState<number | "new" | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [showApiKey, setShowApiKey] = useState(false);

  const isReadOnly = dialogMode === "view";

  const resetForm = () => {
    setEditingIndex(null);
    setDialogMode("new");
    setProviderType("cli");
    setName("");
    setModel("");
    setBaseUrl("");
    setApiKey("");
    setShowApiKey(false);
  };

  const openDialog = (index: number, mode: DialogMode) => {
    const provider = config.ai_providers[index];
    setEditingIndex(index);
    setDialogMode(mode);
    setProviderType(provider.type);
    setName(provider.name);
    setModel(provider.model);
    setBaseUrl(provider.base_url);
    setApiKey(provider.api_key);
    setShowApiKey(false);
    setDialogOpen(true);
  };

  const saveProvider = async () => {
    if (!name || !model) return;

    const provider: AIProvider = {
      type: providerType,
      name,
      model,
      base_url: baseUrl,
      api_key: apiKey,
    };

    try {
      let newConfig: Config;
      if (editingIndex !== null) {
        newConfig = await invoke<Config>("update_ai_provider", {
          index: editingIndex,
          provider,
        });
        toast.success("Provider updated");
      } else {
        newConfig = await invoke<Config>("add_ai_provider", { provider });
        toast.success("Provider added");
      }
      onConfigChange(newConfig);
      setDialogOpen(false);
      resetForm();
    } catch (err) {
      toast.error(`Failed to save provider: ${err}`);
    }
  };

  const removeProvider = async (index: number) => {
    const provider = config.ai_providers[index];
    if (!confirm(`Delete provider "${provider.name}"? This cannot be undone.`)) return;

    try {
      const newConfig = await invoke<Config>("remove_ai_provider", { index });
      // Ensure ai_providers is always an array (backend skips serializing empty arrays)
      onConfigChange({
        ...newConfig,
        ai_providers: newConfig.ai_providers ?? [],
      });
      toast.success("Provider deleted");
    } catch (err) {
      toast.error(`Failed to remove provider: ${err}`);
    }
  };

  const testProvider = async (
    index: number | "new",
    testName: string,
    testModel: string,
    type: "cli" | "api",
    testBaseUrl: string,
    testApiKey: string
  ) => {
    setTestingProvider(index);
    try {
      const result = await invoke<TestResult>("test_ai_provider", {
        providerName: testName,
        providerModel: testModel,
        providerType: type,
        baseUrl: testBaseUrl,
        apiKey: testApiKey,
      });

      const key = index === "new" ? "new" : `${index}`;
      setTestResults((prev) => ({ ...prev, [key]: result }));

      if (result.success) {
        toast.success(`${testName} is working!`);
      } else {
        toast.error(`${testName} failed: ${result.error || "Unknown error"}`);
      }
    } catch (err) {
      const key = index === "new" ? "new" : `${index}`;
      setTestResults((prev) => ({
        ...prev,
        [key]: { success: false, content: null, error: String(err), model_used: null },
      }));
      toast.error(`Failed to test provider: ${err}`);
    } finally {
      setTestingProvider(null);
    }
  };

  // Auto-tagging configuration
  const autoTagging = config.auto_tagging ?? {
    enabled: false,
    rate_limit_ms: 1000,
    max_emails_per_sync: 5,
  };

  const updateAutoTagging = async (updates: Partial<AutoTaggingConfig>) => {
    const newAutoTagging = { ...autoTagging, ...updates };
    const newConfig = { ...config, auto_tagging: newAutoTagging };
    try {
      await invoke("save_config", { config: newConfig });
      onConfigChange(newConfig);
      toast.success("Auto-tagging settings saved");
    } catch (err) {
      toast.error(`Failed to save: ${err}`);
    }
  };

  return (
    <div className="space-y-6">
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.ai.title")}</CardTitle>
        <CardDescription>
          {t("settings.ai.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {config.ai_providers.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("settings.ai.noProviders")}</p>
        ) : (
          <div className="space-y-2">
            {config.ai_providers.map((provider, index) => {
              const testResult = testResults[`${index}`];
              return (
                <div
                  key={index}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="flex items-center gap-3">
                    {testResult &&
                      (testResult.success ? (
                        <CheckCircle className="h-4 w-4 text-green-500" />
                      ) : (
                        <XCircle className="h-4 w-4 text-red-500" />
                      ))}
                    <div>
                      <p className="font-medium">
                        {provider.name}/{provider.model}
                      </p>
                      <p className="text-sm text-muted-foreground">
                        {provider.type === "cli" ? "CLI Tool" : "API"}
                        {provider.base_url && ` • ${provider.base_url}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() =>
                        testProvider(
                          index,
                          provider.name,
                          provider.model,
                          provider.type,
                          provider.base_url,
                          provider.api_key
                        )
                      }
                      disabled={testingProvider === index}
                      title="Test provider"
                    >
                      {testingProvider === index ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Zap className="h-4 w-4" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => openDialog(index, "view")}
                      title="View provider"
                    >
                      <Eye className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => openDialog(index, "edit")}
                      title="Edit provider"
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => removeProvider(index)}
                      title="Remove provider"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <Dialog
          open={dialogOpen}
          onOpenChange={(open) => {
            setDialogOpen(open);
            if (!open) resetForm();
          }}
        >
          <DialogTrigger asChild>
            <Button variant="outline" className="w-full">
              <Plus className="mr-2 h-4 w-4" />
              {t("settings.ai.addProvider")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {dialogMode === "view" && "View AI Provider"}
                {dialogMode === "edit" && "Edit AI Provider"}
                {dialogMode === "new" && "Add AI Provider"}
              </DialogTitle>
              <DialogDescription>
                {dialogMode === "view" && "Provider configuration details"}
                {dialogMode === "edit" && "Update the provider configuration"}
                {dialogMode === "new" && "Add a CLI tool or API provider for AI features"}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              <div className="grid gap-2">
                <Label>{t("settings.ai.type")}</Label>
                <Select
                  key={`provider-type-${editingIndex ?? "new"}-${dialogMode}`}
                  value={providerType}
                  onValueChange={(value: "cli" | "api") => setProviderType(value)}
                  disabled={isReadOnly}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("settings.ai.type")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="cli">{t("settings.ai.typeCli")}</SelectItem>
                    <SelectItem value="api">{t("settings.ai.typeApi")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="provider_name">{t("settings.ai.name")}</Label>
                <Input
                  id="provider_name"
                  placeholder={providerType === "cli" ? "claude, codex, gemini..." : "openai, groq..."}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={isReadOnly}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="provider_model">{t("settings.ai.model")}</Label>
                <Input
                  id="provider_model"
                  placeholder={providerType === "cli" ? "haiku, o4-mini, flash..." : "gpt-4o-mini, llama-3.1-8b..."}
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  disabled={isReadOnly}
                />
              </div>

              {providerType === "api" && (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="provider_base_url">{t("settings.ai.baseUrl")}</Label>
                    <Input
                      id="provider_base_url"
                      placeholder="https://api.openai.com/v1"
                      value={baseUrl}
                      onChange={(e) => setBaseUrl(e.target.value)}
                      disabled={isReadOnly}
                    />
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="provider_api_key">{t("settings.ai.apiKey")}</Label>
                    <div className="relative">
                      <Input
                        id="provider_api_key"
                        type={showApiKey ? "text" : "password"}
                        placeholder="sk-..."
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        disabled={isReadOnly}
                        className="pr-10"
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="absolute right-0 top-0 h-full px-3 hover:bg-transparent"
                        onClick={() => setShowApiKey(!showApiKey)}
                      >
                        {showApiKey ? (
                          <EyeOff className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <Eye className="h-4 w-4 text-muted-foreground" />
                        )}
                      </Button>
                    </div>
                  </div>
                </>
              )}
            </div>

            <DialogFooter className="flex-col sm:flex-row gap-2">
              {isReadOnly ? (
                <div className="flex gap-2 ml-auto">
                  <Button variant="outline" onClick={() => setDialogOpen(false)}>
                    {t("common.cancel")}
                  </Button>
                  <Button onClick={() => setDialogMode("edit")}>
                    <Pencil className="mr-2 h-4 w-4" />
                    {t("common.edit")}
                  </Button>
                </div>
              ) : (
                <>
                  <div className="flex items-center gap-2">
                    {testResults["new"] &&
                      (testResults["new"].success ? (
                        <span className="flex items-center gap-1 text-sm text-green-600">
                          <CheckCircle className="h-4 w-4" />
                          Working
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-sm text-red-600">
                          <XCircle className="h-4 w-4" />
                          Failed
                        </span>
                      ))}
                    <Button
                      variant="outline"
                      onClick={() => testProvider("new", name, model, providerType, baseUrl, apiKey)}
                      disabled={!name || !model || testingProvider === "new"}
                    >
                      {testingProvider === "new" ? (
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      ) : (
                        <Zap className="mr-2 h-4 w-4" />
                      )}
                      Test
                    </Button>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      onClick={() => {
                        setDialogOpen(false);
                        setTestResults((prev) => {
                          const next = { ...prev };
                          delete next["new"];
                          return next;
                        });
                      }}
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button onClick={saveProvider} disabled={!name || !model}>
                      {dialogMode === "edit" ? t("common.save") : t("settings.ai.addProvider")}
                    </Button>
                  </div>
                </>
              )}
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>

    {/* Auto-Tagging Settings */}
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Sparkles className="h-5 w-5" />
          Auto-Tagging
        </CardTitle>
        <CardDescription>
          Automatically generate tags for new emails using AI during sync.
          Requires at least one AI provider configured above.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <Label htmlFor="auto-tag-enabled">Enable Auto-Tagging</Label>
            <p className="text-sm text-muted-foreground">
              Automatically tag new emails after sync completes
            </p>
          </div>
          <Switch
            id="auto-tag-enabled"
            checked={autoTagging.enabled}
            onCheckedChange={(checked) => updateAutoTagging({ enabled: checked })}
            disabled={config.ai_providers.length === 0}
          />
        </div>

        {autoTagging.enabled && (
          <>
            <div className="grid gap-2">
              <Label htmlFor="max-emails">Max emails per sync</Label>
              <p className="text-sm text-muted-foreground">
                Maximum number of emails to auto-tag during each sync (1-20)
              </p>
              <Input
                id="max-emails"
                type="number"
                min={1}
                max={20}
                value={autoTagging.max_emails_per_sync}
                onChange={(e) => {
                  const val = Math.min(20, Math.max(1, parseInt(e.target.value) || 5));
                  updateAutoTagging({ max_emails_per_sync: val });
                }}
                className="w-24"
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="rate-limit">Rate limit (ms)</Label>
              <p className="text-sm text-muted-foreground">
                Delay between AI API calls to avoid rate limiting (500-5000ms)
              </p>
              <Input
                id="rate-limit"
                type="number"
                min={500}
                max={5000}
                step={100}
                value={autoTagging.rate_limit_ms}
                onChange={(e) => {
                  const val = Math.min(5000, Math.max(500, parseInt(e.target.value) || 1000));
                  updateAutoTagging({ rate_limit_ms: val });
                }}
                className="w-24"
              />
            </div>
          </>
        )}

        {config.ai_providers.length === 0 && (
          <p className="text-sm text-amber-600 dark:text-amber-400">
            Add an AI provider above to enable auto-tagging.
          </p>
        )}
      </CardContent>
    </Card>
    </div>
  );
}
