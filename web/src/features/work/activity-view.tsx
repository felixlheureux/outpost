import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  type ActivityEvent,
  type ActivityPage,
  activityEvent,
  activityPageQuery,
  artifactPreviewQuery,
  checkOutputQuery,
  olderActivity,
  sendWorkControl,
  sendWorkMessage,
  type WorkOverview,
  workChangesQuery,
  workDiffQuery,
  workOverviewQuery,
} from "@/lib/api";

function display(event: ActivityEvent): string {
  const payload = event.payload;
  if (typeof payload.summary === "string" && payload.summary) return payload.summary;
  if (event.kind === "phase.changed") return `Phase: ${payload.phase ?? "unknown"}`;
  if (event.kind === "task.changed")
    return `Task #${payload.issue_number ?? "?"}: ${payload.state ?? "changed"}`;
  if (event.kind === "run.changed") return `Run: ${payload.state ?? "changed"}`;
  if (event.kind === "attempt.changed") return `Attempt: ${payload.phase ?? "preparing"}`;
  if (event.kind === "activity.appended")
    return `Provider: ${payload.state ?? payload.category ?? "update"}`;
  return event.kind.replace(".", " ");
}

function draftKey(projectID: string, attemptID: string) {
  return `outpost.work-draft.${projectID}.${attemptID}`;
}
function savedDraft(key: string) {
  try {
    return sessionStorage.getItem(key) ?? "";
  } catch {
    return "";
  }
}
function elapsed(startedAt: string | undefined, now: number) {
  if (!startedAt) return "";
  const started = Date.parse(startedAt);
  if (!Number.isFinite(started)) return "";
  const minutes = Math.max(0, Math.floor((now - started) / 60000));
  return minutes >= 60 ? `${Math.floor(minutes / 60)}h ${minutes % 60}m` : `${minutes}m`;
}

export function ActivityView({ runID }: { runID: string }) {
  const client = useQueryClient();
  const page = useQuery(activityPageQuery(runID));
  const work = useQuery(workOverviewQuery(runID));
  const [connection, setConnection] = useState<"connecting" | "connected" | "reconnecting">(
    "connecting",
  );
  const [lastSeen, setLastSeen] = useState("");
  const [older, setOlder] = useState<ActivityEvent[]>([]);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [filter, setFilter] = useState<"all" | "message" | "command" | "state">("all");
  const [following, setFollowing] = useState(true);
  const followingRef = useRef(true);
  const [newCount, setNewCount] = useState(0);
  const [opened, setOpened] = useState<ActivityEvent | null>(null);
  const [draft, setDraft] = useState("");
  const [draftScope, setDraftScope] = useState("");
  const [pendingID, setPendingID] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const [continuationMode, setContinuationMode] = useState<"resume" | "fresh">("resume");
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, []);
  const feed = useRef<HTMLDivElement>(null);
  const restoreScroll = useRef<{ height: number; top: number } | null>(null);
  const key = useMemo(() => ["runs", runID, "activity"] as const, [runID]);
  const startCursor = page.data?.snapshot.cursor;
  const latestActivity = page.data?.events.at(-1)?.created_at;

  useEffect(() => {
    followingRef.current = following;
  }, [following]);

  const currentScope = work.data?.attempt_id
    ? draftKey(work.data.project_id, work.data.attempt_id)
    : "";
  useEffect(() => {
    if (!currentScope || currentScope === draftScope) return;
    setDraft(savedDraft(currentScope));
    setPendingID(savedDraft(`${currentScope}.pending`));
    setDraftScope(currentScope);
  }, [currentScope, draftScope]);
  useEffect(() => {
    if (!draftScope) return;
    try {
      sessionStorage.setItem(draftScope, draft);
    } catch {
      /* Optional tab storage. */
    }
  }, [draftScope, draft]);
  useEffect(() => {
    if (!pendingID || !work.data || !draftScope) return;
    const receipt = work.data.messages.find((item) => item.id === pendingID);
    if (receipt?.state === "accepted") {
      if (draft === receipt.body) setDraft("");
      setPendingID("");
      try {
        sessionStorage.removeItem(`${draftScope}.pending`);
      } catch {
        /* Optional tab storage. */
      }
    }
  }, [pendingID, work.data, draftScope, draft]);

  useEffect(() => {
    if (startCursor === undefined) return;
    const stream = new EventSource(
      `/api/v1/activity/stream?run=${encodeURIComponent(runID)}&after=${startCursor}`,
    );
    stream.onopen = () => {
      setConnection("connected");
      setLastSeen(new Date().toISOString());
    };
    stream.onerror = () => setConnection("reconnecting");
    stream.addEventListener("snapshot", () => {
      setLastSeen(new Date().toISOString());
      void client.invalidateQueries({ queryKey: ["bootstrap"] });
      void client.invalidateQueries({ queryKey: ["runs", runID, "work"] });
    });
    stream.addEventListener("reset", () => {
      setOlder([]);
      setNewCount(0);
      void client.invalidateQueries({ queryKey: key });
      void client.invalidateQueries({ queryKey: ["bootstrap"] });
    });
    const receive = (message: MessageEvent) => {
      let body: unknown;
      try {
        body = JSON.parse(message.data);
      } catch {
        return;
      }
      const parsed = activityEvent.safeParse(body);
      if (!parsed.success) return;
      setLastSeen(new Date().toISOString());
      client.setQueryData<ActivityPage>(key, (current) => {
        if (!current || current.events.some((item) => item.cursor === parsed.data.cursor))
          return current;
        return { ...current, events: [...current.events, parsed.data].slice(-100) };
      });
      if (!followingRef.current) setNewCount((count) => count + 1);
      if (parsed.data.kind !== "activity.appended") {
        void client.invalidateQueries({ queryKey: key });
        void client.invalidateQueries({ queryKey: ["bootstrap"] });
        void client.invalidateQueries({ queryKey: ["runs", runID, "work"] });
      }
    };
    const kinds = [
      "run.changed",
      "task.changed",
      "attempt.changed",
      "phase.changed",
      "activity.appended",
      "check.changed",
      "message.changed",
      "delivery.changed",
      "source.changed",
    ];
    kinds.forEach((kind) => {
      stream.addEventListener(kind, receive);
    });
    return () => {
      kinds.forEach((kind) => {
        stream.removeEventListener(kind, receive);
      });
      stream.close();
    };
  }, [client, runID, startCursor, key]);

  useEffect(() => {
    if (following && feed.current && !restoreScroll.current)
      feed.current.scrollTop = feed.current.scrollHeight;
  });

  useLayoutEffect(() => {
    if (restoreScroll.current && feed.current) {
      feed.current.scrollTop =
        restoreScroll.current.top + feed.current.scrollHeight - restoreScroll.current.height;
      restoreScroll.current = null;
    }
  });

  async function loadOlder() {
    const first = older[0]?.cursor ?? page.data?.events[0]?.cursor;
    if (!first || loadingOlder) return;
    setFollowing(false);
    setLoadingOlder(true);
    try {
      const previous = await olderActivity(runID, first);
      if (feed.current)
        restoreScroll.current = { height: feed.current.scrollHeight, top: feed.current.scrollTop };
      setOlder((items) => [
        ...previous.events.filter((event) => !items.some((item) => item.cursor === event.cursor)),
        ...items,
      ]);
    } finally {
      setLoadingOlder(false);
    }
  }
  const events = [...older, ...(page.data?.events ?? [])].filter((event) => {
    if (filter === "all") return true;
    if (filter === "state") return event.kind !== "activity.appended";
    return event.kind === "activity.appended" && event.payload.category === filter;
  });
  const state = work.data;
  const canSendLive = state?.provider_state === "turn_started" && state.turn_open;
  const canQueueNext = state?.provider_state === "thread_started";
  const canContinue =
    state?.attempt_outcome === "running" &&
    ["interrupted", "failed", "unknown"].includes(state.provider_state);
  const canResume = Boolean(state?.thread_id);
  useEffect(() => {
    if (!canResume) setContinuationMode("fresh");
  }, [canResume]);
  const immediateStop = Boolean(
    state?.controls.some((item) => item.action === "interrupt" && item.state !== "rejected"),
  );
  const pendingReceipt = state?.messages.find((item) => item.id === pendingID);

  async function submitInstructions() {
    if (!state || !draft.trim() || busy || pendingID) return;
    setBusy(true);
    setActionError("");
    try {
      const id = crypto.randomUUID();
      const result = await sendWorkMessage(runID, {
        id,
        attempt_id: state.attempt_id,
        thread_id: state.thread_id,
        expected_turn_id: canSendLive ? state.turn_id : "",
        target: canSendLive ? "live" : "next",
        body: draft,
      });
      setPendingID(result.id);
      try {
        sessionStorage.setItem(`${draftScope}.pending`, result.id);
      } catch {
        /* Optional tab storage. */
      }
      await client.invalidateQueries({ queryKey: ["runs", runID, "work"] });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Could not submit instructions.");
    } finally {
      setBusy(false);
    }
  }

  async function control(action: "finish" | "undo" | "interrupt" | "continue") {
    if (!state || busy) return;
    setBusy(true);
    setActionError("");
    try {
      const result = await sendWorkControl(runID, {
        id: crypto.randomUUID(),
        attempt_id: state.attempt_id,
        action,
        ...(action === "continue" ? { mode: continuationMode, instructions: draft } : {}),
      });
      if (action === "continue" && result.state === "accepted") setDraft("");
      await client.invalidateQueries({ queryKey: ["runs", runID, "work"] });
      await client.invalidateQueries({ queryKey: ["bootstrap"] });
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Could not change the run.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="activity-work">
      <div className="activity-head">
        <div>
          <span className="eyebrow">
            Current assignment · {state?.attempt_phase || "preparing"}
          </span>
          <h2>
            {state?.snapshot?.title
              ? `#${state.issue_number} ${state.snapshot.title}`
              : "Waiting for assignment"}
          </h2>
          <p>
            {state?.model
              ? `${state.model} · ${state.reasoning} · ${state.sandbox} · ${state.approval_policy}`
              : "Provider settings appear when the task starts."}
          </p>
          <p>
            {state
              ? `${state.successful_count} of ${state.task_limit} tasks completed${elapsed(state.assignment_started_at, now) ? ` · ${elapsed(state.assignment_started_at, now)} elapsed` : ""} · ${state.stop_requested ? "Stops after this task" : "Queue can continue"}`
              : `Run ${runID} · loading`}
          </p>
        </div>
        <div className="activity-connection" role="status">
          <strong>
            {connection === "connected"
              ? "Connected"
              : connection === "reconnecting"
                ? "Reconnecting"
                : "Connecting"}
          </strong>
          <span>
            {lastSeen
              ? `Last stream contact ${new Date(lastSeen).toLocaleTimeString()}`
              : "Awaiting live connection"}
          </span>
          <span>
            {latestActivity
              ? `Last activity ${new Date(latestActivity).toLocaleTimeString()}`
              : "No recorded activity yet"}
          </span>
          <span>Connection status does not determine worker state.</span>
        </div>
      </div>
      {page.isError ? <p role="alert">Activity is unavailable: {page.error.message}</p> : null}
      {work.isError ? (
        <p role="alert">Work controls are unavailable: {work.error.message}</p>
      ) : null}
      <div className="work-control-row">
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={state?.attempt_outcome !== "running" || busy || state.stop_requested}
          onClick={() => void control("finish")}
        >
          Finish task &amp; stop
        </Button>
        {state?.stop_requested && !immediateStop ? (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy || state.schedule_outcome !== "active"}
            onClick={() => void control("undo")}
          >
            Undo stop
          </Button>
        ) : null}
        <Button
          type="button"
          size="sm"
          variant="destructive"
          disabled={!canSendLive || busy}
          onClick={() => void control("interrupt")}
        >
          Interrupt now
        </Button>
      </div>
      {state?.stop_requested ? (
        <p className="work-note" role="status">
          {immediateStop
            ? "Interruption requested. The next task admission remains blocked; continue this assignment explicitly after the provider settles."
            : "The next task admission is blocked. This task can finish or be interrupted."}
        </p>
      ) : null}
      <div className="work-layout">
        {state?.snapshot ? (
          <details className="work-brief">
            <summary>Task brief</summary>
            <div>{state.snapshot.body}</div>
            {state.snapshot.issue_url ? (
              <a href={state.snapshot.issue_url} target="_blank" rel="noreferrer">
                Open issue on GitHub
              </a>
            ) : null}
            {state.attempts && state.attempts.length > 1 ? (
              <section className="work-attempts" aria-label="Attempt history">
                <strong>Attempt history</strong>
                {state.attempts.map((attempt) => (
                  <p key={attempt.id}>
                    #{attempt.ordinal} · {attempt.phase} · {attempt.outcome}
                  </p>
                ))}
              </section>
            ) : null}
          </details>
        ) : null}
        <div className="work-primary">
          <Tabs defaultValue="activity">
            <TabsList aria-label="Work details">
              <TabsTrigger value="activity">Activity</TabsTrigger>
              <TabsTrigger value="changes">Changes</TabsTrigger>
              <TabsTrigger value="checks">Checks</TabsTrigger>
            </TabsList>
            <TabsContent value="activity">
              <div className="activity-toolbar">
                <fieldset aria-label="Activity filter">
                  {(["all", "message", "command", "state"] as const).map((item) => (
                    <Button
                      key={item}
                      type="button"
                      size="sm"
                      variant={filter === item ? "secondary" : "ghost"}
                      onClick={() => setFilter(item)}
                    >
                      {item === "all"
                        ? "All"
                        : item === "state"
                          ? "State"
                          : `${item[0].toUpperCase()}${item.slice(1)}s`}
                    </Button>
                  ))}
                </fieldset>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setFollowing(true);
                    setNewCount(0);
                  }}
                >
                  {following
                    ? "Following live"
                    : `Follow live${newCount ? ` · ${newCount} new` : ""}`}
                </Button>
              </div>
              <div
                className="activity-feed"
                ref={feed}
                onScroll={(event) => {
                  const target = event.currentTarget;
                  if (target.scrollHeight - target.scrollTop - target.clientHeight > 35)
                    setFollowing(false);
                }}
              >
                {page.isLoading ? <p>Loading activity…</p> : null}
                {page.data?.next_before &&
                (older[0]?.cursor ?? page.data.events[0]?.cursor ?? 0) >
                  page.data.snapshot.oldest_cursor ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    disabled={loadingOlder}
                    onClick={() => void loadOlder()}
                  >
                    {loadingOlder ? "Loading…" : "Load older activity"}
                  </Button>
                ) : null}
                {page.data?.history_truncated && !page.data?.next_before ? (
                  <p>Earlier notifications expired; retained output may still be available.</p>
                ) : null}
                {events.length === 0 && !page.isLoading ? (
                  <p>No activity has been recorded for this run yet.</p>
                ) : null}
                {events.map((event) => (
                  <button
                    className="activity-entry"
                    type="button"
                    key={event.cursor}
                    onClick={() => setOpened(event)}
                  >
                    <span className="activity-entry-meta">
                      {event.kind} · {new Date(event.created_at).toLocaleTimeString()}
                    </span>
                    <span>{display(event)}</span>
                    {event.artifact_id ? (
                      <span className="activity-entry-meta">Retained output available</span>
                    ) : null}
                  </button>
                ))}
              </div>
            </TabsContent>
            <TabsContent value="changes">
              <ChangesPanel runID={runID} />
            </TabsContent>
            <TabsContent value="checks">
              <ChecksPanel runID={runID} checks={work.data?.checks ?? []} />
            </TabsContent>
          </Tabs>
          {canContinue ? (
            <div className="work-recovery" role="status">
              <strong>
                {state?.provider_state === "interrupted"
                  ? "Provider interrupted"
                  : state?.provider_state === "unknown"
                    ? "Provider outcome unknown"
                    : "Provider stopped"}
              </strong>
              <p>
                {state?.provider_failure_code ? `${state.provider_failure_code}. ` : ""}The pinned
                assignment, checkout changes, and earlier receipts remain available.
                {canResume
                  ? " The saved session can be resumed."
                  : " No saved session was recorded; choose a fresh conversation."}
              </p>
            </div>
          ) : null}
          <div className="activity-draft">
            <Label htmlFor="work-instructions-draft">
              {canContinue
                ? "Continue with instructions"
                : canQueueNext
                  ? "Instructions for next turn"
                  : "Instructions for current turn"}
            </Label>
            <Textarea
              id="work-instructions-draft"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Add direction for this task"
            />
            {canContinue ? (
              <fieldset className="work-control-row" aria-label="Continuation mode">
                <Button
                  type="button"
                  size="sm"
                  variant={continuationMode === "resume" ? "secondary" : "outline"}
                  disabled={!canResume}
                  onClick={() => setContinuationMode("resume")}
                >
                  Resume saved session
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant={continuationMode === "fresh" ? "secondary" : "outline"}
                  onClick={() => setContinuationMode("fresh")}
                >
                  Fresh conversation
                </Button>
              </fieldset>
            ) : null}
            <div className="work-composer-footer">
              <span>
                {canContinue
                  ? "Continues this assignment. Queue processing remains stopped until checks and delivery are available."
                  : canSendLive
                    ? `Targets turn ${state?.turn_id}`
                    : canQueueNext
                      ? "Targets the next turn in this attempt."
                      : "Wait for an active turn or choose a continuation."}
              </span>
              <Button
                type="button"
                size="sm"
                disabled={
                  busy ||
                  !state ||
                  (!canContinue && !canSendLive && !canQueueNext) ||
                  (!canContinue && !draft.trim()) ||
                  Boolean(pendingID) ||
                  (canContinue && continuationMode === "resume" && !canResume)
                }
                onClick={() => void (canContinue ? control("continue") : submitInstructions())}
              >
                {busy
                  ? "Submitting…"
                  : canContinue
                    ? "Continue with instructions"
                    : canQueueNext
                      ? "Queue for next turn"
                      : "Send instructions"}
              </Button>
            </div>
            {actionError ? <p role="alert">{actionError}</p> : null}
            {pendingReceipt && ["unknown", "rejected"].includes(pendingReceipt.state) ? (
              <div role="alert" className="work-control-row">
                <span>
                  {pendingReceipt.state === "unknown"
                    ? "Delivery acknowledgement is unknown. Inspect the receipt before choosing a new message."
                    : "The provider rejected this message. Your draft is preserved."}
                </span>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setPendingID("");
                    try {
                      sessionStorage.removeItem(`${draftScope}.pending`);
                    } catch {
                      /* Optional tab storage. */
                    }
                  }}
                >
                  Prepare a new message
                </Button>
              </div>
            ) : null}
            {state?.messages.length ? (
              <section className="work-receipts" aria-label="Instruction receipts">
                {state.messages.slice(0, 8).map((message) => (
                  <div key={message.id}>
                    <strong>{message.state}</strong>
                    <span>{message.body}</span>
                    <small>
                      {message.reason ||
                        (message.target === "live"
                          ? `Turn ${message.expected_turn_id}`
                          : "Next turn")}
                      {message.state === "accepted" && message.contract_changed
                        ? " · Validation contract changed"
                        : ""}
                    </small>
                  </div>
                ))}
              </section>
            ) : null}
            {state?.controls.length ? (
              <section className="work-control-receipts" aria-label="Control receipts">
                {state.controls.slice(0, 4).map((item) => (
                  <span key={item.id}>
                    {item.action}: {item.state}
                    {item.reason ? ` · ${item.reason}` : ""}
                  </span>
                ))}
              </section>
            ) : null}
          </div>
        </div>
      </div>
      <ActivityReader
        event={opened}
        onOpenChange={(open) => {
          if (!open) setOpened(null);
        }}
      />
    </div>
  );
}

function ChangesPanel({ runID }: { runID: string }) {
  const changes = useQuery(workChangesQuery(runID));
  const [path, setPath] = useState("");
  const diff = useQuery({ ...workDiffQuery(runID, path), enabled: Boolean(path) });
  return (
    <div className="work-changes">
      {changes.isLoading ? <p>Loading checkout changes…</p> : null}
      {changes.isError ? (
        <p role="alert">Changes could not be read: {changes.error.message}</p>
      ) : null}
      {changes.data?.paths.length === 0 ? <p>No changed paths in the owned checkout.</p> : null}
      <div className="work-paths">
        {changes.data?.paths.map((item) => (
          <Button
            key={item.path}
            type="button"
            size="sm"
            variant={path === item.path ? "secondary" : "ghost"}
            onClick={() => setPath(item.path)}
          >
            <span>{item.status}</span>
            {item.path}
          </Button>
        ))}
      </div>
      {changes.data?.truncated ? <p>Showing the first 200 paths.</p> : null}
      {path ? (
        <div className="work-diff">
          <strong>{path}</strong>
          {diff.isLoading ? <p>Loading diff…</p> : null}
          {diff.isError ? <p role="alert">Diff could not be read: {diff.error.message}</p> : null}
          {diff.data ? (
            <>
              <pre>{diff.data.diff || "No text diff for this path."}</pre>
              {diff.data.truncated ? <p>Preview capped at 64 KiB.</p> : null}
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function ActivityReader({
  event,
  onOpenChange,
}: {
  event: ActivityEvent | null;
  onOpenChange: (open: boolean) => void;
}) {
  const artifact = useQuery({
    ...artifactPreviewQuery(event?.artifact_id ?? ""),
    enabled: Boolean(event?.artifact_id),
  });
  return (
    <Dialog open={Boolean(event)} onOpenChange={onOpenChange}>
      <DialogContent className="activity-reader">
        <DialogHeader>
          <DialogTitle>{event?.kind ?? "Activity"}</DialogTitle>
          <DialogDescription>
            {event ? new Date(event.created_at).toLocaleString() : ""}
          </DialogDescription>
        </DialogHeader>
        <pre>{event ? display(event) : ""}</pre>
        {event?.artifact_id ? (
          <div>
            {artifact.isLoading ? <p>Loading retained output…</p> : null}
            {artifact.isError ? (
              <p>Retained output is unavailable: {artifact.error.message}</p>
            ) : null}
            {artifact.data ? (
              <>
                <pre>{artifact.data.text}</pre>
                <p>
                  {artifact.data.preview_truncated ? "Preview truncated. " : ""}
                  {artifact.data.capture_truncated ? "Capture reached its limit. " : ""}
                  {artifact.data.retained_bytes} retained bytes.
                </p>
                <a href={artifact.data.download_url}>Download retained output</a>
              </>
            ) : null}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function ChecksPanel({ runID, checks }: { runID: string; checks: WorkOverview["checks"] }) {
  const [opened, setOpened] = useState("");
  const output = useQuery({ ...checkOutputQuery(runID, opened), enabled: Boolean(opened) });
  if (checks.length === 0) {
    return (
      <div className="work-empty-panel">
        <h3>Checks</h3>
        <p>No runner check receipts yet. Agent claims in Activity do not count as check results.</p>
      </div>
    );
  }
  return (
    <section className="work-checks" aria-label="Runner check receipts">
      {checks.map((check) => (
        <div className="work-state" key={check.id}>
          <div className="work-check-header">
            <strong>{check.name || check.check_id}</strong>
            <span>
              {check.required ? "Required" : "Optional"} · {check.status}
            </span>
          </div>
          <p>
            {check.reason ||
              (check.status === "reused"
                ? "Earlier terminal evidence still matches these inputs."
                : "Runner-owned result")}
          </p>
          {check.input_digest && (
            <p className="activity-entry-meta">
              Input {check.input_digest.slice(0, 12)} · contract{" "}
              {check.contract_digest.slice(0, 12)}{" "}
              {check.candidate_digest ? `· candidate ${check.candidate_digest.slice(0, 12)} ` : ""}·{" "}
              {check.duration_ms} ms{check.exit_code !== null ? ` · exit ${check.exit_code}` : ""}
              {check.case_count !== null ? ` · ${check.case_count} cases` : ""}
            </p>
          )}
          {check.status === "running" && (
            <p>
              PID {check.pid} owns this workspace. On the host, run{" "}
              <code>outpost stop --run-id {runID}</code> to interrupt the recorded worker and its
              check.
            </p>
          )}
          {check.truncated && <p>Retained output reached its 1 MiB limit.</p>}
          {check.status !== "not-run" && (
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpened(opened === check.id ? "" : check.id)}
            >
              {opened === check.id ? "Hide output" : "View retained output"}
            </Button>
          )}
          {opened === check.id && (
            <div role="status">
              {output.isLoading ? (
                "Loading output…"
              ) : output.isError ? (
                "Retained output is unavailable."
              ) : (
                <pre className="work-check-output">
                  {output.data?.output || "No output captured."}
                </pre>
              )}
            </div>
          )}
        </div>
      ))}
    </section>
  );
}
