import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { issueOperationsQuery, projectIssuesQuery, syncProjectIssues, writeIssue } from "@/lib/api";

export type IssueDraft = {
  projectID: string;
  projectName: string;
  number: number;
  title: string;
  body: string;
  expectedDigest: string;
  expectedRevision: number;
  requestKey: string;
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: IssueDraft | null;
  onDraftChange: (draft: IssueDraft | null) => void;
};

export default function IssueEditor({ open, onOpenChange, draft, onDraftChange }: Props) {
  const queryClient = useQueryClient();
  const latestQuery = useQuery({
    ...projectIssuesQuery(draft?.projectID ?? ""),
    enabled: Boolean(draft),
  });
  const latest = latestQuery.data?.issues.find((issue) => issue.number === draft?.number);
  const mutation = useMutation({
    mutationFn: (value: IssueDraft) =>
      writeIssue(value.projectID, {
        request_key: value.requestKey,
        title: value.title.trim(),
        body: value.body,
        ...(value.number
          ? {
              issue_number: value.number,
              expected_digest: value.expectedDigest,
              expected_revision: value.expectedRevision,
            }
          : {}),
      }),
    onSuccess: (receipt) => {
      if (draft && (receipt.state === "confirmed" || receipt.state === "conflict")) {
        const projectID = draft.projectID;
        void syncProjectIssues(projectID)
          .then(() =>
            queryClient.invalidateQueries({ queryKey: projectIssuesQuery(projectID).queryKey }),
          )
          .catch(() => undefined);
      }
    },
    onSettled: () => {
      if (draft) {
        void queryClient.invalidateQueries({
          queryKey: projectIssuesQuery(draft.projectID).queryKey,
        });
        void queryClient.invalidateQueries({
          queryKey: issueOperationsQuery(draft.projectID).queryKey,
        });
      }
    },
  });
  if (!draft) return null;
  const locked =
    mutation.isPending ||
    (draft.requestKey !== "" &&
      (mutation.isError ||
        mutation.data?.state === "unknown" ||
        mutation.data?.state === "pending"));
  const done = mutation.data?.state === "confirmed";
  function edit(field: "title" | "body", value: string) {
    if (!draft) return;
    onDraftChange({ ...draft, [field]: value, requestKey: "" });
    mutation.reset();
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft) return;
    const value = draft.requestKey ? draft : { ...draft, requestKey: crypto.randomUUID() };
    onDraftChange(value);
    mutation.mutate(value);
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="issue-editor-dialog">
        <DialogHeader>
          <DialogTitle>{draft.number ? `Edit issue #${draft.number}` : "Create issue"}</DialogTitle>
          <DialogDescription>
            {draft.projectName} · Issues define tasks. Add the issue to the shared queue separately
            when it should run.
          </DialogDescription>
        </DialogHeader>
        <form className="repository-form" onSubmit={submit}>
          <label htmlFor="issue-title">Title</label>
          <Input
            id="issue-title"
            value={draft.title}
            required
            maxLength={500}
            disabled={locked || done}
            onChange={(event) => edit("title", event.target.value)}
          />
          <label htmlFor="issue-body">Specification</label>
          <textarea
            id="issue-body"
            className="issue-body-input"
            value={draft.body}
            disabled={locked || done}
            onChange={(event) => edit("body", event.target.value)}
            rows={13}
            maxLength={262144}
          />
          {draft.number > 0 && (
            <p className="form-hint">
              Editing source revision {draft.expectedRevision}. A detected change keeps this draft
              here for comparison.
            </p>
          )}
          {mutation.isError && (
            <p className="form-error" role="alert">
              Request outcome is uncertain. Inspect using the same request key before making another
              change.
            </p>
          )}
          {mutation.data && (
            <p
              className={mutation.data.state === "confirmed" ? "form-hint" : "form-error"}
              role="status"
            >
              {mutation.data.state === "confirmed"
                ? `Saved as issue #${mutation.data.result_number}.`
                : `${mutation.data.state.replaceAll("_", " ")}: ${mutation.data.reason}`}
            </p>
          )}
          {mutation.data?.state === "conflict" &&
            latest &&
            latest.content_digest !== draft.expectedDigest && (
              <div className="issue-conflict">
                <details>
                  <summary>Review the latest issue before replacing it</summary>
                  <strong>{latest.title}</strong>
                  <pre>{latest.body}</pre>
                </details>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    onDraftChange({
                      ...draft,
                      expectedDigest: latest.content_digest,
                      expectedRevision: latest.local_revision,
                      requestKey: "",
                    });
                    mutation.reset();
                  }}
                >
                  Use latest revision for this draft
                </Button>
              </div>
            )}
          <div className="form-actions">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Close
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                onDraftChange(null);
                mutation.reset();
                onOpenChange(false);
              }}
            >
              Discard draft
            </Button>
            {!done && (
              <Button type="submit" disabled={mutation.isPending || !draft.title.trim()}>
                {mutation.isPending
                  ? "Checking GitHub…"
                  : locked
                    ? "Inspect operation"
                    : draft.number
                      ? "Save issue"
                      : "Create issue"}
              </Button>
            )}
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
