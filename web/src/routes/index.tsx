import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { GitBranch, GitFork, ListOrdered, Play } from "lucide-react";
import { type FormEvent, useState } from "react";
import { useIssueEditor, useSetup } from "@/components/shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { QueueManifest } from "@/features/queue/queue-manifest";
import {
  addRepository,
  bootstrapQuery,
  inspectIssueOperation,
  issueOperationsQuery,
  projectIssuesQuery,
  projectsQuery,
  retryRepositorySetup,
  syncProjectIssues,
} from "@/lib/api";

export const Route = createFileRoute("/")({ component: Queue });

function Queue() {
  const { data } = useSuspenseQuery(projectsQuery);
  const { data: bootstrap } = useSuspenseQuery(bootstrapQuery);
  const selectedProjectID = useRouterState({ select: (state) => state.location.search.project });
  const project = data.projects.find((item) => item.id === selectedProjectID) ?? data.projects[0];
  const openSetup = useSetup();
  const openIssueEditor = useIssueEditor();
  const navigate = useNavigate();
  const [repositoryURL, setRepositoryURL] = useState("");
  const queryClient = useQueryClient();
  const retry = useMutation({
    mutationFn: retryRepositorySetup,
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey }),
  });
  const add = useMutation({
    mutationFn: () => addRepository(repositoryURL.trim()),
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey }),
    onSuccess: async (newProject) => {
      await queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey });
      await navigate({ to: "/", search: { project: newProject.id } });
      setRepositoryURL("");
    },
  });
  function submitRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    add.mutate();
  }
  const issueQuery = useQuery({
    ...projectIssuesQuery(project?.id ?? ""),
    enabled: Boolean(project),
  });
  const operationsQuery = useQuery({
    ...issueOperationsQuery(project?.id ?? ""),
    enabled: Boolean(project),
  });
  const inspect = useMutation({
    mutationFn: (requestKey: string) => inspectIssueOperation(project?.id ?? "", requestKey),
    onSettled: () => {
      if (project)
        void queryClient.invalidateQueries({ queryKey: issueOperationsQuery(project.id).queryKey });
    },
  });
  const sync = useMutation({
    mutationFn: syncProjectIssues,
    onSettled: () => {
      if (project)
        void queryClient.invalidateQueries({ queryKey: projectIssuesQuery(project.id).queryKey });
    },
  });
  if (project)
    return (
      <section className="page">
        <div className="page-title">
          <div>
            <p className="eyebrow" title={project.name}>
              {project.name}
            </p>
            <h1>Queue</h1>
            <p>What runs next, and what is waiting.</p>
          </div>
          <div className="page-title-actions">
            <Button variant="outline" onClick={openSetup}>
              Add repository
            </Button>
            <Button onClick={() => openIssueEditor(project.id, project.name)}>New task</Button>
            <Button asChild variant="outline">
              <Link to="/work" search={{ project: project.id }}>
                {bootstrap.current_run ? "Open Work" : "Start work"}
              </Link>
            </Button>
          </div>
        </div>
        {project.setup_state !== "ready" && (
          <div className="project-setup-alert" role="status">
            <div>
              <strong>Repository setup: {project.setup_stage.replaceAll("_", " ")}</strong>
              <p>
                {project.setup_error ||
                  (project.setup_stage === "source_pending"
                    ? "Checkout ready. Retry setup to finish issue and queue initialization."
                    : "Outpost is preparing this repository.")}
              </p>
            </div>
            <Button
              variant="outline"
              disabled={retry.isPending}
              onClick={() => retry.mutate(project.id)}
            >
              {retry.isPending ? "Retrying setup…" : "Retry setup"}
            </Button>
            {retry.isError && <p role="alert">{retry.error.message}</p>}
          </div>
        )}
        <QueueManifest
          key={project.id}
          projectID={project.id}
          projectName={project.name}
          repositoryURL={project.repository_url}
          issues={issueQuery.data?.issues ?? []}
        />
        <details className="issue-source-panel">
          <summary>GitHub issue sync and recent changes</summary>
          <div className="issue-source-actions">
            <Button
              variant="outline"
              disabled={sync.isPending}
              onClick={() => sync.mutate(project.id)}
            >
              {sync.isPending ? "Syncing…" : "Sync issues"}
            </Button>
          </div>
          <p className="issue-source-note">
            {issueQuery.data?.source.state === "ready"
              ? `Last successful sync: ${issueQuery.data.source.last_success_at}`
              : issueQuery.data?.source.last_error || "Issue source has not been synchronized yet."}
          </p>
          {sync.isError && (
            <p role="alert" className="form-error">
              {sync.error.message}
            </p>
          )}
          {issueQuery.isError && (
            <p role="alert" className="form-error">
              {issueQuery.error.message}
            </p>
          )}
          {operationsQuery.data?.operations.map((operation) => (
            <div key={operation.id} className="issue-receipt">
              <span>
                {operation.kind} #{operation.result_number || operation.issue_number || ""}:{" "}
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
            </div>
          ))}
          {inspect.isError && (
            <p role="alert" className="form-error">
              {inspect.error.message}
            </p>
          )}
        </details>
      </section>
    );
  return (
    <section className="page">
      <div className="page-title">
        <div>
          <p className="eyebrow">Your workspace</p>
          <h1>Queue</h1>
          <p>The next piece of work starts with a repository.</p>
        </div>
        <span className="count-label">0 repositories</span>
      </div>
      <div className="onboarding-panel">
        <div className="empty-state">
          <div className="empty-icon">
            <GitBranch size={27} strokeWidth={1.5} />
          </div>
          <p className="eyebrow">A place to move things forward</p>
          <h2>Bring your next project.</h2>
          <p>
            Connect a GitHub repository to keep its tasks, coding sessions, and results in one
            place.
          </p>
          <form className="onboarding-form" onSubmit={submitRepository}>
            <label htmlFor="first-repository-url">GitHub repository URL</label>
            <Input
              id="first-repository-url"
              type="url"
              required
              placeholder="https://github.com/owner/repo"
              value={repositoryURL}
              onChange={(event) => setRepositoryURL(event.target.value)}
            />
            <Button type="submit" disabled={add.isPending}>
              {add.isPending ? "Adding repository…" : "Add"}
            </Button>
          </form>
          {add.isPending && (
            <p role="status">
              Inspecting repository and preparing the managed checkout. This may take a moment.
            </p>
          )}
          {add.isError && (
            <p role="alert" className="form-error">
              {add.error.message}
            </p>
          )}
          <Button variant="ghost" onClick={openSetup}>
            More setup options
          </Button>
          <span className="quiet-note">One host. Separate projects. One active coding task.</span>
        </div>
        <div className="setup-steps">
          {[
            {
              icon: GitFork,
              title: "Connect a repository",
              text: "Use your existing GitHub login.",
            },
            {
              icon: ListOrdered,
              title: "Choose what’s next",
              text: "Issues define work. Your queue sets order.",
            },
            {
              icon: Play,
              title: "Start when you’re ready",
              text: "Review the plan before authorizing a run.",
            },
          ].map(({ icon: Icon, title, text }, index) => (
            <div key={title}>
              <span className="step-number">0{index + 1}</span>
              <Icon size={17} />
              <div>
                <h3>{title}</h3>
                <p>{text}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
