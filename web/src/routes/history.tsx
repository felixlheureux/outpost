import { useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useRouterState } from "@tanstack/react-router";
import { History } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  activityPageQuery,
  bootstrapQuery,
  diagnosticsQuery,
  historicalMessageQuery,
  historyArtifactsQuery,
  historyDetailQuery,
  historyQuery,
  projectsQuery,
  removeArtifact,
  retryReports,
} from "@/lib/api";

export const Route = createFileRoute("/history")({ component: HistoryPage });

function HistoryPage() {
  const client = useQueryClient();
  const { data: projects } = useSuspenseQuery(projectsQuery);
  const selectedProjectID = useRouterState({ select: (state) => state.location.search.project });
  const project =
    projects.projects.find((item) => item.id === selectedProjectID) ?? projects.projects[0];
  const [offset, setOffset] = useState(0);
  const [selectedRun, setSelectedRun] = useState("");
  const [showDiagnostics, setShowDiagnostics] = useState(false);
  const [retentionError, setRetentionError] = useState("");
  const [reportStatus, setReportStatus] = useState("");
  useEffect(() => {
    if (!project?.id) return;
    setOffset(0);
    setSelectedRun("");
  }, [project?.id]);
  const list = useQuery(historyQuery(project?.id ?? "", offset));
  const visibleRunID = list.data?.runs.some((run) => run.id === selectedRun) ? selectedRun : "";
  const detail = useQuery(historyDetailQuery(visibleRunID));
  const activity = useQuery({ ...activityPageQuery(visibleRunID), enabled: Boolean(visibleRunID) });
  const artifacts = useQuery(historyArtifactsQuery(visibleRunID));
  const { data: bootstrap } = useSuspenseQuery(bootstrapQuery);
  const diagnostics = useQuery({
    ...diagnosticsQuery(project?.id ?? ""),
    enabled: showDiagnostics,
  });
  return (
    <section className="page history-page">
      <div className="page-title">
        <div>
          <p className="eyebrow">{project?.name ?? "Results"}</p>
          <h1>History</h1>
          <p>Runs, linked attempts, and retained evidence from this repository.</p>
        </div>
      </div>
      <div className="work-state">
        <Button size="sm" variant="outline" onClick={() => setShowDiagnostics(!showDiagnostics)}>
          {showDiagnostics ? "Hide connection" : "Connection and storage"}
        </Button>
        {showDiagnostics ? (
          <div role="status">
            {diagnostics.isLoading ? <p>Observing host services…</p> : null}
            {diagnostics.isError ? (
              <p>Diagnostics unavailable: {diagnostics.error.message}</p>
            ) : null}
            {diagnostics.data ? (
              <>
                <p>
                  {diagnostics.data.mode} · Outpost {diagnostics.data.version} · observed{" "}
                  {new Date(diagnostics.data.observed_at).toLocaleString()}
                </p>
                <p>
                  Private address: {diagnostics.data.private_url || "Not configured"} · allowed
                  identity: {diagnostics.data.allowed_identity}
                </p>
                <p>
                  Tailscale: {diagnostics.data.tailscale} · Serve: {diagnostics.data.serve_route} ·
                  web service: {diagnostics.data.web_service}
                </p>
                <p>
                  GitHub: {diagnostics.data.github} · Codex: {diagnostics.data.provider}
                  {diagnostics.data.provider_version
                    ? ` (${diagnostics.data.provider_version})`
                    : ""}
                </p>
                {diagnostics.data.artifacts ? (
                  <p>
                    Retained output: {diagnostics.data.artifacts.count} artifacts,{" "}
                    {diagnostics.data.artifacts.recorded_bytes} recorded bytes,{" "}
                    {diagnostics.data.artifacts.truncated_count} truncated · private state{" "}
                    {diagnostics.data.state_access ? "accessible" : "unavailable"}
                  </p>
                ) : null}
                {diagnostics.data.disk_available_bytes !== undefined ? (
                  <p>
                    Available state volume:{" "}
                    {Math.floor(diagnostics.data.disk_available_bytes / (1024 * 1024))} MiB
                  </p>
                ) : null}
                {diagnostics.data.project_setup ? (
                  <p>
                    Repository setup: {diagnostics.data.project_setup} · issues:{" "}
                    {diagnostics.data.issue_source || "not observed"} · queue:{" "}
                    {diagnostics.data.queue_source || "not observed"}
                    {diagnostics.data.source_observed_at
                      ? ` · last source observation ${new Date(diagnostics.data.source_observed_at).toLocaleString()}`
                      : ""}{" "}
                    ·{" "}
                    <Link to="/" search={{ project: project?.id }}>
                      Open Queue to sync
                    </Link>
                  </p>
                ) : null}
              </>
            ) : null}
          </div>
        ) : null}
      </div>
      {!project ? (
        <div className="empty-state bordered-empty">
          <History size={26} />
          <h2>No attempts yet</h2>
          <p>Add a repository to begin work; its runs will appear here.</p>
        </div>
      ) : null}
      {list.isError ? <p role="alert">History is unavailable: {list.error.message}</p> : null}
      {list.data?.runs.length === 0 && offset === 0 ? (
        <div className="empty-state bordered-empty">
          <History size={26} />
          <h2>No attempts yet</h2>
          <p>Runs in this repository will appear here, including launch failures.</p>
        </div>
      ) : null}
      {list.data?.runs.map((run) => (
        <div className="work-state history-run" key={run.id}>
          <div className="work-control-row">
            <Button
              variant={selectedRun === run.id ? "secondary" : "outline"}
              size="sm"
              onClick={() => setSelectedRun(selectedRun === run.id ? "" : run.id)}
            >
              {selectedRun === run.id ? "Hide details" : "View details"}
            </Button>
            <strong>{run.schedule_outcome || run.state}</strong>
            <span>{new Date(run.created_at).toLocaleString()}</span>
          </div>
          <p>
            {run.assignment_count} assignment{run.assignment_count === 1 ? "" : "s"} ·{" "}
            {run.successful_count} of {run.task_limit} completed · worker reservation{" "}
            {run.reservation_state}
          </p>
          {run.schedule_reason ? <p>{run.schedule_reason}</p> : null}
          {run.launch_observation === "unknown" ? (
            <p role="status">
              Launch acknowledgement is unknown. Check the recorded worker before retrying.
            </p>
          ) : null}
          {selectedRun === run.id ? (
            <div className="history-detail">
              {detail.isLoading ? <p>Loading retained receipts…</p> : null}
              {detail.isError ? (
                <p role="alert">Details are unavailable: {detail.error.message}</p>
              ) : null}
              {detail.data?.assignments.map((assignment) => (
                <section key={assignment.id} className="work-brief">
                  <h3>
                    #{assignment.issue_number} {assignment.title || "Title not recorded"}
                  </h3>
                  <p>
                    Assignment {assignment.state} · started{" "}
                    {new Date(assignment.created_at).toLocaleString()}
                  </p>
                  <p>
                    Issue revision: {assignment.issue_revision || "Not recorded"} · issue digest:{" "}
                    {assignment.issue_digest || "Not recorded"} · queue revision:{" "}
                    {assignment.queue_sha || "Not recorded"}
                  </p>
                  <p>
                    Branch: {assignment.branch || "Not recorded"} · baseline:{" "}
                    {assignment.baseline_head || "Not recorded"}
                  </p>
                  {assignment.attempts.map((attempt) => (
                    <div key={attempt.id} className="work-receipts">
                      <strong>
                        Attempt {attempt.ordinal} · {attempt.outcome}
                      </strong>
                      <span>
                        {attempt.phase} · provider {attempt.provider_state || "Not recorded"}
                      </span>
                      <small>
                        {attempt.model || "Model not recorded"} ·{" "}
                        {attempt.reasoning || "Reasoning not recorded"} ·{" "}
                        {attempt.sandbox || "Permissions not recorded"}
                        {` · ${attempt.approval_policy || "Approval policy not recorded"}`}
                      </small>
                      {attempt.failure_code ? <small>Failure: {attempt.failure_code}</small> : null}
                      <small>
                        Saved thread: {attempt.thread_id || "Not recorded"} · turn:{" "}
                        {attempt.turn_id || "Not recorded"}
                        {attempt.predecessor_id ? ` · follows ${attempt.predecessor_id}` : ""}
                      </small>
                    </div>
                  ))}
                  {assignment.checks.map((check) => (
                    <p key={check.id}>
                      Check · attempt {check.attempt_id} · {check.name}: {check.status}
                      {check.reason ? ` · ${check.reason}` : ""} ·{" "}
                      <a
                        href={`/api/v1/runs/${encodeURIComponent(run.id)}/checks/${encodeURIComponent(check.id)}/output`}
                      >
                        Output receipt
                      </a>
                    </p>
                  ))}
                  {assignment.delivery ? (
                    <p>
                      Delivery: {assignment.delivery.state} · {assignment.delivery.policy}
                      {assignment.delivery.commit_sha ? ` · ${assignment.delivery.commit_sha}` : ""}
                      {assignment.delivery.reason ? ` · ${assignment.delivery.reason}` : ""}
                    </p>
                  ) : null}
                  {assignment.report ? (
                    <p>
                      GitHub report: {assignment.report.state}
                      {assignment.report.comment_id
                        ? ` · comment ${assignment.report.comment_id}`
                        : ""}
                      {assignment.report.reason ? ` · ${assignment.report.reason}` : ""}
                    </p>
                  ) : null}
                  {assignment.messages.map((message) => (
                    <p key={message.id}>
                      Instruction · attempt {message.attempt_id} · turn {message.turn_id || "next"}{" "}
                      · {message.state}: {message.body}
                      {message.body_truncated ? (
                        <HistoricalMessage runID={run.id} messageID={message.id} />
                      ) : null}
                      {message.reason ? ` · ${message.reason}` : ""}
                    </p>
                  ))}
                </section>
              ))}
              {detail.data?.assignment_count === 0 ? (
                <p>
                  No assignment was admitted. Launch observation: {detail.data.launch_observation}.
                </p>
              ) : null}
              {run.reservation_state === "released" &&
              detail.data?.assignments.some(
                (assignment) =>
                  assignment.report?.state !== "settled" && assignment.state === "succeeded",
              ) ? (
                <div className="work-control-row">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={async () => {
                      setReportStatus("Checking GitHub report receipts…");
                      try {
                        const result = await retryReports(run.id);
                        setReportStatus(
                          `${result.settled} pending report${result.settled === 1 ? "" : "s"} settled.`,
                        );
                        await client.invalidateQueries({ queryKey: ["history", "run", run.id] });
                      } catch (error) {
                        setReportStatus(
                          error instanceof Error ? error.message : "Reporting is still pending.",
                        );
                      }
                    }}
                  >
                    Retry pending GitHub reports
                  </Button>
                  {reportStatus ? <span role="status">{reportStatus}</span> : null}
                </div>
              ) : null}
              <section className="work-brief">
                <h3>Recent activity</h3>
                {activity.data?.events.map((event) => (
                  <p key={event.cursor}>
                    {new Date(event.created_at).toLocaleString()} · {event.kind} ·{" "}
                    {typeof event.payload.summary === "string" ? event.payload.summary : ""}
                    {event.artifact_id ? (
                      <a
                        href={`/api/v1/artifacts/${encodeURIComponent(event.artifact_id)}?download=1`}
                      >
                        {" "}
                        Retained output
                      </a>
                    ) : null}
                  </p>
                ))}
                {activity.data?.next_before ? (
                  <p>Earlier activity is available in the run record.</p>
                ) : null}
              </section>
              <section className="work-brief">
                <h3>Retained output</h3>
                <p>
                  Removing output keeps run, attempt, message, check, and delivery receipts. Removed
                  output cannot be restored here.
                </p>
                {artifacts.data?.artifacts.map((artifact) => (
                  <div key={artifact.id} className="work-control-row">
                    <span>
                      Attempt {artifact.attempt_id} · {artifact.bytes_written} bytes ·{" "}
                      {artifact.retention_state}
                      {artifact.truncated ? " · truncated" : ""}
                    </span>
                    {artifact.retention_state === "retained" ? (
                      <>
                        <a href={`/api/v1/artifacts/${encodeURIComponent(artifact.id)}?download=1`}>
                          Download
                        </a>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={run.reservation_state !== "released"}
                          onClick={async () => {
                            if (
                              !window.confirm(
                                "Remove retained provider output for this settled attempt? Receipts will remain.",
                              )
                            )
                              return;
                            setRetentionError("");
                            try {
                              await removeArtifact(artifact.id);
                              await client.invalidateQueries({
                                queryKey: ["history", "run", run.id, "artifacts"],
                              });
                            } catch (error) {
                              setRetentionError(
                                error instanceof Error
                                  ? error.message
                                  : "Output could not be removed.",
                              );
                            }
                          }}
                        >
                          Remove output
                        </Button>
                      </>
                    ) : null}
                  </div>
                ))}
                {retentionError ? <p role="alert">{retentionError}</p> : null}
              </section>
              {bootstrap.current_run?.id === run.id ? (
                <Button asChild variant="outline">
                  <Link to="/work" search={{ project: run.project_id }}>
                    Open current work
                  </Link>
                </Button>
              ) : (
                <p className="work-note">
                  Historical receipts are read only. Controls belong to current Work.
                </p>
              )}
            </div>
          ) : null}
        </div>
      ))}
      {offset > 0 ? (
        <Button
          variant="outline"
          onClick={() => {
            setOffset(Math.max(0, offset - 30));
            setSelectedRun("");
          }}
        >
          Newer
        </Button>
      ) : null}
      {list.data?.runs.length === 30 ? (
        <Button
          variant="outline"
          onClick={() => {
            setOffset(offset + 30);
            setSelectedRun("");
          }}
        >
          Older
        </Button>
      ) : null}
    </section>
  );
}

function HistoricalMessage({ runID, messageID }: { runID: string; messageID: string }) {
  const [open, setOpen] = useState(false);
  const message = useQuery({ ...historicalMessageQuery(runID, messageID), enabled: open });
  return (
    <span>
      {" "}
      <Button size="sm" variant="ghost" onClick={() => setOpen(!open)}>
        {open ? "Hide full message" : "Read full message"}
      </Button>
      {open && message.data ? <span>{message.data.body}</span> : null}
      {open && message.isError ? (
        <span role="alert">Message unavailable: {message.error.message}</span>
      ) : null}
    </span>
  );
}
