import { useMutation } from "@tanstack/react-query";
import type { FormEvent } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { addRepository } from "@/lib/api";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  url: string;
  onURLChange: (url: string) => void;
};

export default function AddRepositoryDialog({ open, onOpenChange, url, onURLChange }: Props) {
  const mutation = useMutation({ mutationFn: addRepository });
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    mutation.mutate(url.trim());
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add repository</DialogTitle>
          <DialogDescription>
            Paste a GitHub URL. Outpost will use the host’s existing GitHub login.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="repository-form">
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
          <p id="setup-availability" className="form-hint">
            This starter has the setup form. Managed cloning and source setup arrive in OP-003;
            submitting won’t create a project yet.
          </p>
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
              {mutation.isPending ? "Checking…" : "Add repository"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
