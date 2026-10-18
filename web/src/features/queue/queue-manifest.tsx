import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { useIssueEditor } from "@/components/shell";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  type IssueSource,
  inspectQueueOperation,
  previewAdmission,
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
function saved(key: string): string | null {
  try {
    return sessionStorage.getItem(key);
  } catch {
    return null;
  }
}
function remember(key: string, value: string) {
  try {
    sessionStorage.setItem(key, value);
  } catch {
    /* Optional browsing context. */
  }
}

function UnavailableSetting({ id, label }: { id: string; label: string }) {
  return (
    <div className="start-field">
      <Label htmlFor={id}>{label}</Label>
      <Select value="unavailable" disabled>
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="unavailable">Provider not connected</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function blocker(issue: IssueSource | undefined, issues: Map<number, IssueSource>): string | null {
  if (!issue) return "Issue has not been observed locally";
  if (issue.stale) return "Issue was not seen in the latest sync";
  if (issue.state === "closed") return "Closed on GitHub · completion is unverified";
  if (issue.parse_error) return issue.parse_error;
  if (issue.requirements?.settings.hold_reason)
    return `On hold: ${issue.requirements.settings.hold_reason}`;
  const check = issue.requirements?.settings.check_profile;
  if (check && check !== "inherit") return `Check profile “${check}” is not configured`;
  for (const number of issue.requirements?.settings.depends_on ?? []) {
    if (!issues.has(number)) return `Dependency #${number} is not in the local source`;
    // Verified completion is owned by the scheduler; GitHub closure is insufficient.
    return `Waiting for verified completion of #${number}`;
  }
  return null;
}

export function QueueManifest({
  projectID,
  projectName,
  repositoryURL,
  issues,
  currentRun,
}: {
  projectID: string;
  projectName: string;
  repositoryURL: string;
  issues: IssueSource[];
  currentRun: { project_id: string; state: string } | null;
}) {
  const client = useQueryClient();
  const openEditor = useIssueEditor();
  const queue = useQuery(projectQueueQuery(projectID));
  const operations = useQuery(queueOperationsQuery(projectID));
  const contextKey = `outpost.queue.${projectID}`;
  const [tab, setTab] = useState<"queued" | "inbox">(() =>
    saved(`${contextKey}.tab`) === "inbox" ? "inbox" : "queued",
  );
  const [selected, setSelected] = useState<number | null>(() => {
    const value = Number(saved(`${contextKey}.selected`));
    return value > 0 ? value : null;
  });
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = Number(saved(`${contextKey}.scroll`)) || 0;
  }, [contextKey]);
  useEffect(() => remember(`${contextKey}.tab`, tab), [contextKey, tab]);
  useEffect(
    () => remember(`${contextKey}.selected`, String(selected ?? "")),
    [contextKey, selected],
  );
  const [pendingEdit, setPendingEdit] = useState<Edit | null>(null);
  const [message, setMessage] = useState("");
  const [scope, setScope] = useState<"follow" | "fixed">(() =>
    saved(`${contextKey}.scope`) === "fixed" ? "fixed" : "follow",
  );
  const [taskLimit, setTaskLimit] = useState(() => {
    const value = Number(saved(`${contextKey}.taskLimit`));
    return Number.isInteger(value) && value >= 1 && value <= 100 ? value : 20;
  });
  const [runtimeBudget, setRuntimeBudget] = useState(
    () => saved(`${contextKey}.runtimeBudget`) ?? "",
  );
  const [instructions, setInstructions] = useState(() => saved(`${contextKey}.instructions`) ?? "");
  useEffect(() => remember(`${contextKey}.scope`, scope), [contextKey, scope]);
  useEffect(() => remember(`${contextKey}.taskLimit`, String(taskLimit)), [contextKey, taskLimit]);
  useEffect(
    () => remember(`${contextKey}.runtimeBudget`, runtimeBudget),
    [contextKey, runtimeBudget],
  );
  useEffect(() => remember(`${contextKey}.instructions`, instructions), [contextKey, instructions]);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: projectQueueQuery(projectID).queryKey });
    void client.invalidateQueries({ queryKey: queueOperationsQuery(projectID).queryKey });
  };
  const sync = useMutation({ mutationFn: () => syncProjectQueue(projectID), onSettled: refresh });
  const write = useMutation({
    mutationFn: (edit: Edit) => writeQueue(projectID, edit),
    onSuccess: (receipt) => {
      setMessage(
        `${receipt.state}: ${receipt.reason || `${receipt.kind} #${receipt.intended.issue}`}`,
      );
      if (receipt.state === "confirmed") setPendingEdit(null);
    },
    onSettled: refresh,
  });
  const inspect = useMutation({
    mutationFn: (key: string) => inspectQueueOperation(projectID, key),
    onSuccess: (receipt) => {
      setMessage(`${receipt.state}: ${receipt.reason || "Remote order inspected"}`);
      if (receipt.state === "confirmed") setPendingEdit(null);
    },
    onSettled: refresh,
  });
  const order = queue.data?.issues ?? [];
  const byNumber = new Map(issues.map((issue) => [issue.number, issue]));
  const members = new Set(order);
  const inbox = issues.filter((issue) => issue.state === "open" && !members.has(issue.number));
  const rows = tab === "queued" ? order : inbox.map((issue) => issue.number);
  const selectedIssue = selected === null ? undefined : byNumber.get(selected);
  const selectedPosition = selected === null ? -1 : order.indexOf(selected);
  const next =
    queue.data?.state === "ready"
      ? order.find((number) => !blocker(byNumber.get(number), byNumber))
      : undefined;
  const fixedIssue = selected !== null && order.includes(selected) ? selected : next;
  const preview = useMutation({
    mutationFn: (input: { mode: "follow" | "fixed"; selected: number }) =>
      previewAdmission(projectID, input.mode, input.selected),
  });
  const previewCurrent =
    preview.variables?.mode === scope &&
    preview.variables.selected === (scope === "fixed" ? (fixedIssue ?? 0) : 0);
  const editingBlocked = write.isPending || Boolean(pendingEdit);
  const editReceipt = inspect.data ?? write.data;
  function submit(kind: Edit["kind"], issue: number, before?: number) {
    if (!queue.data?.sha || editingBlocked) return;
    inspect.reset();
    const edit = {
      request_key: crypto.randomUUID(),
      expected_sha: queue.data.sha,
      kind,
      issue,
      before,
    };
    setMessage("");
    setPendingEdit(edit);
    write.mutate(edit);
  }
  function retryAfterConflict() {
    if (!pendingEdit || queue.data?.state !== "ready") return;
    inspect.reset();
    const edit = { ...pendingEdit, request_key: crypto.randomUUID(), expected_sha: queue.data.sha };
    setPendingEdit(edit);
    setMessage("");
    write.mutate(edit);
  }
  const issueURL = (number: number) =>
    byNumber.get(number)?.url ?? `${repositoryURL}/issues/${number}`;
  return (
    <section className="queue-workspace" aria-label="Tasks">
      <div className="queue-toolbar">
        <div role="tablist" aria-label="Task list" className="queue-tabs">
          <button
            type="button"
            role="tab"
            aria-selected={tab === "queued"}
            onClick={() => setTab("queued")}
          >
            Queued <span>{order.length}</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === "inbox"}
            onClick={() => setTab("inbox")}
          >
            Inbox <span>{inbox.length}</span>
          </button>
        </div>
        <div className="queue-sync">
          <span>{queue.data?.state === "ready" ? "GitHub queue synced" : "Queue needs sync"}</span>
          <Button size="sm" variant="ghost" disabled={sync.isPending} onClick={() => sync.mutate()}>
            {sync.isPending ? "Syncing…" : "Sync queue"}
          </Button>
        </div>
      </div>
      {queue.data?.state !== "ready" && queue.data?.last_error && (
        <p role="status" className="queue-feedback">
          {queue.data.last_error}
        </p>
      )}
      {(queue.isError || sync.isError) && (
        <p role="alert" className="form-error">
          {queue.error?.message || sync.error?.message}
        </p>
      )}
      {message && (
        <p role="status" className="queue-feedback">
          {message}
        </p>
      )}
      {write.isError && pendingEdit && (
        <p role="alert" className="form-error">
          Queue write outcome is uncertain. Inspect the remote operation before another edit.
        </p>
      )}
      {pendingEdit && !write.isPending && (
        <div className="queue-reconcile">
          {(write.isError ||
            editReceipt?.state === "unknown" ||
            editReceipt?.state === "pending") && (
            <Button
              size="sm"
              variant="outline"
              disabled={inspect.isPending}
              onClick={() => inspect.mutate(pendingEdit.request_key)}
            >
              Inspect GitHub
            </Button>
          )}
          {(editReceipt?.state === "conflict" || editReceipt?.state === "rejected") && (
            <>
              <span>
                Intended {pendingEdit.kind} for #{pendingEdit.issue}. Current order:{" "}
                {(editReceipt.current_issues ?? order).map((n) => `#${n}`).join(", ") || "empty"}.
              </span>
              <Button
                size="sm"
                variant="outline"
                disabled={queue.isFetching}
                onClick={retryAfterConflict}
              >
                Apply to latest order
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setPendingEdit(null)}>
                Discard intent
              </Button>
            </>
          )}
        </div>
      )}
      <div className="queue-grid">
        <div
          className="queue-list"
          role="tabpanel"
          ref={listRef}
          onScroll={(event) =>
            remember(`${contextKey}.scroll`, String(event.currentTarget.scrollTop))
          }
        >
          {!queue.data && <p className="queue-empty">Loading shared queue…</p>}
          {queue.data && queue.data.state !== "ready" && (
            <p className="queue-empty">
              Queue is unavailable. Sync or resolve the source issue before editing order.
            </p>
          )}
          {queue.data?.state === "ready" && rows.length === 0 && (
            <p className="queue-empty">
              {tab === "queued"
                ? "No tasks queued. Add an Inbox issue when it is ready to run."
                : "Inbox is empty. Create a task or sync GitHub issues."}
            </p>
          )}
          {queue.data?.state === "ready" &&
            rows.map((number, index) => {
              const issue = byNumber.get(number);
              const reason = blocker(issue, byNumber);
              return (
                <button
                  type="button"
                  className={`queue-task ${selected === number ? "selected" : ""}`}
                  aria-pressed={selected === number}
                  key={number}
                  onClick={() => setSelected(number)}
                >
                  <span className="queue-position">{tab === "queued" ? index + 1 : "·"}</span>
                  <span className="queue-task-main">
                    <strong>
                      #{number} {issue?.title ?? "Issue not observed"}
                    </strong>
                    <small>{reason ?? (number === next ? "Next eligible" : "Eligible")}</small>
                  </span>
                </button>
              );
            })}
        </div>
        <aside className="queue-inspector" aria-label="Task inspector">
          {selected === null ? (
            <p className="queue-empty">Select a task to inspect its source and queue actions.</p>
          ) : (
            <>
              <div className="queue-inspector-heading">
                <span className="eyebrow">
                  {selectedPosition >= 0 ? `Queued · position ${selectedPosition + 1}` : "Inbox"}
                </span>
                <Button size="sm" variant="ghost" onClick={() => setSelected(null)}>
                  Close details
                </Button>
              </div>
              <h2>
                #{selected} {selectedIssue?.title ?? "Issue not observed"}
              </h2>
              <p className="queue-status">
                {blocker(selectedIssue, byNumber) ??
                  (selected === next ? "Next eligible" : "Eligible")}
              </p>
              {selectedIssue ? (
                <>
                  <p>
                    {selectedIssue.requirements?.outcome ||
                      selectedIssue.body ||
                      selectedIssue.title}
                  </p>
                  {Boolean(selectedIssue.requirements?.criteria.length) && (
                    <>
                      <h3>Acceptance criteria</h3>
                      <ul>
                        {selectedIssue.requirements?.criteria.map((item) => (
                          <li key={item}>{item}</li>
                        ))}
                      </ul>
                    </>
                  )}
                  {Boolean(selectedIssue.requirements?.settings.depends_on?.length) && (
                    <>
                      <h3>Dependencies</h3>
                      <p>
                        {selectedIssue.requirements?.settings.depends_on?.map((number) => (
                          <a key={number} href={issueURL(number)} target="_blank" rel="noreferrer">
                            #{number}{" "}
                          </a>
                        ))}
                      </p>
                    </>
                  )}
                  <h3>Effective policy</h3>
                  <dl className="queue-policy">
                    <div>
                      <dt>Checks</dt>
                      <dd>
                        {selectedIssue.requirements?.settings.check_profile === "inherit" ||
                        !selectedIssue.requirements
                          ? "Project default"
                          : selectedIssue.requirements.settings.check_profile}
                      </dd>
                    </div>
                    <div>
                      <dt>Delivery</dt>
                      <dd>
                        {selectedIssue.requirements?.settings.delivery === "inherit" ||
                        !selectedIssue.requirements
                          ? "Project default"
                          : selectedIssue.requirements.settings.delivery}
                      </dd>
                    </div>
                  </dl>
                  <p className="queue-hint">Effective values are checked at admission.</p>
                  <p className="queue-hint">
                    Changes to an active issue apply to a future attempt. Use Work for live
                    direction.
                  </p>
                </>
              ) : (
                <p>
                  Sync issues to inspect the source. This queue reference remains in the manifest.
                </p>
              )}
              <div className="queue-inspector-actions">
                {selectedIssue && (
                  <Button
                    size="sm"
                    onClick={() => openEditor(projectID, projectName, selectedIssue)}
                  >
                    Edit task
                  </Button>
                )}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="sm" variant="outline">
                      More actions
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    <DropdownMenuItem asChild>
                      <a href={issueURL(selected)} target="_blank" rel="noreferrer">
                        Open on GitHub
                      </a>
                    </DropdownMenuItem>
                    {selectedIssue?.state === "open" && (
                      <DropdownMenuItem
                        onSelect={() => openEditor(projectID, projectName, selectedIssue, true)}
                      >
                        {selectedIssue.requirements?.settings.hold_reason ? "Edit hold" : "Hold…"}
                      </DropdownMenuItem>
                    )}
                    {selectedPosition >= 0 ? (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          disabled={
                            editingBlocked ||
                            selectedPosition === 0 ||
                            queue.data?.state !== "ready"
                          }
                          onSelect={() => submit("move", selected, order[selectedPosition - 1])}
                        >
                          Move earlier
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={
                            editingBlocked ||
                            selectedPosition === order.length - 1 ||
                            queue.data?.state !== "ready"
                          }
                          onSelect={() => submit("move", selected, order[selectedPosition + 2])}
                        >
                          Move later
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={editingBlocked || queue.data?.state !== "ready"}
                          onSelect={() => submit("remove", selected)}
                          variant="destructive"
                        >
                          Remove from queue
                        </DropdownMenuItem>
                      </>
                    ) : (
                      selectedIssue?.state === "open" && (
                        <DropdownMenuItem
                          disabled={editingBlocked || queue.data?.state !== "ready"}
                          onSelect={() => submit("append", selected)}
                        >
                          Add to end of queue
                        </DropdownMenuItem>
                      )
                    )}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem asChild>
                      <Link to="/history" search={{ project: projectID }}>
                        History
                      </Link>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </>
          )}
        </aside>
      </div>
      <div className="start-panel">
        <div className="start-panel-summary">
          <div>
            <span className="eyebrow">Start work</span>
            <h2>
              {next
                ? `Queue candidate: #${next} ${byNumber.get(next)?.title ?? ""}`
                : "No local queue candidate"}
            </h2>
            <p>
              {next
                ? `Queued position ${order.indexOf(next) + 1}. ${order.indexOf(next) > 0 ? "Earlier tasks are blocked or closed." : "First in queue."}`
                : "Queue issues are blocked, closed, or source data is unavailable."}
            </p>
            {currentRun && (
              <p role="status">
                The execution slot is occupied by{" "}
                {currentRun.project_id === projectID ? "this repository" : "another repository"} (
                {currentRun.state}). Open Work to inspect the active run.
              </p>
            )}
          </div>
        </div>
        <div className="start-options">
          <p className="start-context">
            <strong>{projectName}</strong> · Branch checked at admission
          </p>
          <div className="start-field start-field-wide">
            <Label id="start-scope-label">Scope</Label>
            <RadioGroup
              value={scope}
              aria-labelledby="start-scope-label"
              onValueChange={(value) => {
                setScope(value as "follow" | "fixed");
                preview.reset();
              }}
              className="start-scope"
            >
              <div>
                <RadioGroupItem value="follow" id="start-follow" />
                <Label htmlFor="start-follow">Follow queue</Label>
              </div>
              <div>
                <RadioGroupItem value="fixed" id="start-fixed" />
                <Label htmlFor="start-fixed">Fixed scope</Label>
              </div>
            </RadioGroup>
            <p className="start-help">
              {scope === "follow"
                ? "Tasks deliberately added to this queue may run after the current task, within the remaining budget."
                : `Only issue #${fixedIssue ?? "—"} would be in scope.`}
            </p>
          </div>
          <div className="start-provider-grid">
            <UnavailableSetting id="start-model" label="Model" />
            <UnavailableSetting id="start-reasoning" label="Reasoning" />
            <UnavailableSetting id="start-permissions" label="Permissions" />
            <UnavailableSetting id="start-delivery" label="Delivery" />
          </div>
          <p className="start-help">
            Outpost will show supported choices from the installed Codex provider when the adapter
            is connected. No model or permission is assumed.
          </p>
          <div className="start-budget-grid">
            <div className="start-field">
              <Label htmlFor="start-task-limit">Task limit</Label>
              <Input
                id="start-task-limit"
                type="number"
                min={1}
                max={100}
                value={taskLimit}
                onChange={(event) => setTaskLimit(Number(event.target.value))}
              />
            </div>
            <div className="start-field">
              <Label htmlFor="start-runtime-budget">Runtime budget (minutes, optional)</Label>
              <Input
                id="start-runtime-budget"
                type="number"
                min={1}
                value={runtimeBudget}
                onChange={(event) => setRuntimeBudget(event.target.value)}
              />
            </div>
          </div>
          <div className="start-field">
            <Label htmlFor="start-instructions">Instructions</Label>
            <Textarea
              id="start-instructions"
              value={instructions}
              onChange={(event) => setInstructions(event.target.value)}
              placeholder="Optional guidance for this run"
              rows={3}
            />
          </div>
          <div className="start-footer">
            <div>
              <Button
                size="sm"
                variant="outline"
                disabled={preview.isPending || (scope === "fixed" && !fixedIssue)}
                onClick={() =>
                  preview.mutate({
                    mode: scope,
                    selected: scope === "fixed" ? (fixedIssue ?? 0) : 0,
                  })
                }
              >
                {preview.isPending ? "Checking…" : "Check eligibility"}
              </Button>
              <p role="status">Start is unavailable until the Codex adapter is connected.</p>
            </div>
            <Button disabled>Start work</Button>
          </div>
          {previewCurrent && preview.data && (
            <div className="start-preview" role="status">
              <p>
                {preview.data.selected
                  ? `Fresh source selects #${preview.data.selected}.`
                  : `Admission: ${preview.data.outcome || "no eligible task"}.`}
              </p>
              {preview.data.waiting.map((item) => (
                <p key={`${item.issue}-${item.code}`}>
                  #{item.issue}: {item.reason}
                </p>
              ))}
            </div>
          )}
          {previewCurrent && preview.isError && (
            <p role="alert">Eligibility check failed. Retry after checking repository access.</p>
          )}
        </div>
      </div>
      <details className="queue-source-details">
        <summary>Source details and recent changes</summary>
        <p>Planning branch SHA: {queue.data?.sha || "unavailable"}</p>
        {operations.data?.operations.map((operation) => (
          <p key={operation.id}>
            {operation.kind} #{operation.intended.issue ?? ""}: {operation.state} {operation.reason}
          </p>
        ))}
      </details>
    </section>
  );
}
