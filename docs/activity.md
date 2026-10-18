# Activity and retained output

OP-009 adds SQLite notification cursors and private provider output. The current
assignment, attempt, provider and run tables remain the source of truth; activity
events are a bounded notification history. Schema 8 retains the newest 10,000
notifications across the installation. A missing cursor produces an explicit
`reset` and a fresh snapshot. Notification pruning never removes provider files.

## HTTP contract

- `GET /api/v1/runs/{run}/activity?before={cursor}&limit={1..100}` returns a
  current run/attempt/provider snapshot, an ascending page of events, the next
  older cursor, and whether earlier notifications expired. With no `before`, the
  page contains the newest 100 events.
- `GET /api/v1/activity/stream?run={run}&after={cursor}` sends an informational
  `snapshot`, then durable events after the requested cursor. `Last-Event-ID`
  wins on reconnect. Expired or future cursors receive `reset` with current
  state. Heartbeat comments do not claim task progress. Eight streams may be
  open at once; each reads at most 100 events per pass and has a bounded write
  deadline. Slow/disconnected viewers release their stream without affecting
  the worker.
- Event names are `run.changed`, `task.changed`, `attempt.changed`,
  `phase.changed`, `activity.appended`, `check.changed`, `message.changed`,
  `delivery.changed`, and `source.changed`. State transitions produce their
  small notification in the same SQLite transaction. Provider item summaries
  are labeled as provider reports. No event alone proves checks or delivery.
- `GET /api/v1/artifacts/{id}` returns a UTF-8 preview, byte offset, retained
  size, capture/preview truncation flags and download URL. `offset` and `limit`
  provide bounded reading. `?download=1` serves retained bytes as an attachment.
  Artifact IDs resolve through the SQLite manifest and a confined private state
  root; no browser-provided path is opened. Missing retained files return 410.
- `GET /api/v1/activity/storage` reports recorded retained bytes, artifact
  count, truncated count and the capture limit. `DELETE /api/v1/artifacts/{id}`
  records removal intent before deleting a file and accepts only terminal
  assignments in released runs. An uncertain deletion remains inspectable and
  can be reconciled by retrying the same ID. Removed output returns 410.

The provider's raw newline-delimited events are capped at 8 MiB per assignment
in schema 8 and stored under `artifacts/{run ID}/{attempt ID}/` outside the
checkout. Once the cap is reached, additional raw events are suppressed and
the manifest reports `capture_truncated`; control/result processing continues.
The Work page previews at most 12 KiB and offers the retained download. There
is no automatic artifact deletion. Active and unresolved assignments retain
their files. The removal API is available for explicit settled retention work;
the dashboard has no cleanup control yet.

The same private Tailscale identity gate covers HTML, API, SSE and artifacts.
Provider text is rendered as inert text in React; raw output is an attachment,
not active HTML. The Work draft is stored in tab-scoped session storage by run
ID, survives navigation, and is not sent until live controls exist.
