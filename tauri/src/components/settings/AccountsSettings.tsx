import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";
import { useAccountsStore } from "@/stores/accounts";
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
import {
  Plus,
  Trash2,
  Zap,
  CheckCircle,
  XCircle,
  Loader2,
  Pencil,
  Eye,
  EyeOff,
  Upload,
  X,
  Images,
} from "lucide-react";
import { toast } from "sonner";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { FullAccount } from "@/types/account";
import { useLocale } from "@/lib/i18n";

interface AccountsSettingsProps {
  fullAccounts: FullAccount[];
  onFullAccountsChange: (accounts: FullAccount[]) => void;
}

type Provider = "gmail" | "yahoo" | "qq";

const PROVIDER_DEFAULTS: Record<
  Provider,
  { imapHost: string; imapPort: number; smtpHost: string; smtpPort: number }
> = {
  gmail: {
    imapHost: "imap.gmail.com",
    imapPort: 993,
    smtpHost: "smtp.gmail.com",
    smtpPort: 587,
  },
  yahoo: {
    imapHost: "imap.mail.yahoo.com",
    imapPort: 993,
    smtpHost: "smtp.mail.yahoo.com",
    smtpPort: 587,
  },
  qq: {
    imapHost: "imap.qq.com",
    imapPort: 993,
    smtpHost: "smtp.qq.com",
    smtpPort: 465,
  },
};

interface FormState {
  provider: Provider;
  name: string;
  displayName: string;
  email: string;
  password: string;
  imapHost: string;
  imapPort: number;
  smtpHost: string;
  smtpPort: number;
}

const DEFAULT_FORM: FormState = {
  provider: "gmail",
  name: "",
  displayName: "",
  email: "",
  password: "",
  ...PROVIDER_DEFAULTS.gmail,
};

export function AccountsSettings({
  fullAccounts,
  onFullAccountsChange,
}: AccountsSettingsProps) {
  const { t } = useLocale();

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);

  // Form state (consolidated)
  const [form, setForm] = useState<FormState>(DEFAULT_FORM);
  const updateForm = (updates: Partial<FormState>) =>
    setForm((prev) => ({ ...prev, ...updates }));

  // UI state
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    error?: string;
  } | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const [avatarFilePath, setAvatarFilePath] = useState<string | null>(null);
  const [selectedExistingAvatar, setSelectedExistingAvatar] = useState<
    string | null
  >(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);

  // Use shared store for avatar URLs
  const {
    avatarUrls,
    setAvatarUrl,
    removeAvatarUrl,
    setFullAccounts,
    loadAvatarUrls,
  } = useAccountsStore();

  // Sync full accounts to store and load avatar URLs
  useEffect(() => {
    setFullAccounts(fullAccounts);
    loadAvatarUrls();
  }, [fullAccounts, setFullAccounts, loadAvatarUrls]);

  const resetForm = () => {
    setEditingName(null);
    setForm(DEFAULT_FORM);
    setTestResult(null);
    setShowPassword(false);
    setAvatarPreview(null);
    setAvatarFilePath(null);
    setSelectedExistingAvatar(null);
  };

  const openEditDialog = async (account: FullAccount) => {
    setEditingName(account.name);
    setForm({
      provider: account.provider as Provider,
      name: account.name,
      displayName: account.display_name || "",
      email: account.credentials.email,
      password: account.credentials.password,
      imapHost: account.credentials.imap_host,
      imapPort: account.credentials.imap_port,
      smtpHost: account.credentials.smtp_host,
      smtpPort: account.credentials.smtp_port,
    });
    setTestResult(null);
    setShowPassword(false);
    setAvatarFilePath(null);
    setSelectedExistingAvatar(null);
    setAvatarPreview(
      account.avatar && avatarUrls[account.credentials.email]
        ? avatarUrls[account.credentials.email]
        : null,
    );
    setDialogOpen(true);
  };

  const selectAvatar = async () => {
    try {
      const selected = await open({
        multiple: false,
        filters: [
          {
            name: "Images",
            extensions: ["png", "jpg", "jpeg", "gif", "webp"],
          },
        ],
      });

      if (selected) {
        const filePath = selected as string;
        setAvatarFilePath(filePath);
        setSelectedExistingAvatar(null);
        // Show filename as indicator since we can't preview without fs plugin
        setAvatarPreview(
          `selected:${filePath.split("/").pop() || filePath.split("\\").pop()}`,
        );
      }
    } catch (err) {
      toast.error(`Failed to select image: ${err}`);
    }
  };

  const removeAvatar = () => {
    setAvatarPreview(null);
    setAvatarFilePath(null);
    setSelectedExistingAvatar(null);
  };

  const handleProviderChange = (provider: Provider) => {
    updateForm({ provider, ...PROVIDER_DEFAULTS[provider] });
  };

  const testConnection = async () => {
    if (!form.email || !form.password || !form.imapHost) return;

    setTesting(true);
    setTestResult(null);

    try {
      await invoke("test_account", {
        email: form.email,
        password: form.password,
        imapHost: form.imapHost,
        imapPort: form.imapPort,
      });
      setTestResult({ success: true });
      toast.success("Connection successful!");
    } catch (err) {
      setTestResult({ success: false, error: String(err) });
      toast.error(`Connection failed: ${err}`);
    } finally {
      setTesting(false);
    }
  };

  const saveAccount = async () => {
    if (!form.name || !form.email || !form.password) return;

    // Preserve existing avatar when editing
    const existingAccount = editingName
      ? fullAccounts.find((a) => a.name === editingName)
      : undefined;

    const account: FullAccount = {
      name: form.name,
      provider: form.provider,
      credentials: {
        email: form.email,
        password: form.password,
        imap_host: form.imapHost,
        imap_port: form.imapPort,
        smtp_host: form.smtpHost,
        smtp_port: form.smtpPort,
      },
      display_name: form.displayName || undefined,
      avatar: existingAccount?.avatar,
    };

    try {
      let newAccounts: FullAccount[];
      if (editingName) {
        newAccounts = await invoke<FullAccount[]>("update_account", {
          name: editingName,
          account,
        });
        toast.success("Account updated");
      } else {
        newAccounts = await invoke<FullAccount[]>("add_account", { account });
        toast.success("Account added");
      }

      // Handle avatar upload/copy/removal
      if (avatarFilePath) {
        // Upload new avatar from file
        setUploadingAvatar(true);
        try {
          newAccounts = await invoke<FullAccount[]>("upload_account_avatar", {
            accountName: form.name,
            filePath: avatarFilePath,
          });
          const urls = await invoke<Record<string, string>>(
            "get_account_avatar_urls",
          );
          if (urls[form.email]) {
            setAvatarUrl(form.email, urls[form.email]);
          }
        } catch (err) {
          toast.error(`Failed to upload avatar: ${err}`);
        } finally {
          setUploadingAvatar(false);
        }
      } else if (selectedExistingAvatar) {
        // Copy avatar from another account
        setUploadingAvatar(true);
        try {
          newAccounts = await invoke<FullAccount[]>("copy_account_avatar", {
            accountName: form.name,
            sourceFilename: selectedExistingAvatar,
          });
          const urls = await invoke<Record<string, string>>(
            "get_account_avatar_urls",
          );
          if (urls[form.email]) {
            setAvatarUrl(form.email, urls[form.email]);
          }
        } catch (err) {
          toast.error(`Failed to copy avatar: ${err}`);
        } finally {
          setUploadingAvatar(false);
        }
      } else if (editingName && avatarPreview === null) {
        // Remove existing avatar if preview was cleared
        const existingAccount = fullAccounts.find(
          (a) => a.name === editingName,
        );
        if (existingAccount?.avatar) {
          try {
            newAccounts = await invoke<FullAccount[]>("delete_account_avatar", {
              accountName: form.name,
            });
            removeAvatarUrl(form.email);
          } catch (err) {
            toast.error(`Failed to remove avatar: ${err}`);
          }
        }
      }

      onFullAccountsChange(newAccounts);
      setDialogOpen(false);
      resetForm();
    } catch (err) {
      toast.error(`Failed to save account: ${err}`);
    }
  };

  const deleteAccount = async (accountName: string) => {
    if (!confirm(`Delete account "${accountName}"? This cannot be undone.`))
      return;

    try {
      const newAccounts = await invoke<FullAccount[]>("remove_account", {
        name: accountName,
      });
      onFullAccountsChange(newAccounts);
      toast.success("Account deleted");
    } catch (err) {
      toast.error(`Failed to delete account: ${err}`);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.accounts.title")}</CardTitle>
        <CardDescription>{t("settings.accounts.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {fullAccounts.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("settings.accounts.noAccounts")}
          </p>
        ) : (
          <div className="space-y-2">
            {fullAccounts.map((account) => (
              <div
                key={account.name}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div className="flex items-center gap-3">
                  <Avatar className="size-10">
                    {avatarUrls[account.credentials.email] && (
                      <AvatarImage
                        src={avatarUrls[account.credentials.email]}
                        alt={account.name}
                      />
                    )}
                    <AvatarFallback className="bg-violet-600 text-xs">
                      {account.name.slice(0, 2).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <div>
                    <p className="font-medium">{account.name}</p>
                    <p className="text-sm text-muted-foreground">
                      {account.credentials.email} • {account.provider}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openEditDialog(account)}
                    title="Edit account"
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => deleteAccount(account.name)}
                    title="Remove account"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ))}
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
              {t("settings.accounts.addAccount")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {editingName ? "Edit Account" : "Add Account"}
              </DialogTitle>
              <DialogDescription>
                {editingName
                  ? "Update your email account settings"
                  : "Add an email account with IMAP/SMTP access"}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              <div className="grid gap-2">
                <Label>Provider</Label>
                <Select
                  value={form.provider}
                  onValueChange={handleProviderChange}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="gmail">Gmail</SelectItem>
                    <SelectItem value="yahoo">Yahoo</SelectItem>
                    <SelectItem value="qq">QQ Mail</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="account_name">Account Name</Label>
                <Input
                  id="account_name"
                  placeholder="Work, Personal..."
                  value={form.name}
                  onChange={(e) => updateForm({ name: e.target.value })}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="display_name">{t("settings.yourName")}</Label>
                <Input
                  id="display_name"
                  placeholder="Yumin"
                  value={form.displayName}
                  onChange={(e) => updateForm({ displayName: e.target.value })}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="account_email">Email</Label>
                <Input
                  id="account_email"
                  type="email"
                  placeholder="you@example.com"
                  value={form.email}
                  onChange={(e) => updateForm({ email: e.target.value })}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="account_password">
                  {form.provider === "qq"
                    ? "Authorization Code"
                    : "App Password"}
                </Label>
                <div className="relative">
                  <Input
                    id="account_password"
                    type={showPassword ? "text" : "password"}
                    placeholder={
                      form.provider === "qq"
                        ? "Authorization code"
                        : "App password"
                    }
                    value={form.password}
                    onChange={(e) => updateForm({ password: e.target.value })}
                    className="pr-10"
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="absolute right-0 top-0 h-full px-3 hover:bg-transparent"
                    onClick={() => setShowPassword(!showPassword)}
                  >
                    {showPassword ? (
                      <EyeOff className="h-4 w-4 text-muted-foreground" />
                    ) : (
                      <Eye className="h-4 w-4 text-muted-foreground" />
                    )}
                  </Button>
                </div>
                {form.provider === "gmail" && (
                  <p className="text-xs text-muted-foreground">
                    Generate at{" "}
                    <a
                      href="https://myaccount.google.com/apppasswords"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="underline"
                    >
                      Google App Passwords
                    </a>
                  </p>
                )}
                {form.provider === "qq" && (
                  <p className="text-xs text-muted-foreground">
                    Generate authorization code in QQ Mail Settings → Account →
                    POP3/IMAP/SMTP
                  </p>
                )}
              </div>

              <div className="grid gap-2">
                <Label>Avatar (optional)</Label>
                <div className="flex items-center gap-3">
                  <Avatar className="size-12">
                    {avatarPreview && !avatarPreview.startsWith("selected:") ? (
                      <AvatarImage src={avatarPreview} alt="Avatar preview" />
                    ) : null}
                    <AvatarFallback className="bg-violet-600 text-xs">
                      {form.name ? form.name.slice(0, 2).toUpperCase() : "?"}
                    </AvatarFallback>
                  </Avatar>
                  <div className="flex flex-col gap-1">
                    {avatarPreview?.startsWith("selected:") && (
                      <p className="text-xs text-muted-foreground">
                        Selected: {avatarPreview.replace("selected:", "")}
                      </p>
                    )}
                    <div className="flex gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={selectAvatar}
                        disabled={uploadingAvatar}
                      >
                        <Upload className="mr-1 h-3 w-3" />
                        Upload
                      </Button>
                      {(() => {
                        const existingAvatars = fullAccounts.filter(
                          (a) => a.avatar && avatarUrls[a.credentials.email],
                        );
                        if (existingAvatars.length === 0) return null;
                        return (
                          <Popover>
                            <PopoverTrigger asChild>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={uploadingAvatar}
                              >
                                <Images className="mr-1 h-3 w-3" />
                                Choose existing
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent
                              side="top"
                              align="start"
                              className="w-fit p-3"
                            >
                              <p className="text-xs text-muted-foreground mb-2">
                                Select an avatar:
                              </p>
                              <div className="grid grid-cols-5 gap-2">
                                {existingAvatars.map((a) => (
                                  <button
                                    key={a.credentials.email}
                                    type="button"
                                    className={`rounded-full ring-2 ring-offset-1 transition-all ${
                                      selectedExistingAvatar === a.avatar
                                        ? "ring-violet-600"
                                        : "ring-transparent hover:ring-muted-foreground/30"
                                    }`}
                                    onClick={() => {
                                      setSelectedExistingAvatar(a.avatar!);
                                      setAvatarPreview(
                                        avatarUrls[a.credentials.email],
                                      );
                                      setAvatarFilePath(null);
                                    }}
                                    title={a.name}
                                  >
                                    <Avatar className="size-10">
                                      <AvatarImage
                                        src={avatarUrls[a.credentials.email]}
                                        alt={a.name}
                                      />
                                      <AvatarFallback className="bg-violet-600 text-xs">
                                        {a.name.slice(0, 2).toUpperCase()}
                                      </AvatarFallback>
                                    </Avatar>
                                  </button>
                                ))}
                              </div>
                            </PopoverContent>
                          </Popover>
                        );
                      })()}
                      {avatarPreview && (
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={removeAvatar}
                          disabled={uploadingAvatar}
                        >
                          <X className="mr-1 h-3 w-3" />
                          Remove
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <DialogFooter className="flex-col sm:flex-row gap-2">
              <div className="flex items-center gap-2">
                {testResult &&
                  (testResult.success ? (
                    <span className="flex items-center gap-1 text-sm text-green-600">
                      <CheckCircle className="h-4 w-4" />
                      Connected
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-sm text-red-600">
                      <XCircle className="h-4 w-4" />
                      Failed
                    </span>
                  ))}
                <Button
                  variant="outline"
                  onClick={testConnection}
                  disabled={
                    !form.email || !form.password || !form.imapHost || testing
                  }
                >
                  {testing ? (
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
                    resetForm();
                  }}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  onClick={saveAccount}
                  disabled={!form.name || !form.email || !form.password}
                >
                  {editingName
                    ? t("common.save")
                    : t("settings.accounts.addAccount")}
                </Button>
              </div>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  );
}
