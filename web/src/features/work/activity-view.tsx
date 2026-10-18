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
import { Textarea } from "@/components/ui/textarea";
import {
  type ActivityEvent,
  type ActivityPage,
  activityEvent,
  activityPageQuery,
  artifactPreviewQuery,
  olderActivity,
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

function draftKey(runID: string) {
  return `outpost.work-draft.${runID}`;
}
function savedDraft(runID: string) {
  try {
    return sessionStorage.getItem(draftKey(runID)) ?? "";
  } catch {
    return "";
  }
}

export function ActivityView({ runID }: { runID: string }) {
  const client = useQueryClient();
  const page = useQuery(activityPageQuery(runID));
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
  const [draft, setDraft] = useState(() => savedDraft(runID));
  const feed = useRef<HTMLDivElement>(null);
  const restoreScroll = useRef<{ height: number; top: number } | null>(null);
  const key = useMemo(() => ["runs", runID, "activity"] as const, [runID]);
  const startCursor = page.data?.snapshot.cursor;
  const latestActivity = page.data?.events.at(-1)?.created_at;

  useEffect(() => {
    followingRef.current = following;
  }, [following]);

  useEffect(() => {
    try {
      sessionStorage.setItem(draftKey(runID), draft);
    } catch {
      /* Optional tab storage. */
    }
  }, [runID, draft]);

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
  const snapshot = page.data?.snapshot;
  return (
    <div className="activity-work">
      <div className="activity-head">
        <div>
          <span className="eyebrow">Current run</span>
          <h2>
            {snapshot?.attempt_phase
              ? `Phase: ${snapshot.attempt_phase}`
              : "Waiting for assignment"}
          </h2>
          <p>
            {snapshot?.model
              ? `${snapshot.model} · ${snapshot.reasoning} · ${snapshot.sandbox} · ${snapshot.approval_policy}`
              : "Provider settings appear when the task starts."}
          </p>
          <p>
            Run {runID} · {snapshot?.run_state ?? "loading"}
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
          {following ? "Following live" : `Follow live${newCount ? ` · ${newCount} new` : ""}`}
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
      <div className="activity-draft">
        <Label htmlFor="work-instructions-draft">Instructions draft</Label>
        <Textarea
          id="work-instructions-draft"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          placeholder="Draft direction for this run"
        />
        <p>Draft is saved in this tab. Sending instructions arrives with the Work controls task.</p>
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
