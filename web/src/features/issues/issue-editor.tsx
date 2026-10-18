import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  inspectIssueOperation,
  inspectQueueOperation,
  issueOperationsQuery,
  projectIssuesQuery,
  projectQueueQuery,
  queueOperationsQuery,
  syncProjectIssues,
  writeIssue,
  writeQueue,
} from "@/lib/api";

export type IssueDraft = {
  projectID: string;
  projectName: string;
  number: number;
  title: string;
  body: string;
  outcome?: string;
  criteria?: string;
  notes?: string;
  dependencies?: string;
  holdReason?: string;
  checkProfile?: string;
  delivery?: string;
  review?: string;
  metadataChanged?: boolean;
  expectedDigest: string;
  expectedRevision: number;
  requestKey: string;
  queueAfterCreate?: boolean;
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: IssueDraft | null;
  onDraftChange: (draft: IssueDraft | null) => void;
  showPolicy: boolean;
};

function bodyForDraft(value: IssueDraft): string {
  const body = value.number
    ? value.body
    : `## Outcome\n${value.outcome?.trim() || value.title.trim()}\n\n## Acceptance criteria\n${(
        value.criteria ?? ""
      )
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => `- ${line.replace(/^[-*] /, "")}`)
        .join("\n")}\n\n## Implementation notes\n${value.notes?.trim() ?? ""}`;
  if (!value.metadataChanged) return body;
  const dependencies = (value.dependencies ?? "")
    .split(",")
    .map((part) => Number(part.trim()))
    .filter((number) => Number.isInteger(number) && number > 0);
  const settings = `## Outpost settings\n\n\`\`\`yaml\noutpost_task: 1\nkind: implementation\ndepends_on: [${dependencies.join(", ")}]\nhold_reason: ${value.holdReason?.trim() ? JSON.stringify(value.holdReason.trim()) : "null"}\ncheck_profile: ${value.checkProfile || "inherit"}\ndelivery: ${value.delivery || "inherit"}\nreview: ${value.review || "inherit"}\n\`\`\``;
  const start = body.search(/^## Outpost settings\s*$/m);
  if (start < 0) return `${body.trimEnd()}\n\n${settings}\n`;
  const tail = body.slice(start);
  const next = tail.search(/^## (?!Outpost settings)[^\n]+$/m);
  return `${body.slice(0, start).trimEnd()}\n\n${settings}\n${next < 0 ? "" : `\n${tail.slice(next).trimStart()}`}`;
}

export default function IssueEditor({
  open,
  onOpenChange,
  draft,
  onDraftChange,
  showPolicy,
}: Props) {
  const queryClient = useQueryClient();
  const [createdNumber, setCreatedNumber] = useState(0);
  const [queueKey, setQueueKey] = useState("");
  const latestQuery = useQuery({
    ...projectIssuesQuery(draft?.projectID ?? ""),
    enabled: Boolean(draft),
  });
  const latest = latestQuery.data?.issues.find((issue) => issue.number === draft?.number);
  const queue = useQuery({
    ...projectQueueQuery(draft?.projectID ?? ""),
    enabled: Boolean(draft),
  });
  const inspectIssue = useMutation({
    mutationFn: () => inspectIssueOperation(draft?.projectID ?? "", draft?.requestKey ?? ""),
    onSuccess: (receipt) => {
      if (draft && !draft.number && receipt.state === "confirmed" && receipt.result_number) {
        setCreatedNumber(receipt.result_number);
        if (draft.queueAfterCreate) appendIssue(receipt.result_number);
      }
    },
    onSettled: () => {
      if (draft)
        void queryClient.invalidateQueries({
          queryKey: issueOperationsQuery(draft.projectID).queryKey,
        });
    },
  });
  const append = useMutation({
    mutationFn: (input: { projectID: string; number: number; key: string; sha: string }) =>
      writeQueue(input.projectID, {
        request_key: input.key,
        expected_sha: input.sha,
        kind: "append",
        issue: input.number,
      }),
    onSettled: () => {
      if (draft) {
        void queryClient.invalidateQueries({
          queryKey: projectQueueQuery(draft.projectID).queryKey,
        });
        void queryClient.invalidateQueries({
          queryKey: queueOperationsQuery(draft.projectID).queryKey,
        });
      }
    },
  });
  const inspectAppend = useMutation({
    mutationFn: () => inspectQueueOperation(draft?.projectID ?? "", queueKey),
    onSettled: () => {
      if (draft)
        void queryClient.invalidateQueries({
          queryKey: projectQueueQuery(draft.projectID).queryKey,
        });
    },
  });
  const appendIssue = (number: number) => {
    if (!draft || !queue.data?.sha || queue.data.state !== "ready") return;
    const key = crypto.randomUUID();
    setQueueKey(key);
    append.mutate({ projectID: draft.projectID, number, key, sha: queue.data.sha });
  };
  const mutation = useMutation({
    mutationFn: (value: IssueDraft) =>
      writeIssue(value.projectID, {
        request_key: value.requestKey,
        title: value.title.trim(),
        body: bodyForDraft(value),
        ...(value.number
          ? {
              issue_number: value.number,
              expected_digest: value.expectedDigest,
              expected_revision: value.expectedRevision,
            }
          : {}),
      }),
    onSuccess: (receipt) => {
      if (draft && !draft.number && receipt.state === "confirmed" && receipt.result_number) {
        setCreatedNumber(receipt.result_number);
        if (draft.queueAfterCreate) appendIssue(receipt.result_number);
      }
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
  const done = mutation.data?.state === "confirmed" || inspectIssue.data?.state === "confirmed";
  function edit(
    field:
      | "title"
      | "body"
      | "outcome"
      | "criteria"
      | "notes"
      | "dependencies"
      | "holdReason"
      | "checkProfile"
      | "delivery",
    value: string,
  ) {
    if (!draft) return;
    onDraftChange({
      ...draft,
      [field]: value,
      requestKey: "",
      metadataChanged:
        ["dependencies", "holdReason", "checkProfile", "delivery"].includes(field) ||
        draft.metadataChanged,
    });
    mutation.reset();
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft) return;
    const value = draft.requestKey ? draft : { ...draft, requestKey: crypto.randomUUID() };
    onDraftChange(value);
    mutation.mutate(value);
  }
  const partialNumber = !draft.number
    ? createdNumber || mutation.data?.result_number || inspectIssue.data?.result_number || 0
    : 0;
  const alreadyQueued = partialNumber > 0 && queue.data?.issues.includes(partialNumber);
  const queueConfirmed =
    append.data?.state === "confirmed" || inspectAppend.data?.state === "confirmed";
  const dependencyParts = (draft.dependencies ?? "")
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
  const invalidDependencies =
    !/^\s*(\d+\s*(,\s*\d+\s*)*)?$/.test(draft.dependencies ?? "") ||
    dependencyParts.length > 50 ||
    new Set(dependencyParts).size !== dependencyParts.length ||
    dependencyParts.some((part) => Number(part) === draft.number);
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
          {draft.number ? (
            <>
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
            </>
          ) : (
            <>
              <label htmlFor="issue-outcome">Desired outcome</label>
              <textarea
                id="issue-outcome"
                className="issue-body-input"
                value={draft.outcome ?? ""}
                disabled={locked || done}
                onChange={(event) => edit("outcome", event.target.value)}
                rows={3}
                maxLength={20000}
              />
              <label htmlFor="issue-criteria">Acceptance criteria (one per line)</label>
              <textarea
                id="issue-criteria"
                className="issue-body-input"
                value={draft.criteria ?? ""}
                disabled={locked || done}
                onChange={(event) => edit("criteria", event.target.value)}
                rows={5}
                maxLength={50000}
              />
              <details>
                <summary>Optional implementation notes</summary>
                <textarea
                  id="issue-notes"
                  aria-label="Implementation notes"
                  className="issue-body-input"
                  value={draft.notes ?? ""}
                  disabled={locked || done}
                  onChange={(event) => edit("notes", event.target.value)}
                  rows={4}
                  maxLength={50000}
                />
              </details>
            </>
          )}
          {!draft.number && (
            <label className="issue-queue-choice">
              <input
                type="checkbox"
                checked={Boolean(draft.queueAfterCreate)}
                disabled={locked || done}
                onChange={(event) =>
                  onDraftChange({ ...draft, queueAfterCreate: event.target.checked })
                }
              />
              Add to the end of the queue after creating
            </label>
          )}
          <details open={showPolicy ? true : undefined}>
            <summary>Optional dependencies, hold, and policy</summary>
            <label htmlFor="issue-dependencies">
              Dependencies (issue numbers, comma separated)
            </label>
            <Input
              id="issue-dependencies"
              value={draft.dependencies ?? ""}
              disabled={locked || done}
              onChange={(event) => edit("dependencies", event.target.value)}
              placeholder="12, 34"
            />
            {invalidDependencies && (
              <p role="alert" className="form-error">
                Enter distinct issue numbers separated by commas, excluding this issue.
              </p>
            )}
            <label htmlFor="issue-hold">Hold reason</label>
            <Input
              id="issue-hold"
              value={draft.holdReason ?? ""}
              disabled={locked || done}
              maxLength={500}
              onChange={(event) => edit("holdReason", event.target.value)}
              placeholder="Leave empty for no hold"
            />
            <label htmlFor="issue-check-profile">Check profile</label>
            <Input
              id="issue-check-profile"
              value={draft.checkProfile ?? "inherit"}
              disabled={locked || done}
              pattern="[A-Za-z][A-Za-z0-9_-]{0,63}"
              onChange={(event) => edit("checkProfile", event.target.value)}
            />
            <label htmlFor="issue-delivery">Delivery</label>
            <select
              id="issue-delivery"
              value={draft.delivery ?? "inherit"}
              disabled={locked || done}
              onChange={(event) => edit("delivery", event.target.value)}
            >
              <option value="inherit">Project default</option>
              <option value="local_changes">Local changes</option>
              <option value="commit">Commit</option>
              <option value="push">Push</option>
            </select>
            <label htmlFor="issue-review">Review</label>
            <select
              id="issue-review"
              value={draft.review ?? "inherit"}
              disabled={locked || done}
              onChange={(event) => {
                onDraftChange({
                  ...draft,
                  review: event.target.value,
                  metadataChanged: true,
                  requestKey: "",
                });
                mutation.reset();
              }}
            >
              <option value="inherit">Project default</option>
              <option value="none">None</option>
              <option value="required">Required</option>
            </select>
          </details>
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
          {(mutation.isError ||
            mutation.data?.state === "unknown" ||
            mutation.data?.state === "pending") &&
            draft.requestKey && (
              <Button
                type="button"
                variant="outline"
                disabled={inspectIssue.isPending}
                onClick={() => inspectIssue.mutate()}
              >
                Inspect issue operation
              </Button>
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
          {partialNumber > 0 && (
            <div className="issue-conflict" role="status">
              <p>
                Issue #{partialNumber} was created.{" "}
                {alreadyQueued || queueConfirmed
                  ? "It is in the queue."
                  : "It remains in Inbox until the queue write is confirmed."}
              </p>
              {!alreadyQueued && !queueConfirmed && (
                <>
                  {append.data && (
                    <p>
                      Queue: {append.data.state} {append.data.reason}
                    </p>
                  )}
                  {append.isError && (
                    <p>Queue response is uncertain. Inspect it before retrying.</p>
                  )}
                  {(append.isError ||
                    append.data?.state === "unknown" ||
                    append.data?.state === "pending") &&
                  queueKey ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={inspectAppend.isPending}
                      onClick={() => inspectAppend.mutate()}
                    >
                      Inspect queue operation
                    </Button>
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={append.isPending || queue.data?.state !== "ready"}
                      onClick={() => appendIssue(partialNumber)}
                    >
                      Add created issue to queue
                    </Button>
                  )}
                </>
              )}
            </div>
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
                setCreatedNumber(0);
                setQueueKey("");
                mutation.reset();
                append.reset();
                onOpenChange(false);
              }}
            >
              Discard draft
            </Button>
            {!done && (
              <Button
                type="submit"
                disabled={mutation.isPending || !draft.title.trim() || invalidDependencies}
              >
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
