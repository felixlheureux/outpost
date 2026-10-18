import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  type IssueSource,
  inspectQueueOperation,
  projectQueueQuery,
  queueOperationsQuery,
  syncProjectQueue,
  writeQueue,
} from "@/lib/api";

type Edit = {
  request_key: string;
  expected_sha: string;
  kind: "append" | "remove" | "move";
  issue: number;
  before?: number;
};

export function QueueManifest({
  projectID,
  repositoryURL,
  issues,
}: {
  projectID: string;
  repositoryURL: string;
  issues: IssueSource[];
}) {
  const client = useQueryClient();
  const queue = useQuery(projectQueueQuery(projectID));
  const operations = useQuery(queueOperationsQuery(projectID));
  const [pendingEdit, setPendingEdit] = useState<Edit | null>(null);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: projectQueueQuery(projectID).queryKey });
    void client.invalidateQueries({ queryKey: queueOperationsQuery(projectID).queryKey });
  };
  const sync = useMutation({ mutationFn: () => syncProjectQueue(projectID), onSettled: refresh });
  const write = useMutation({
    mutationFn: (edit: Edit) => writeQueue(projectID, edit),
    onSuccess: () => setPendingEdit(null),
    onSettled: refresh,
  });
  const inspect = useMutation({
    mutationFn: (key: string) => inspectQueueOperation(projectID, key),
    onSettled: refresh,
  });
  const submit = (kind: Edit["kind"], issue: number, before?: number) => {
    if (!queue.data?.sha) return;
    const edit = {
      request_key: crypto.randomUUID(),
      expected_sha: queue.data.sha,
      kind,
      issue,
      before,
    };
    setPendingEdit(edit);
    write.mutate(edit);
  };
  const byNumber = new Map(issues.map((issue) => [issue.number, issue]));
  const order = queue.data?.issues ?? [];
  const members = new Set(order);
  return (
    <section className="issue-source-panel" aria-label="Shared planning queue">
      <div className="issue-source-heading">
        <div>
          <p className="eyebrow">Planning branch</p>
          <h2 className="issue-source-title">Shared queue</h2>
        </div>
        <Button variant="outline" disabled={sync.isPending} onClick={() => sync.mutate()}>
          {sync.isPending ? "Syncing…" : "Sync queue"}
        </Button>
      </div>
      <p className="issue-source-note">
        {queue.data?.state === "ready"
          ? `${order.length} queued · ${queue.data.sha.slice(0, 12)} · fetched ${queue.data.fetched_at}`
          : queue.data?.last_error || "Planning queue has not been synchronized yet."}
      </p>
      {queue.isError && <p role="alert">{queue.error.message}</p>}
      {sync.isError && <p role="alert">{sync.error.message}</p>}
      {write.isError && pendingEdit && (
        <p role="alert">
          The edit response was lost. Inspect operation {pendingEdit.request_key} before another
          edit.{" "}
          <Button variant="outline" onClick={() => inspect.mutate(pendingEdit.request_key)}>
            Inspect GitHub
          </Button>
        </p>
      )}
      {queue.data?.state === "ready" && (
        <div className="issue-list">
          {order.length === 0 && <p>Queue is empty. Add an issue below.</p>}
          {order.map((number, index) => (
            <div className="issue-row" key={number}>
              <span>
                {index + 1}. #{number}
              </span>
              {byNumber.get(number) ? (
                <a href={byNumber.get(number)?.url} target="_blank" rel="noreferrer">
                  <strong>{byNumber.get(number)?.title}</strong>
                </a>
              ) : (
                <a href={`${repositoryURL}/issues/${number}`} target="_blank" rel="noreferrer">
                  <strong>Open issue on GitHub · not in the current local cache</strong>
                </a>
              )}
              <Button
                size="sm"
                variant="ghost"
                disabled={write.isPending || index === 0}
                onClick={() => submit("move", number, order[index - 1])}
              >
                Up
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={write.isPending || index === order.length - 1}
                onClick={() =>
                  submit("move", number, index + 2 < order.length ? order[index + 2] : undefined)
                }
              >
                Down
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={write.isPending}
                onClick={() => submit("remove", number)}
              >
                Remove
              </Button>
            </div>
          ))}
          {issues
            .filter((issue) => issue.state === "open" && !members.has(issue.number))
            .map((issue) => (
              <div className="issue-row" key={issue.number}>
                <span>Inbox #{issue.number}</span>
                <a href={issue.url} target="_blank" rel="noreferrer">
                  <strong>{issue.title}</strong>
                </a>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={write.isPending}
                  onClick={() => submit("append", issue.number)}
                >
                  Add to queue
                </Button>
              </div>
            ))}
        </div>
      )}
      {operations.data?.operations.map((operation) => (
        <div className="issue-receipt" key={operation.id}>
          <span>
            {operation.kind} {operation.intended.issue ? `#${operation.intended.issue}` : "order"}:{" "}
            {operation.state}
          </span>
          {operation.reason && <small>{operation.reason}</small>}
          {(operation.state === "unknown" || operation.state === "pending") && (
            <Button
              size="sm"
              variant="outline"
              disabled={inspect.isPending}
              onClick={() => inspect.mutate(operation.request_key)}
            >
              Inspect GitHub
            </Button>
          )}
          {operation.state === "conflict" && (
            <small>Current order: {operation.current_issues.map((n) => `#${n}`).join(", ")}</small>
          )}
        </div>
      ))}
      {inspect.isError && <p role="alert">{inspect.error.message}</p>}
    </section>
  );
}
