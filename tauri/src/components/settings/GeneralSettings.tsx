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
import type { Config } from "./types";
import { useLocale } from "@/lib/i18n";

interface GeneralSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function GeneralSettings({ config, onUpdate }: GeneralSettingsProps) {
  const { t } = useLocale();

  return (
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
  );
}
