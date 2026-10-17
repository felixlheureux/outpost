import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Link, Outlet, useRouter } from "@tanstack/react-router";
import { Shell } from "@/components/shell";
import { Button } from "@/components/ui/button";
import { bootstrapQuery, projectsQuery } from "@/lib/api";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(bootstrapQuery),
      context.queryClient.ensureQueryData(projectsQuery),
    ]),
  component: () => (
    <Shell>
      <Outlet />
    </Shell>
  ),
  pendingComponent: () => (
    <main className="loading-page" role="status">
      Connecting to Outpost…
    </main>
  ),
  errorComponent: ConnectionError,
  notFoundComponent: () => (
    <section className="empty-state">
      <h1>Page not found</h1>
      <p>This address does not point to an Outpost page.</p>
      <Link to="/">Return to Queue</Link>
    </section>
  ),
});

function ConnectionError() {
  const router = useRouter();
  return (
    <main className="loading-page">
      <div role="alert">
        <h1>Couldn’t load Outpost</h1>
        <p>Check that the local backend is running, then try again.</p>
        <Button onClick={() => void router.invalidate()}>Try again</Button>
      </div>
    </main>
  );
}
