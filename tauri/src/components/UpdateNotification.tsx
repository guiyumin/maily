import { useUpdater } from "@/hooks/useUpdater";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { Download, RefreshCw, Loader2 } from "lucide-react";

export function UpdateNotification() {
  const {
    available,
    downloading,
    progress,
    update,
    downloadAndInstall,
  } = useUpdater();

  if (!available || !update) {
    return null;
  }

  return (
    <Dialog open={available && !downloading}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <RefreshCw className="h-5 w-5" />
            Update Available
          </DialogTitle>
          <DialogDescription>
            A new version of Maily is available.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="rounded-lg bg-muted p-4">
            <p className="text-sm font-medium">Version {update.version}</p>
            {update.notes && (
              <p className="mt-1 text-sm text-muted-foreground">
                {update.notes}
              </p>
            )}
          </div>

          {downloading && (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span>Downloading...</span>
                <span>{progress}%</span>
              </div>
              <Progress value={progress} />
            </div>
          )}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          {!downloading && (
            <>
              <Button variant="outline" onClick={() => {}}>
                Later
              </Button>
              <Button onClick={downloadAndInstall}>
                <Download className="mr-2 h-4 w-4" />
                Update Now
              </Button>
            </>
          )}
          {downloading && (
            <Button disabled>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Installing...
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
