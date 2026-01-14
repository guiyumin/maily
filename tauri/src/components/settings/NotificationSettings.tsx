import { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Bell, Send, Loader2, CheckCircle, XCircle, Eye, EyeOff } from "lucide-react";
import { toast } from "sonner";
import type { Config, NotificationConfig, TelegramConfig } from "./types";

interface NotificationSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function NotificationSettings({ config, onUpdate }: NotificationSettingsProps) {
  const [testingTelegram, setTestingTelegram] = useState(false);
  const [telegramTestResult, setTelegramTestResult] = useState<boolean | null>(null);
  const [showBotToken, setShowBotToken] = useState(false);

  const notifications = config.notifications ?? {
    native: { enabled: true, new_email: true, calendar_reminder: true },
  };

  const updateNotifications = (updates: Partial<NotificationConfig>) => {
    onUpdate({
      notifications: { ...notifications, ...updates },
    });
  };

  const updateNative = (updates: Partial<NotificationConfig["native"]>) => {
    updateNotifications({
      native: { ...notifications.native, ...updates },
    });
  };

  const updateTelegram = (updates: Partial<TelegramConfig>) => {
    const currentTelegram = notifications.telegram ?? {
      enabled: false,
      bot_token: "",
      chat_id: "",
    };
    updateNotifications({
      telegram: { ...currentTelegram, ...updates },
    });
  };

  const testTelegram = async () => {
    const telegram = notifications.telegram;
    if (!telegram?.bot_token || !telegram?.chat_id) {
      toast.error("Please enter bot token and chat ID first");
      return;
    }

    setTestingTelegram(true);
    setTelegramTestResult(null);

    try {
      await invoke("test_telegram", {
        botToken: telegram.bot_token,
        chatId: telegram.chat_id,
      });
      setTelegramTestResult(true);
      toast.success("Test message sent successfully!");
    } catch (err) {
      setTelegramTestResult(false);
      toast.error(`Failed to send test message: ${err}`);
    } finally {
      setTestingTelegram(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Native Notifications */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Bell className="h-5 w-5" />
            Native Notifications
          </CardTitle>
          <CardDescription>
            System notifications on macOS
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>Enable Notifications</Label>
              <p className="text-sm text-muted-foreground">
                Show native macOS notifications
              </p>
            </div>
            <Switch
              checked={notifications.native.enabled}
              onCheckedChange={(checked) => updateNative({ enabled: checked })}
            />
          </div>

          {notifications.native.enabled && (
            <>
              <div className="flex items-center justify-between pl-4 border-l-2 border-muted">
                <div className="space-y-0.5">
                  <Label>New Email</Label>
                  <p className="text-sm text-muted-foreground">
                    Notify when new emails arrive
                  </p>
                </div>
                <Switch
                  checked={notifications.native.new_email}
                  onCheckedChange={(checked) => updateNative({ new_email: checked })}
                />
              </div>

              <div className="flex items-center justify-between pl-4 border-l-2 border-muted">
                <div className="space-y-0.5">
                  <Label>Calendar Reminders</Label>
                  <p className="text-sm text-muted-foreground">
                    Notify for upcoming calendar events
                  </p>
                </div>
                <Switch
                  checked={notifications.native.calendar_reminder}
                  onCheckedChange={(checked) => updateNative({ calendar_reminder: checked })}
                />
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Telegram Notifications */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Send className="h-5 w-5" />
            Telegram
          </CardTitle>
          <CardDescription>
            Send notifications to a Telegram chat
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>Enable Telegram</Label>
              <p className="text-sm text-muted-foreground">
                Send notifications via Telegram bot
              </p>
            </div>
            <Switch
              checked={notifications.telegram?.enabled ?? false}
              onCheckedChange={(checked) => updateTelegram({ enabled: checked })}
            />
          </div>

          {notifications.telegram?.enabled && (
            <>
              <div className="grid gap-2">
                <Label htmlFor="bot_token">Bot Token</Label>
                <div className="relative">
                  <Input
                    id="bot_token"
                    type={showBotToken ? "text" : "password"}
                    placeholder="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
                    value={notifications.telegram?.bot_token ?? ""}
                    onChange={(e) => updateTelegram({ bot_token: e.target.value })}
                    className="pr-10"
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="absolute right-0 top-0 h-full px-3 hover:bg-transparent"
                    onClick={() => setShowBotToken(!showBotToken)}
                  >
                    {showBotToken ? (
                      <EyeOff className="h-4 w-4 text-muted-foreground" />
                    ) : (
                      <Eye className="h-4 w-4 text-muted-foreground" />
                    )}
                  </Button>
                </div>
                <p className="text-sm text-muted-foreground">
                  Create a bot via{" "}
                  <a
                    href="https://t.me/BotFather"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    @BotFather
                  </a>
                </p>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="chat_id">Chat ID</Label>
                <Input
                  id="chat_id"
                  placeholder="-1001234567890"
                  value={notifications.telegram?.chat_id ?? ""}
                  onChange={(e) => updateTelegram({ chat_id: e.target.value })}
                />
                <p className="text-sm text-muted-foreground">
                  Your user ID or group chat ID (starts with -)
                </p>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={testTelegram}
                  disabled={testingTelegram || !notifications.telegram?.bot_token || !notifications.telegram?.chat_id}
                >
                  {testingTelegram ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Send className="mr-2 h-4 w-4" />
                  )}
                  Send Test Message
                </Button>
                {telegramTestResult !== null && (
                  telegramTestResult ? (
                    <span className="flex items-center gap-1 text-sm text-green-600">
                      <CheckCircle className="h-4 w-4" />
                      Sent
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-sm text-red-600">
                      <XCircle className="h-4 w-4" />
                      Failed
                    </span>
                  )
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
