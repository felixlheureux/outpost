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
