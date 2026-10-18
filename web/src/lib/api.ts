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
