import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Github, Eye, EyeOff } from "lucide-react";
import type { Config, IntegrationsConfig, GitHubConfig } from "./types";

interface IntegrationsSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function IntegrationsSettings({ config, onUpdate }: IntegrationsSettingsProps) {
  const [showGitHubToken, setShowGitHubToken] = useState(false);

  const integrations = config.integrations ?? {};

  const updateIntegrations = (updates: Partial<IntegrationsConfig>) => {
    onUpdate({
      integrations: { ...integrations, ...updates },
    });
  };

  const updateGitHub = (updates: Partial<GitHubConfig>) => {
    const currentGitHub = integrations.github ?? {
      enabled: false,
      token: "",
      parse_emails: true,
    };
    updateIntegrations({
      github: { ...currentGitHub, ...updates },
    });
  };

  return (
    <div className="space-y-6">
      {/* GitHub Integration */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Github className="h-5 w-5" />
            GitHub
          </CardTitle>
          <CardDescription>
            Enhanced GitHub notifications and PR actions
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>Enable GitHub Integration</Label>
              <p className="text-sm text-muted-foreground">
                Parse GitHub emails and show PR status
              </p>
            </div>
            <Switch
              checked={integrations.github?.enabled ?? false}
              onCheckedChange={(checked) => updateGitHub({ enabled: checked })}
            />
          </div>

          {integrations.github?.enabled && (
            <>
              <div className="grid gap-2">
                <Label htmlFor="github_token">Personal Access Token</Label>
                <div className="relative">
                  <Input
                    id="github_token"
                    type={showGitHubToken ? "text" : "password"}
                    placeholder="ghp_xxxxxxxxxxxx"
                    value={integrations.github?.token ?? ""}
                    onChange={(e) => updateGitHub({ token: e.target.value })}
                    className="pr-10"
                  />
                  <button
                    type="button"
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    onClick={() => setShowGitHubToken(!showGitHubToken)}
                  >
                    {showGitHubToken ? (
                      <EyeOff className="h-4 w-4" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                  </button>
                </div>
                <p className="text-sm text-muted-foreground">
                  Token with <code className="bg-muted px-1 rounded">repo</code> and{" "}
                  <code className="bg-muted px-1 rounded">notifications</code> scopes.{" "}
                  <a
                    href="https://github.com/settings/tokens/new"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    Create token
                  </a>
                </p>
              </div>

              <div className="flex items-center justify-between pl-4 border-l-2 border-muted">
                <div className="space-y-0.5">
                  <Label>Parse GitHub Emails</Label>
                  <p className="text-sm text-muted-foreground">
                    Auto-detect PR/issue emails and show status
                  </p>
                </div>
                <Switch
                  checked={integrations.github?.parse_emails ?? true}
                  onCheckedChange={(checked) => updateGitHub({ parse_emails: checked })}
                />
              </div>

              <div className="rounded-md bg-muted/50 p-4 text-sm text-muted-foreground">
                <p className="font-medium text-foreground mb-2">Coming soon:</p>
                <ul className="list-disc list-inside space-y-1">
                  <li>Show CI status inline</li>
                  <li>Quick actions: approve, request changes, merge</li>
                  <li>Group notifications by repo/PR</li>
                  <li>PRs waiting for your review</li>
                </ul>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Future integrations placeholder */}
      <Card className="border-dashed">
        <CardHeader>
          <CardTitle className="text-muted-foreground">More Integrations</CardTitle>
          <CardDescription>
            Coming soon: Linear, Jira, Slack, Discord
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}
