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
} from "lucide-react";
import { toast } from "sonner";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import type { FullAccount } from "@/types/account";
import { useLocale } from "@/lib/i18n";

interface AccountsSettingsProps {
  accounts: FullAccount[];
  onAccountsChange: (accounts: FullAccount[]) => void;
}

const PROVIDER_DEFAULTS = {
  gmail: {
    imap_host: "imap.gmail.com",
    imap_port: 993,
    smtp_host: "smtp.gmail.com",
    smtp_port: 587,
  },
  yahoo: {
    imap_host: "imap.mail.yahoo.com",
    imap_port: 993,
    smtp_host: "smtp.mail.yahoo.com",
    smtp_port: 587,
  },
  qq: {
    imap_host: "imap.qq.com",
    imap_port: 993,
    smtp_host: "smtp.qq.com",
    smtp_port: 465,
  },
};

export function AccountsSettings({
  accounts,
  onAccountsChange,
}: AccountsSettingsProps) {
  const { t } = useLocale();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [provider, setProvider] = useState<"gmail" | "yahoo" | "qq">("gmail");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [imapHost, setImapHost] = useState("imap.gmail.com");
  const [imapPort, setImapPort] = useState(993);
  const [smtpHost, setSmtpHost] = useState("smtp.gmail.com");
  const [smtpPort, setSmtpPort] = useState(587);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    error?: string;
  } | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const [avatarFilePath, setAvatarFilePath] = useState<string | null>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);

  // Use shared store for avatar URLs
  const { avatarUrls, setAvatarUrl, removeAvatarUrl, loadAvatarUrls } = useAccountsStore();

  // Load avatar URLs for all accounts on mount
  useEffect(() => {
    loadAvatarUrls();
  }, [accounts, loadAvatarUrls]);

  const resetForm = () => {
    setEditingName(null);
    setProvider("gmail");
    setName("");
    setEmail("");
    setPassword("");
    setImapHost("imap.gmail.com");
    setImapPort(993);
    setSmtpHost("smtp.gmail.com");
    setSmtpPort(587);
    setTestResult(null);
    setShowPassword(false);
    setAvatarPreview(null);
    setAvatarFilePath(null);
  };

  const openEditDialog = async (account: FullAccount) => {
    setEditingName(account.name);
    const accountProvider = account.provider as "gmail" | "yahoo" | "qq";
    setProvider(accountProvider);
    setName(account.name);
    setEmail(account.credentials.email);
    setPassword(account.credentials.password);
    setImapHost(account.credentials.imap_host);
    setImapPort(account.credentials.imap_port);
    setSmtpHost(account.credentials.smtp_host);
    setSmtpPort(account.credentials.smtp_port);
    setTestResult(null);
    setShowPassword(false);
    setAvatarFilePath(null);

    // Load existing avatar if any
    if (account.avatar && avatarUrls[account.credentials.email]) {
      setAvatarPreview(avatarUrls[account.credentials.email]);
    } else {
      setAvatarPreview(null);
    }

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

        // For preview, we'll create an object URL. Since we can't read the file
        // directly in frontend without fs plugin, we'll just show a placeholder
        // and the actual image after upload. For now, use a simple indicator.
        setAvatarFilePath(filePath);
        // Show filename as indicator since we can't preview without fs plugin
        setAvatarPreview(`selected:${filePath.split("/").pop() || filePath.split("\\").pop()}`);
      }
    } catch (err) {
      toast.error(`Failed to select image: ${err}`);
    }
  };

  const removeAvatar = () => {
    setAvatarPreview(null);
    setAvatarFilePath(null);
  };

  const handleProviderChange = (newProvider: "gmail" | "yahoo" | "qq") => {
    setProvider(newProvider);
    const defaults = PROVIDER_DEFAULTS[newProvider];
    setImapHost(defaults.imap_host);
    setImapPort(defaults.imap_port);
    setSmtpHost(defaults.smtp_host);
    setSmtpPort(defaults.smtp_port);
  };

  const testConnection = async () => {
    if (!email || !password || !imapHost) return;

    setTesting(true);
    setTestResult(null);

    try {
      await invoke("test_account", {
        email,
        password,
        imapHost,
        imapPort,
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
    if (!name || !email || !password) return;

    const account: FullAccount = {
      name,
      provider,
      credentials: {
        email,
        password,
        imap_host: imapHost,
        imap_port: imapPort,
        smtp_host: smtpHost,
        smtp_port: smtpPort,
      },
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

      // Handle avatar upload/removal
      if (avatarFilePath) {
        // Upload new avatar
        setUploadingAvatar(true);
        try {
          newAccounts = await invoke<FullAccount[]>("upload_account_avatar", {
            accountName: name,
            filePath: avatarFilePath,
          });
          // Refresh avatar URL in shared store
          const dataUrl = await invoke<string | null>("get_account_avatar_data_url", {
            accountName: name,
          });
          if (dataUrl) {
            setAvatarUrl(email, dataUrl);
          }
        } catch (err) {
          toast.error(`Failed to upload avatar: ${err}`);
        } finally {
          setUploadingAvatar(false);
        }
      } else if (editingName && avatarPreview === null) {
        // Remove existing avatar if preview was cleared
        const existingAccount = accounts.find((a) => a.name === editingName);
        if (existingAccount?.avatar) {
          try {
            newAccounts = await invoke<FullAccount[]>("delete_account_avatar", {
              accountName: name,
            });
            removeAvatarUrl(email);
          } catch (err) {
            toast.error(`Failed to remove avatar: ${err}`);
          }
        }
      }

      onAccountsChange(newAccounts);
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
      onAccountsChange(newAccounts);
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
        {accounts.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("settings.accounts.noAccounts")}
          </p>
        ) : (
          <div className="space-y-2">
            {accounts.map((account) => (
              <div
                key={account.name}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div className="flex items-center gap-3">
                  <Avatar className="size-10">
                    {avatarUrls[account.credentials.email] && (
                      <AvatarImage src={avatarUrls[account.credentials.email]} alt={account.name} />
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
                <Select value={provider} onValueChange={handleProviderChange}>
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
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="account_email">Email</Label>
                <Input
                  id="account_email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="account_password">
                  {provider === "qq" ? "Authorization Code" : "App Password"}
                </Label>
                <div className="relative">
                  <Input
                    id="account_password"
                    type={showPassword ? "text" : "password"}
                    placeholder={
                      provider === "qq" ? "Authorization code" : "App password"
                    }
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
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
                {provider === "gmail" && (
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
                {provider === "qq" && (
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
                      {name ? name.slice(0, 2).toUpperCase() : "?"}
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
                        {avatarPreview ? "Change" : "Upload"}
                      </Button>
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
                  disabled={!email || !password || !imapHost || testing}
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
                  disabled={!name || !email || !password}
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
