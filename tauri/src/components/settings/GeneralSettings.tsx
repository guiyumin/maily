import { useState, useEffect } from "react";
import { getVersion } from "@tauri-apps/api/app";
import { check } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
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
import { Progress } from "@/components/ui/progress";
import { RefreshCw, Check, Download, Loader2, AlertCircle } from "lucide-react";
import type { Config } from "@/types/config";
import { useLocale } from "@/lib/i18n";

interface GeneralSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function GeneralSettings({ config, onUpdate }: GeneralSettingsProps) {
  const { t } = useLocale();
  const [version, setVersion] = useState<string>("");
  const [updateState, setUpdateState] = useState<
    "idle" | "checking" | "available" | "downloading" | "up-to-date" | "error"
  >("idle");
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateNotes, setUpdateNotes] = useState<string>("");
  const [downloadProgress, setDownloadProgress] = useState(0);
  const [errorMessage, setErrorMessage] = useState<string>("");

  useEffect(() => {
    getVersion().then(setVersion).catch(console.error);
  }, []);

  const handleCheckForUpdates = async () => {
    setUpdateState("checking");
    setErrorMessage("");
    try {
      const update = await check();
      if (update) {
        setUpdateState("available");
        setUpdateVersion(update.version);
        setUpdateNotes(update.body || "");
      } else {
        setUpdateState("up-to-date");
      }
    } catch (error) {
      console.error("[updater] Check failed:", error);
      setUpdateState("error");
      const msg = error instanceof Error
        ? error.message
        : typeof error === "string"
          ? error
          : JSON.stringify(error);
      setErrorMessage(msg || "Failed to check for updates");
    }
  };

  const handleDownloadAndInstall = async () => {
    setUpdateState("downloading");
    setDownloadProgress(0);
    try {
      const update = await check();
      if (!update) {
        setUpdateState("error");
        setErrorMessage("No update available");
        return;
      }

      let downloaded = 0;
      let contentLength = 0;

      await update.downloadAndInstall((event) => {
        switch (event.event) {
          case "Started":
            contentLength = event.data.contentLength || 0;
            break;
          case "Progress":
            downloaded += event.data.chunkLength;
            if (contentLength > 0) {
              setDownloadProgress(Math.round((downloaded / contentLength) * 100));
            }
            break;
          case "Finished":
            setDownloadProgress(100);
            break;
        }
      });

      await relaunch();
    } catch (error) {
      setUpdateState("error");
      setErrorMessage(error instanceof Error ? error.message : "Failed to install update");
    }
  };

  return (
    <>
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.general.title")}</CardTitle>
        <CardDescription>{t("settings.general.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-2">
          <Label htmlFor="max_emails">{t("settings.general.maxEmails")}</Label>
          <Input
            id="max_emails"
            type="number"
            value={config.max_emails}
            onChange={(e) =>
              onUpdate({ max_emails: parseInt(e.target.value) || 50 })
            }
          />
          <p className="text-sm text-muted-foreground">
            {t("settings.general.maxEmailsDescription")}
          </p>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="default_label">{t("settings.general.defaultLabel")}</Label>
          <Input
            id="default_label"
            value={config.default_label}
            onChange={(e) => onUpdate({ default_label: e.target.value })}
          />
          <p className="text-sm text-muted-foreground">
            {t("settings.general.defaultLabelDescription")}
          </p>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="theme">{t("settings.general.theme")}</Label>
          <Select
            value={config.theme}
            onValueChange={(value) => onUpdate({ theme: value })}
          >
            <SelectTrigger>
              <SelectValue placeholder={t("settings.general.theme")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="default">{t("settings.general.themeDefault")}</SelectItem>
              <SelectItem value="dark">{t("settings.general.themeDark")}</SelectItem>
              <SelectItem value="light">{t("settings.general.themeLight")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="language">{t("settings.general.language")}</Label>
          <Select
            value={config.language || "auto"}
            onValueChange={(value) =>
              onUpdate({ language: value === "auto" ? "" : value })
            }
          >
            <SelectTrigger>
              <SelectValue placeholder={t("settings.general.language")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">{t("settings.general.languageAuto")}</SelectItem>
              <SelectItem value="en">English</SelectItem>
              <SelectItem value="ko">한국어</SelectItem>
              <SelectItem value="ja">日本語</SelectItem>
              <SelectItem value="zh-Hans">简体中文</SelectItem>
              <SelectItem value="zh-Hant">繁體中文</SelectItem>
              <SelectItem value="es">Español</SelectItem>
              <SelectItem value="de">Deutsch</SelectItem>
              <SelectItem value="fr">Français</SelectItem>
              <SelectItem value="pt-BR">Português (Brasil)</SelectItem>
              <SelectItem value="pl">Polski</SelectItem>
              <SelectItem value="nl">Nederlands</SelectItem>
              <SelectItem value="it">Italiano</SelectItem>
              <SelectItem value="ru">Русский</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>

    {/* About & Updates Card */}
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.about.title")}</CardTitle>
        <CardDescription>{t("settings.about.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">{t("settings.about.currentVersion")}</p>
            <p className="text-sm text-muted-foreground">desktop-v{version}</p>
          </div>

          {updateState === "idle" && (
            <Button variant="outline" onClick={handleCheckForUpdates}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {t("settings.about.checkForUpdates")}
            </Button>
          )}

          {updateState === "checking" && (
            <Button variant="outline" disabled>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t("settings.about.checking")}
            </Button>
          )}

          {updateState === "up-to-date" && (
            <Button variant="outline" onClick={handleCheckForUpdates}>
              <Check className="mr-2 h-4 w-4 text-green-500" />
              {t("settings.about.upToDate")}
            </Button>
          )}

          {updateState === "available" && (
            <Button onClick={handleDownloadAndInstall}>
              <Download className="mr-2 h-4 w-4" />
              {t("settings.about.updateTo")} desktop-v{updateVersion}
            </Button>
          )}

          {updateState === "downloading" && (
            <Button disabled>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t("settings.about.installing")}
            </Button>
          )}

          {updateState === "error" && (
            <Button variant="outline" onClick={handleCheckForUpdates}>
              <AlertCircle className="mr-2 h-4 w-4 text-destructive" />
              {t("settings.about.retry")}
            </Button>
          )}
        </div>

        {updateState === "downloading" && (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span>{t("settings.about.downloading")}</span>
              <span>{downloadProgress}%</span>
            </div>
            <Progress value={downloadProgress} />
          </div>
        )}

        {updateState === "available" && updateNotes && (
          <div className="rounded-lg bg-muted p-3">
            <p className="text-sm font-medium mb-1">{t("settings.about.whatsNew")}</p>
            <p className="text-sm text-muted-foreground">{updateNotes}</p>
          </div>
        )}

        {updateState === "error" && (
          <p className="text-sm text-destructive">{errorMessage || "Unknown error (check console with Cmd+Option+I)"}</p>
        )}
      </CardContent>
    </Card>
    </>
  );
}
