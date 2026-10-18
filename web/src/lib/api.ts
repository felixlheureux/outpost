import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

const bootstrap = z.object({
  version: z.string(),
  mode: z.enum(["development", "private"]),
  current_run: z
    .object({
      id: z.string(),
      project_id: z.string(),
      state: z.string(),
      liveness: z.enum(["active", "stopped", "unknown"]),
      stop_requested: z.boolean().optional(),
    })
    .nullable(),
  capabilities: z.object({ repository_setup: z.boolean(), execution: z.boolean() }),
});

const projects = z.object({
  projects: z.array(
    z.object({
      id: z.string(),
      name: z.string(),
      repository_url: z.url(),
      setup_state: z.enum(["pending", "ready", "failed"]),
      setup_stage: z.string(),
      setup_error: z.string().optional(),
      created_at: z.iso.datetime(),
    }),
  ),
});
const project = projects.shape.projects.element;

const errorResponse = z.object({
  error: z.object({ code: z.string(), message: z.string(), request_id: z.string() }),
});

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public requestID?: string,
  ) {
    super(message);
  }
}

// Runtime validation lives at the network boundary. Components never consume unchecked JSON.
async function request<T>(url: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${url}`, { credentials: "same-origin", ...init });
  const body: unknown = await response.json();
  if (!response.ok) {
    const parsed = errorResponse.safeParse(body);
    if (parsed.success) {
      const { code, message, request_id } = parsed.data.error;
      throw new ApiError(response.status, code, message, request_id);
    }
    throw new ApiError(
      response.status,
      "invalid_response",
      "The server returned an unexpected response.",
    );
  }
  return schema.parse(body);
}

export const bootstrapQuery = queryOptions({
  queryKey: ["bootstrap"],
  queryFn: () => request("/bootstrap", bootstrap),
});
export const projectsQuery = queryOptions({
  queryKey: ["projects"],
  queryFn: () => request("/projects", projects),
});

const historyAttempt = z.object({
  id: z.string(),
  ordinal: z.number(),
  phase: z.string(),
  outcome: z.string(),
  provider_state: z.string(),
  failure_code: z.string(),
  model: z.string(),
  reasoning: z.string(),
  sandbox: z.string(),
  approval_policy: z.string(),
  created_at: z.string(),
  settled_at: z.string(),
});
const historyAssignment = z.object({
  id: z.string(),
  issue_number: z.number(),
  title: z.string(),
  issue_digest: z.string(),
  issue_revision: z.number(),
  queue_sha: z.string(),
  baseline_head: z.string(),
  branch: z.string(),
  state: z.string(),
  created_at: z.string(),
  settled_at: z.string(),
  attempts: z.array(historyAttempt),
});
const historyRun = z.object({
  id: z.string(),
  project_id: z.string(),
  state: z.string(),
  reservation_state: z.string(),
  launch_observation: z.string(),
  schedule_outcome: z.string(),
  schedule_reason: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
  successful_count: z.number(),
  task_limit: z.number(),
  assignment_count: z.number(),
  assignments: z.array(historyAssignment),
});
export type HistoryRun = z.infer<typeof historyRun>;
export function historyQuery(projectID: string, offset = 0) {
  return queryOptions({
    queryKey: ["history", projectID, offset],
    queryFn: () =>
      request(
        `/history?project=${encodeURIComponent(projectID)}&offset=${offset}`,
        z.object({ runs: z.array(historyRun), next_offset: z.number() }),
      ),
    enabled: Boolean(projectID),
  });
}
export function historyDetailQuery(runID: string) {
  return queryOptions({
    queryKey: ["history", "run", runID],
    queryFn: () => request(`/history/${encodeURIComponent(runID)}`, historyRun),
    enabled: Boolean(runID),
  });
}
const historyArtifacts = z.object({
  artifacts: z.array(
    z.object({
      id: z.string(),
      attempt_id: z.string(),
      bytes_written: z.number(),
      truncated: z.boolean(),
      retention_state: z.string(),
      removed_at: z.string(),
    }),
  ),
});
export function historyArtifactsQuery(runID: string) {
  return queryOptions({
    queryKey: ["history", "run", runID, "artifacts"],
    queryFn: () => request(`/history/${encodeURIComponent(runID)}/artifacts`, historyArtifacts),
    enabled: Boolean(runID),
  });
}
export function removeArtifact(id: string) {
  return request(`/artifacts/${encodeURIComponent(id)}`, z.object({ state: z.string() }), {
    method: "DELETE",
  });
}
export function retryReports(runID: string) {
  return request(
    `/runs/${encodeURIComponent(runID)}/reports/retry`,
    z.object({ settled: z.number() }),
    { method: "POST" },
  );
}
const diagnostics = z.object({
  version: z.string(),
  observed_at: z.string(),
  mode: z.string(),
  private_url: z.string(),
  allowed_identity: z.string(),
  tailscale: z.string(),
  serve_route: z.string(),
  web_service: z.string(),
  github: z.string(),
  provider: z.string(),
  provider_version: z.string().optional(),
  state_access: z.boolean().optional(),
  disk_available_bytes: z.number().optional(),
  project_setup: z.string().optional(),
  issue_source: z.string().optional(),
  queue_source: z.string().optional(),
  source_observed_at: z.string().optional(),
  artifacts: z
    .object({ count: z.number(), recorded_bytes: z.number(), truncated_count: z.number() })
    .optional(),
});
export function diagnosticsQuery(projectID: string) {
  return queryOptions({
    queryKey: ["diagnostics", projectID],
    queryFn: () => request(`/diagnostics?project=${encodeURIComponent(projectID)}`, diagnostics),
    staleTime: 10_000,
    retry: false,
  });
}

const agentCapabilities = z.object({
  version: z.string(),
  models: z.array(
    z.object({
      model: z.string(),
      displayName: z.string(),
      hidden: z.boolean(),
      isDefault: z.boolean(),
      defaultReasoningEffort: z.string(),
      supportedReasoningEfforts: z.array(z.object({ reasoningEffort: z.string() })),
    }),
  ),
  resume: z.boolean(),
  steer: z.boolean(),
  interrupt: z.boolean(),
  structured_results: z.boolean(),
  permission_controls: z.boolean(),
});

export function agentCapabilitiesQuery(projectID: string) {
  return queryOptions({
    queryKey: ["projects", projectID, "agent", "capabilities"],
    queryFn: () =>
      request(`/agent/capabilities?project=${encodeURIComponent(projectID)}`, agentCapabilities),
    staleTime: 60_000,
    retry: false,
  });
}

export const activityEvent = z.object({
  cursor: z.number().int().nonnegative(),
  project_id: z.string(),
  run_id: z.string(),
  attempt_id: z.string(),
  kind: z.string(),
  payload: z.record(z.string(), z.unknown()),
  artifact_id: z.string().optional(),
  created_at: z.string(),
});
export type ActivityEvent = z.infer<typeof activityEvent>;

export const activitySnapshot = z.object({
  cursor: z.number().int().nonnegative(),
  oldest_cursor: z.number().int().nonnegative(),
  pruned_through: z.number().int().nonnegative().optional(),
  run_id: z.string(),
  project_id: z.string(),
  run_state: z.string(),
  assignment_id: z.string().optional(),
  attempt_phase: z.string().optional(),
  provider_state: z.string().optional(),
  provider_version: z.string().optional(),
  model: z.string().optional(),
  reasoning: z.string().optional(),
  sandbox: z.string().optional(),
  approval_policy: z.string().optional(),
});
export const activityPage = z.object({
  snapshot: activitySnapshot,
  events: z.array(activityEvent),
  next_before: z.number().int().nonnegative(),
  history_truncated: z.boolean(),
});
export type ActivityPage = z.infer<typeof activityPage>;
export function activityPageQuery(runID: string) {
  return queryOptions({
    queryKey: ["runs", runID, "activity"],
    queryFn: () => request(`/runs/${encodeURIComponent(runID)}/activity`, activityPage),
    retry: false,
  });
}
export function olderActivity(runID: string, before: number) {
  return request(`/runs/${encodeURIComponent(runID)}/activity?before=${before}`, activityPage);
}
export const artifactPreview = z.object({
  artifact_id: z.string(),
  text: z.string(),
  offset: z.number().int().nonnegative(),
  next_offset: z.number().int().nonnegative(),
  retained_bytes: z.number().int().nonnegative(),
  capture_truncated: z.boolean(),
  preview_truncated: z.boolean(),
  download_url: z.string(),
});
export function artifactPreviewQuery(id: string) {
  return queryOptions({
    queryKey: ["artifacts", id, "preview"],
    queryFn: () => request(`/artifacts/${encodeURIComponent(id)}`, artifactPreview),
    retry: false,
  });
}

const workMessage = z.object({
  id: z.string(),
  run_id: z.string(),
  assignment_id: z.string(),
  attempt_id: z.string(),
  thread_id: z.string(),
  expected_turn_id: z.string(),
  target: z.enum(["live", "next"]),
  body: z.string(),
  digest: z.string(),
  state: z.enum(["queued", "sending", "accepted", "rejected", "unknown"]),
  reason: z.string(),
  contract_changed: z.boolean(),
  created_at: z.string(),
  updated_at: z.string(),
});
const workControl = z.object({
  id: z.string(),
  run_id: z.string(),
  attempt_id: z.string(),
  action: z.enum(["finish", "undo", "interrupt", "continue"]),
  mode: z.string(),
  instructions: z.string(),
  digest: z.string(),
  state: z.enum(["queued", "sending", "accepted", "rejected", "unknown"]),
  reason: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});
const workOverview = z.object({
  run_id: z.string(),
  project_id: z.string(),
  run_state: z.string(),
  assignment_id: z.string(),
  review_candidate_digest: z.string().optional(),
  assignment_started_at: z.string().optional(),
  attempt_id: z.string(),
  attempt_phase: z.string(),
  attempt_outcome: z.string(),
  provider_state: z.string(),
  provider_failure_code: z.string().optional(),
  thread_id: z.string(),
  turn_id: z.string(),
  turn_open: z.boolean(),
  issue_number: z.number(),
  snapshot: z
    .object({
      issue_number: z.number(),
      title: z.string(),
      body: z.string(),
      issue_url: z.string().optional(),
    })
    .passthrough()
    .nullable(),
  model: z.string(),
  reasoning: z.string(),
  sandbox: z.string(),
  approval_policy: z.string(),
  successful_count: z.number(),
  task_limit: z.number(),
  stop_requested: z.boolean(),
  schedule_outcome: z.string(),
  schedule_reason: z.string(),
  messages: z.array(workMessage),
  controls: z.array(workControl),
  checks: z.array(
    z.object({
      id: z.string(),
      check_id: z.string(),
      name: z.string(),
      required: z.boolean(),
      status: z.string(),
      reason: z.string(),
      input_digest: z.string(),
      candidate_digest: z.string(),
      contract_digest: z.string(),
      duration_ms: z.number(),
      exit_code: z.number().nullable(),
      signal: z.string(),
      case_count: z.number().nullable(),
      require_cases: z.boolean(),
      started_at: z.string(),
      finished_at: z.string(),
      pid: z.number(),
      output_bytes: z.number(),
      truncated: z.boolean(),
    }),
  ),
  delivery: z
    .object({
      id: z.string(),
      policy: z.string(),
      candidate_digest: z.string(),
      branch: z.string(),
      upstream: z.string(),
      commit_sha: z.string(),
      state: z.string(),
      reason: z.string(),
    })
    .nullable()
    .optional(),
  report: z.object({ id: z.string(), state: z.string(), reason: z.string() }).nullable().optional(),
  attempts: z
    .array(
      z.object({
        id: z.string(),
        ordinal: z.number(),
        phase: z.string(),
        outcome: z.string(),
        predecessor_id: z.string(),
        created_at: z.string(),
        settled_at: z.string(),
      }),
    )
    .optional(),
});
export type WorkOverview = z.infer<typeof workOverview>;
const reviewDecision = z.object({
  assignment_id: z.string(),
  candidate_digest: z.string(),
  decision: z.enum(["approved", "rejected"]),
});
export function decideReview(
  runID: string,
  assignmentID: string,
  candidate: string,
  decision: "approved" | "rejected",
  feedback: string,
) {
  return request(`/runs/${encodeURIComponent(runID)}/review`, reviewDecision, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      assignment_id: assignmentID,
      candidate_digest: candidate,
      decision,
      feedback,
    }),
  });
}
const checkOutput = z.object({ id: z.string(), output: z.string(), truncated: z.boolean() });
export function checkOutputQuery(runID: string, receiptID: string) {
  return queryOptions({
    queryKey: ["runs", runID, "checks", receiptID, "output"],
    queryFn: () =>
      request(
        `/runs/${encodeURIComponent(runID)}/checks/${encodeURIComponent(receiptID)}/output`,
        checkOutput,
      ),
    enabled: Boolean(receiptID),
    retry: false,
  });
}
export function workOverviewQuery(runID: string) {
  return queryOptions({
    queryKey: ["runs", runID, "work"],
    queryFn: () => request(`/runs/${encodeURIComponent(runID)}/work`, workOverview),
    refetchInterval: 1500,
    retry: false,
  });
}
export function sendWorkMessage(
  runID: string,
  input: {
    id: string;
    attempt_id: string;
    thread_id: string;
    expected_turn_id: string;
    target: "live" | "next";
    body: string;
  },
) {
  return request(`/runs/${encodeURIComponent(runID)}/messages`, workMessage, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}
export function sendWorkControl(
  runID: string,
  input: {
    id: string;
    attempt_id: string;
    action: "finish" | "undo" | "interrupt" | "continue";
    mode?: string;
    instructions?: string;
  },
) {
  return request(`/runs/${encodeURIComponent(runID)}/controls`, workControl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}
const workChanges = z.object({
  paths: z.array(z.object({ path: z.string(), status: z.string() })),
  truncated: z.boolean(),
});
const workDiff = z.object({ path: z.string(), diff: z.string(), truncated: z.boolean() });
export function workChangesQuery(runID: string) {
  return queryOptions({
    queryKey: ["runs", runID, "changes"],
    queryFn: () => request(`/runs/${encodeURIComponent(runID)}/changes`, workChanges),
    refetchInterval: 3000,
    retry: false,
  });
}
export function workDiffQuery(runID: string, path: string) {
  return queryOptions({
    queryKey: ["runs", runID, "changes", path],
    queryFn: () =>
      request(
        `/runs/${encodeURIComponent(runID)}/changes?path=${encodeURIComponent(path)}`,
        workDiff,
      ),
    enabled: Boolean(path),
    retry: false,
  });
}

export function addRepository(repositoryURL: string) {
  return request("/projects", project.omit({ created_at: true }), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ repository_url: repositoryURL }),
  });
}

export function addExistingRepository(checkoutPath: string) {
  return request("/projects/existing", project.omit({ created_at: true }), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ checkout_path: checkoutPath }),
  });
}

export function retryRepositorySetup(projectID: string) {
  return request(
    `/projects/${encodeURIComponent(projectID)}/retry`,
    project.omit({ created_at: true }),
    {
      method: "POST",
    },
  );
}

export const issueRequirements = z.object({
  title: z.string(),
  outcome: z.string(),
  criteria: z.array(z.string()),
  notes: z.string(),
  settings: z.object({
    outpost_task: z.number().optional(),
    kind: z.string(),
    depends_on: z.array(z.number()).nullable().optional(),
    hold_reason: z.string().nullable().optional(),
    check_profile: z.string(),
    delivery: z.string(),
    review: z.string(),
  }),
  has_settings: z.boolean(),
});

export const issueSource = z.object({
  source: z.object({
    state: z.enum(["not_synced", "ready", "needs_attention"]),
    last_success_at: z.string(),
    last_error: z.string(),
  }),
  issues: z.array(
    z.object({
      number: z.number().int().positive(),
      url: z.url(),
      title: z.string(),
      body: z.string(),
      state: z.enum(["open", "closed"]),
      source_updated_at: z.string(),
      content_digest: z.string(),
      fetched_at: z.string(),
      local_revision: z.number().int().positive(),
      requirements: issueRequirements.nullable(),
      parse_error: z.string(),
      stale: z.boolean(),
    }),
  ),
});

export function projectIssuesQuery(projectID: string) {
  return queryOptions({
    queryKey: ["projects", projectID, "issues"],
    queryFn: () => request(`/projects/${encodeURIComponent(projectID)}/issues`, issueSource),
    refetchInterval: 30_000,
  });
}

export type IssueSource = z.infer<typeof issueSource>["issues"][number];

const issueOperation = z.object({
  id: z.string(),
  request_key: z.string(),
  kind: z.enum(["create", "edit"]),
  state: z.enum(["pending", "confirmed", "rejected", "conflict", "unknown"]),
  issue_number: z.number().int().nonnegative(),
  result_number: z.number().int().nonnegative(),
  reason: z.string(),
  created_at: z.string(),
});

export function issueOperationsQuery(projectID: string) {
  return queryOptions({
    queryKey: ["projects", projectID, "issue-operations"],
    queryFn: () =>
      request(
        `/projects/${encodeURIComponent(projectID)}/issue-operations`,
        z.object({ operations: z.array(issueOperation) }),
      ),
  });
}

export function inspectIssueOperation(projectID: string, requestKey: string) {
  return request(
    `/projects/${encodeURIComponent(projectID)}/issue-operations/${encodeURIComponent(requestKey)}/inspect`,
    issueOperation,
    { method: "POST" },
  );
}

export function writeIssue(
  projectID: string,
  input: {
    request_key: string;
    title: string;
    body: string;
    issue_number?: number;
    expected_digest?: string;
    expected_revision?: number;
  },
) {
  const path = input.issue_number ? `/issues/${input.issue_number}` : "/issues";
  return request(`/projects/${encodeURIComponent(projectID)}${path}`, issueOperation, {
    method: input.issue_number ? "PATCH" : "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function syncProjectIssues(projectID: string) {
  return request(
    `/projects/${encodeURIComponent(projectID)}/issues/sync`,
    z.object({ observed: z.number().int().nonnegative() }),
    {
      method: "POST",
    },
  );
}

export const queueSource = z.object({
  sha: z.string(),
  issues: z.array(z.number().int().positive()),
  state: z.enum(["not_synced", "ready", "needs_attention"]),
  fetched_at: z.string(),
  last_error: z.string(),
});

export function projectQueueQuery(projectID: string) {
  return queryOptions({
    queryKey: ["projects", projectID, "queue"],
    queryFn: () => request(`/projects/${encodeURIComponent(projectID)}/queue`, queueSource),
    refetchInterval: 30_000,
  });
}

export function syncProjectQueue(projectID: string) {
  return request(`/projects/${encodeURIComponent(projectID)}/queue/sync`, queueSource, {
    method: "POST",
  });
}

const admissionPreview = z.object({
  selected: z.number().int().nonnegative(),
  waiting: z.array(z.object({ issue: z.number().int(), code: z.string(), reason: z.string() })),
  outcome: z.string(),
  queue_sha: z.string(),
  fetched_at: z.string(),
});

export function previewAdmission(projectID: string, mode: "follow" | "fixed", selected: number) {
  return request(`/projects/${encodeURIComponent(projectID)}/admission-preview`, admissionPreview, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode, selected }),
  });
}

const startedRun = z.object({
  run_id: z.string(),
  state: z.enum(["reserved", "launch_unknown", "completed"]),
});
export type StartRunInput = {
  mode: "follow" | "fixed";
  selected: number;
  model: string;
  reasoning: string;
  delivery: "local_changes" | "commit" | "push";
  review: "none" | "required";
  close_issue: boolean;
  task_limit: number;
  deadline_at: string;
  instructions: string;
  request_key: string;
};
export function startRun(projectID: string, input: StartRunInput) {
  return request(`/projects/${encodeURIComponent(projectID)}/runs`, startedRun, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

const queueOperation = z.object({
  id: z.string(),
  request_key: z.string(),
  kind: z.enum(["replace", "append", "remove", "move"]),
  state: z.enum(["pending", "confirmed", "rejected", "conflict", "unknown"]),
  expected_sha: z.string(),
  intended: z.object({
    request_key: z.string(),
    expected_sha: z.string(),
    kind: z.enum(["replace", "append", "remove", "move"]),
    issues: z.array(z.number().int().positive()).optional(),
    issue: z.number().int().positive().optional(),
    before: z.number().int().positive().optional(),
  }),
  current_issues: z.array(z.number().int().positive()),
  reason: z.string(),
  created_at: z.string(),
});

export function queueOperationsQuery(projectID: string) {
  return queryOptions({
    queryKey: ["projects", projectID, "queue-operations"],
    queryFn: () =>
      request(
        `/projects/${encodeURIComponent(projectID)}/queue/operations`,
        z.object({ operations: z.array(queueOperation) }),
      ),
  });
}

export function writeQueue(
  projectID: string,
  input: {
    request_key: string;
    expected_sha: string;
    kind: "append" | "remove" | "move" | "replace";
    issue?: number;
    before?: number;
    issues?: number[];
  },
) {
  return request(`/projects/${encodeURIComponent(projectID)}/queue/operations`, queueOperation, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function inspectQueueOperation(projectID: string, requestKey: string) {
  return request(
    `/projects/${encodeURIComponent(projectID)}/queue/operations/${encodeURIComponent(requestKey)}/inspect`,
    queueOperation,
    { method: "POST" },
  );
}
