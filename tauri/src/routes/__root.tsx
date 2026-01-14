import { createRootRoute, Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { UpdateNotification } from "@/components/UpdateNotification";

export const Route = createRootRoute({
  component: () => (
    <>
      <Outlet />
      <UpdateNotification />
      <TanStackRouterDevtools />
    </>
  ),
});
