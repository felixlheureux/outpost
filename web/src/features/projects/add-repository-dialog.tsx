import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { addExistingRepository, addRepository, projectsQuery } from "@/lib/api";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  url: string;
  onURLChange: (url: string) => void;
};

export default function AddRepositoryDialog({ open, onOpenChange, url, onURLChange }: Props) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: projectData } = useQuery(projectsQuery);
  const [existing, setExisting] = useState(false);
  const [checkoutPath, setCheckoutPath] = useState("");
  const mutation = useMutation({
    mutationFn: () =>
      existing ? addExistingRepository(checkoutPath.trim()) : addRepository(url.trim()),
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey }),
  });
  useEffect(() => {
    if (!mutation.isPending) return;
    const timer = window.setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey });
    }, 1500);
    return () => window.clearInterval(timer);
  }, [mutation.isPending, queryClient]);
  const normalizedURL = url
    .trim()
    .replace(/\.git\/?$/i, "")
    .replace(/\/$/, "")
    .toLowerCase();
  const activeSetup = projectData?.projects.find(
    (project) => project.repository_url.toLowerCase() === normalizedURL,
  );
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    mutation.mutate(undefined, {
      onSuccess: async (project) => {
        await queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey });
        await navigate({ to: "/", search: { project: project.id } });
        onURLChange("");
        setCheckoutPath("");
        onOpenChange(false);
      },
    });
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add repository</DialogTitle>
          <DialogDescription>
            {existing
              ? "Select an existing Git checkout on the Outpost host. Its files stay in place."
              : "Paste a GitHub URL. Outpost will use the host’s existing GitHub login."}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="repository-form">
          {existing ? (
            <>
              <label htmlFor="checkout-path">Checkout path on host</label>
              <Input
                id="checkout-path"
                type="text"
                placeholder="/home/you/code/repository"
                autoComplete="off"
                required
                value={checkoutPath}
                onChange={(event) => {
                  setCheckoutPath(event.target.value);
                  mutation.reset();
                }}
                aria-describedby="setup-availability"
              />
            </>
          ) : (
            <>
              <label htmlFor="repository-url">GitHub repository</label>
              <Input
                id="repository-url"
                type="url"
                placeholder="https://github.com/owner/repo"
                autoComplete="off"
                required
                value={url}
                onChange={(event) => {
                  onURLChange(event.target.value);
                  mutation.reset();
                }}
                aria-describedby="setup-availability"
              />
            </>
          )}
          <p id="setup-availability" className="form-hint">
            {existing
              ? "Outpost checks the GitHub origin and preserves local changes. Issue and queue setup follows registration."
              : "Outpost uses the host GitHub login and keeps an interrupted checkout for inspection. Issue and queue setup follows the clone."}
          </p>
          {!existing && mutation.isPending && activeSetup && (
            <p role="status" className="form-hint">
              {activeSetup.name}: {activeSetup.setup_stage.replaceAll("_", " ")}
            </p>
          )}
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setExisting(!existing);
              mutation.reset();
            }}
          >
            {existing ? "Clone from GitHub instead" : "Use an existing checkout"}
          </Button>
          {mutation.isError && (
            <p className="form-error" role="alert">
              {mutation.error.message}
            </p>
          )}
          <div className="form-actions">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? existing
                  ? "Inspecting checkout…"
                  : "Inspecting and cloning…"
                : "Add repository"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
