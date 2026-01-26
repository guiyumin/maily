import { useState, useEffect, useCallback } from "react";
import { check } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";

export interface UpdateInfo {
  version: string;
  notes: string;
  date: string;
}

export interface UpdaterState {
  checking: boolean;
  available: boolean;
  downloading: boolean;
  progress: number;
  error: string | null;
  update: UpdateInfo | null;
}

export function useUpdater() {
  const [state, setState] = useState<UpdaterState>({
    checking: false,
    available: false,
    downloading: false,
    progress: 0,
    error: null,
    update: null,
  });

  const checkForUpdates = useCallback(async () => {
    setState((s) => ({ ...s, checking: true, error: null }));
    try {
      const update = await check();
      if (update) {
        setState((s) => ({
          ...s,
          checking: false,
          available: true,
          update: {
            version: update.version,
            notes: update.body || "",
            date: update.date || "",
          },
        }));
        return update;
      } else {
        setState((s) => ({ ...s, checking: false, available: false }));
        return null;
      }
    } catch (error) {
      console.error("[updater] Check failed:", error);
      const msg = error instanceof Error
        ? error.message
        : typeof error === "string"
          ? error
          : JSON.stringify(error);
      setState((s) => ({
        ...s,
        checking: false,
        error: msg || "Failed to check for updates",
      }));
      return null;
    }
  }, []);

  const downloadAndInstall = useCallback(async () => {
    setState((s) => ({ ...s, downloading: true, progress: 0, error: null }));
    try {
      const update = await check();
      if (!update) {
        setState((s) => ({ ...s, downloading: false, error: "No update available" }));
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
              setState((s) => ({
                ...s,
                progress: Math.round((downloaded / contentLength) * 100),
              }));
            }
            break;
          case "Finished":
            setState((s) => ({ ...s, progress: 100 }));
            break;
        }
      });

      // Relaunch the app after update
      await relaunch();
    } catch (error) {
      setState((s) => ({
        ...s,
        downloading: false,
        error: error instanceof Error ? error.message : "Failed to download update",
      }));
    }
  }, []);

  // Check for updates on mount (with slight delay to not block startup)
  useEffect(() => {
    const timer = setTimeout(() => {
      checkForUpdates();
    }, 3000);
    return () => clearTimeout(timer);
  }, [checkForUpdates]);

  return {
    ...state,
    checkForUpdates,
    downloadAndInstall,
  };
}
