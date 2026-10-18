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
  agentCapabilitiesQuery,
  bootstrapQuery,
  type IssueSource,
  inspectQueueOperation,
  previewAdmission,
  projectQueueQuery,
  queueOperationsQuery,
  startRun,
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

function UnavailableSetting({
  id,
  label,
  value = "Provider not connected",
}: {
  id: string;
  label: string;
  value?: string;
}) {
  return (
    <div className="start-field">
      <Label htmlFor={id}>{label}</Label>
      <Select value="unavailable" disabled>
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="unavailable">{value}</SelectItem>
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
}: {
  projectID: string;
  projectName: string;
  repositoryURL: string;
  issues: IssueSource[];
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
  const detailHeadingRef = useRef<HTMLHeadingElement>(null);
  const selectedRowRef = useRef<HTMLButtonElement>(null);
  const lastSelectedRef = useRef(selected);
  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = Number(saved(`${contextKey}.scroll`)) || 0;
  }, [contextKey]);
  useEffect(() => remember(`${contextKey}.tab`, tab), [contextKey, tab]);
  useEffect(
    () => remember(`${contextKey}.selected`, String(selected ?? "")),
    [contextKey, selected],
  );
  useEffect(() => {
    if (selected !== null && window.matchMedia("(max-width: 900px)").matches) {
      detailHeadingRef.current?.focus();
    }
  }, [selected]);
  const [pendingEdit, setPendingEdit] = useState<Edit | null>(null);
  const [message, setMessage] = useState("");
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
  function closeDetails() {
    setSelected(null);
    window.requestAnimationFrame(() => selectedRowRef.current?.focus());
  }
  return (
    <section className="queue-workspace" aria-label="Tasks">
      <div className="queue-toolbar">
        <div
          role="tablist"
          aria-label="Task list"
          className="queue-tabs"
          onKeyDown={(event) => {
            if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
            event.preventDefault();
            const nextTab = tab === "queued" ? "inbox" : "queued";
            setTab(event.key === "Home" ? "queued" : event.key === "End" ? "inbox" : nextTab);
            const buttons = event.currentTarget.querySelectorAll<HTMLButtonElement>("[role=tab]");
            buttons[
              event.key === "Home" ? 0 : event.key === "End" ? 1 : tab === "queued" ? 1 : 0
            ]?.focus();
          }}
        >
          <button
            type="button"
            role="tab"
            aria-selected={tab === "queued"}
            tabIndex={tab === "queued" ? 0 : -1}
            onClick={() => setTab("queued")}
          >
            Queued <span>{order.length}</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === "inbox"}
            tabIndex={tab === "inbox" ? 0 : -1}
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
      <div className={`queue-grid ${selected !== null ? "show-detail" : ""}`}>
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
                  ref={lastSelectedRef.current === number ? selectedRowRef : undefined}
                  onClick={() => {
                    lastSelectedRef.current = number;
                    setSelected(number);
                  }}
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
            <div className="queue-inspector-body">
              <p className="queue-empty">Select a task to inspect its source and queue actions.</p>
            </div>
          ) : (
            <>
              <div className="queue-inspector-body">
                <div className="queue-inspector-heading">
                  <span className="eyebrow">
                    {selectedPosition >= 0 ? `Queued · position ${selectedPosition + 1}` : "Inbox"}
                  </span>
                  <Button size="sm" variant="ghost" onClick={closeDetails}>
                    Back to tasks
                  </Button>
                </div>
                <h2
                  ref={detailHeadingRef}
                  tabIndex={-1}
                  onKeyDown={(event) => {
                    if (event.key === "Escape") closeDetails();
                  }}
                >
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
                            <a
                              key={number}
                              href={issueURL(number)}
                              target="_blank"
                              rel="noreferrer"
                            >
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
              </div>
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

export function StartPanel({
  projectID,
  projectName,
  issues,
}: {
  projectID: string;
  projectName: string;
  issues: IssueSource[];
}) {
  const contextKey = `outpost.queue.${projectID}`;
  const queue = useQuery(projectQueueQuery(projectID));
  const bootstrap = useQuery(bootstrapQuery);
  const queryClient = useQueryClient();
  const capabilities = useQuery(agentCapabilitiesQuery(projectID));
  const order = queue.data?.issues ?? [];
  const byNumber = new Map(issues.map((issue) => [issue.number, issue]));
  const next =
    queue.data?.state === "ready"
      ? order.find((number) => !blocker(byNumber.get(number), byNumber))
      : undefined;
  const selected = Number(saved(`${contextKey}.selected`));
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
  const [model, setModel] = useState(() => saved(`${contextKey}.model`) ?? "");
  const [reasoning, setReasoning] = useState(() => saved(`${contextKey}.reasoning`) ?? "");
  const [delivery, setDelivery] = useState<"local_changes" | "commit" | "push">(() => {
    const value = saved(`${contextKey}.delivery`);
    return value === "commit" || value === "push" ? value : "local_changes";
  });
  const [review, setReview] = useState<"none" | "required">(() =>
    saved(`${contextKey}.review`) === "required" ? "required" : "none",
  );
  const [closeIssue, setCloseIssue] = useState(
    () => saved(`${contextKey}.closeIssueChoice`) !== "false",
  );
  useEffect(() => remember(`${contextKey}.scope`, scope), [contextKey, scope]);
  useEffect(() => remember(`${contextKey}.taskLimit`, String(taskLimit)), [contextKey, taskLimit]);
  useEffect(
    () => remember(`${contextKey}.runtimeBudget`, runtimeBudget),
    [contextKey, runtimeBudget],
  );
  useEffect(() => remember(`${contextKey}.instructions`, instructions), [contextKey, instructions]);
  useEffect(() => remember(`${contextKey}.model`, model), [contextKey, model]);
  useEffect(() => remember(`${contextKey}.reasoning`, reasoning), [contextKey, reasoning]);
  useEffect(() => remember(`${contextKey}.delivery`, delivery), [contextKey, delivery]);
  useEffect(() => remember(`${contextKey}.review`, review), [contextKey, review]);
  const models = capabilities.data?.models ?? [];
  const selectedModel =
    models.find((item) => item.model === model) ?? models.find((item) => item.isDefault);
  const efforts = selectedModel?.supportedReasoningEfforts ?? [];
  const selectedReasoning =
    efforts.find((item) => item.reasoningEffort === reasoning)?.reasoningEffort ??
    efforts.find((item) => item.reasoningEffort === "medium")?.reasoningEffort ??
    selectedModel?.defaultReasoningEffort;
  const fixedIssue = selected > 0 && order.includes(selected) ? selected : next;
  const preview = useMutation({
    mutationFn: (input: { mode: "follow" | "fixed"; selected: number }) =>
      previewAdmission(projectID, input.mode, input.selected),
  });
  const previewCurrent =
    preview.variables?.mode === scope &&
    preview.variables.selected === (scope === "fixed" ? (fixedIssue ?? 0) : 0);
  const start = useMutation({
    mutationFn: () => {
      const minutes = runtimeBudget.trim() === "" ? 0 : Number(runtimeBudget);
      if (!Number.isInteger(minutes) || minutes < 0 || minutes > 10080) {
        throw new Error("Runtime budget must be 1 to 10080 minutes, or blank.");
      }
      const selectedIssue = scope === "fixed" ? (fixedIssue ?? 0) : 0;
      const signature = JSON.stringify({
        scope,
        selectedIssue,
        model: selectedModel?.model,
        reasoning: selectedReasoning,
        delivery,
        review,
        closeIssue,
        taskLimit,
        minutes,
        instructions,
      });
      const identityKey = `${contextKey}.startIdentity`;
      let identity: { signature: string; requestKey: string; deadline: string } | undefined;
      try {
        const prior = saved(identityKey);
        if (prior) identity = JSON.parse(prior) as typeof identity;
      } catch {
        /* use fresh identity */
      }
      if (!identity || identity.signature !== signature) {
        identity = {
          signature,
          requestKey: crypto.randomUUID(),
          deadline: minutes ? new Date(Date.now() + minutes * 60_000).toISOString() : "",
        };
        remember(identityKey, JSON.stringify(identity));
      }
      return startRun(projectID, {
        mode: scope,
        selected: selectedIssue,
        model: selectedModel?.model ?? "",
        reasoning: selectedReasoning ?? "",
        delivery,
        review,
        close_issue: closeIssue,
        task_limit: taskLimit,
        deadline_at: identity.deadline,
        instructions,
        request_key: identity.requestKey,
      });
    },
    onSuccess: (result) => {
      if (result.state !== "launch_unknown") {
        try {
          sessionStorage.removeItem(`${contextKey}.startIdentity`);
        } catch {
          /* optional storage */
        }
      }
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
  });
  return (
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
          {selectedModel ? (
            <div className="start-field">
              <Label htmlFor="start-model">Model</Label>
              <Select
                value={selectedModel.model}
                onValueChange={(value) => {
                  setModel(value);
                  setReasoning("");
                }}
              >
                <SelectTrigger id="start-model" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {models.map((item) => (
                    <SelectItem key={item.model} value={item.model}>
                      {item.displayName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : (
            <UnavailableSetting id="start-model" label="Model" />
          )}
          {selectedModel && selectedReasoning ? (
            <div className="start-field">
              <Label htmlFor="start-reasoning">Reasoning</Label>
              <Select value={selectedReasoning} onValueChange={setReasoning}>
                <SelectTrigger id="start-reasoning" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {efforts.map((item) => (
                    <SelectItem key={item.reasoningEffort} value={item.reasoningEffort}>
                      {item.reasoningEffort}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : (
            <UnavailableSetting id="start-reasoning" label="Reasoning" />
          )}
          <UnavailableSetting
            id="start-permissions"
            label="Permissions"
            value={capabilities.data ? "Workspace write · approval on request" : undefined}
          />
          <div className="start-field">
            <Label htmlFor="start-delivery">Delivery</Label>
            <Select
              value={delivery}
              onValueChange={(value) => setDelivery(value as typeof delivery)}
            >
              <SelectTrigger id="start-delivery" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="local_changes">Local changes</SelectItem>
                <SelectItem value="commit">Commit</SelectItem>
                <SelectItem value="push">Push upstream</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="start-field">
            <Label htmlFor="start-review">Review</Label>
            <Select value={review} onValueChange={(value) => setReview(value as typeof review)}>
              <SelectTrigger id="start-review" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">Automatic completion</SelectItem>
                <SelectItem value="required">Require review</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <p className="start-help">
          {capabilities.data
            ? `${capabilities.data.version} catalog. Availability is confirmed when a turn starts. The selected delivery policy is verified after checks.`
            : capabilities.isError
              ? `Codex settings are unavailable: ${capabilities.error.message}`
              : "Checking installed Codex models and reasoning options…"}
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
        <label className="start-help">
          <input
            type="checkbox"
            checked={closeIssue}
            onChange={(event) => {
              setCloseIssue(event.target.checked);
              remember(`${contextKey}.closeIssueChoice`, String(event.target.checked));
            }}
          />{" "}
          Close the GitHub issue after the completed result comment
        </label>
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
            <p role="status">
              {start.isError
                ? start.error.message
                : start.data
                  ? `Run ${start.data.run_id}: ${start.data.state === "launch_unknown" ? "worker launch needs inspection" : "reserved"}.`
                  : bootstrap.data?.capabilities.execution
                    ? "Ready to reserve one run."
                    : "Start requires the installed private service."}
            </p>
          </div>
          <Button
            disabled={
              !bootstrap.data?.capabilities.execution ||
              !selectedModel ||
              !selectedReasoning ||
              start.isPending ||
              taskLimit < 1 ||
              taskLimit > 100 ||
              (scope === "fixed" && !fixedIssue)
            }
            onClick={() => start.mutate()}
          >
            {start.isPending ? "Starting…" : "Start work"}
          </Button>
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
  );
}
