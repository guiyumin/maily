import { createRootRoute, Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { UpdateNotification } from "@/components/UpdateNotification";
import { LocaleProvider } from "@/lib/i18n";
import { useEffect, useState } from "react";
import { invoke } from "@tauri-apps/api/core";

interface Config {
  language?: string;
}

function RootComponent() {
  const [locale, setLocale] = useState("en");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    invoke<Config>("get_config")
      .then((config) => {
        if (config.language) {
          setLocale(config.language);
        }
      })
      .catch(console.error)
      .finally(() => setLoaded(true));
  }, []);

  // Show nothing until config is loaded to avoid flash of wrong language
  if (!loaded) {
    return null;
  }

  return (
    <LocaleProvider locale={locale}>
      <Outlet />
      <UpdateNotification />
      <TanStackRouterDevtools position="top-right" />
    </LocaleProvider>
  );
}

export const Route = createRootRoute({
  component: RootComponent,
});
