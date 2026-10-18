import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, useRouterState } from "@tanstack/react-router";
import { ArrowUpRight, GitBranch, GitFork, ListOrdered, Play } from "lucide-react";
import { useIssueEditor, useSetup } from "@/components/shell";
import { Button } from "@/components/ui/button";
import { QueueManifest } from "@/features/queue/queue-manifest";
import {
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
  const selectedProjectID = useRouterState({ select: (state) => state.location.search.project });
  const project = data.projects.find((item) => item.id === selectedProjectID) ?? data.projects[0];
  const openSetup = useSetup();
  const openIssueEditor = useIssueEditor();
  const queryClient = useQueryClient();
  const retry = useMutation({
    mutationFn: retryRepositorySetup,
    onSettled: () => queryClient.invalidateQueries({ queryKey: projectsQuery.queryKey }),
  });
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
            <p className="eyebrow">{project.name}</p>
            <h1>Queue</h1>
            <p>Repository setup: {project.setup_stage.replaceAll("_", " ")}</p>
          </div>
          <Button onClick={openSetup}>Add repository</Button>
        </div>
        <div className="project-list">
          <article>
            <GitFork size={20} />
            <div>
              <h2>{project.name}</h2>
              {project.setup_stage === "source_pending" ? (
                <p>Checkout ready. Issue and queue setup is pending.</p>
              ) : (
                <p>Setup: {project.setup_stage.replaceAll("_", " ")}</p>
              )}
              {project.setup_error && <p role="status">{project.setup_error}</p>}
              {project.setup_state === "failed" && (
                <Button
                  variant="outline"
                  disabled={retry.isPending}
                  onClick={() => retry.mutate(project.id)}
                >
                  {retry.isPending ? "Retrying setup…" : "Retry setup"}
                </Button>
              )}
              {retry.isError && <p role="alert">{retry.error.message}</p>}
            </div>
            <a
              href={project.repository_url}
              target="_blank"
              rel="noreferrer"
              aria-label={`Open ${project.name} on GitHub`}
            >
              <ArrowUpRight size={18} />
            </a>
          </article>
        </div>
        <QueueManifest
          projectID={project.id}
          repositoryURL={project.repository_url}
          issues={issueQuery.data?.issues ?? []}
        />
        <div className="issue-source-panel">
          <div className="issue-source-heading">
            <div>
              <p className="eyebrow">GitHub source</p>
              <h2 className="issue-source-title">Issues</h2>
            </div>
            <div className="issue-source-actions">
              <Button variant="outline" onClick={() => openIssueEditor(project.id, project.name)}>
                Create issue
              </Button>
              <Button
                variant="outline"
                disabled={sync.isPending}
                onClick={() => sync.mutate(project.id)}
              >
                {sync.isPending ? "Syncing…" : "Sync issues"}
              </Button>
            </div>
          </div>
          <p className="issue-source-note">
            {issueQuery.data?.source.state === "ready"
              ? `Cached ${issueQuery.data.issues.length} issues.`
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
          {issueQuery.data && issueQuery.data.issues.length > 0 && (
            <div className="issue-list">
              {issueQuery.data.issues.map((issue) => (
                <div className="issue-row" key={issue.number}>
                  <span>#{issue.number}</span>
                  <a href={issue.url} target="_blank" rel="noreferrer">
                    <strong>{issue.title}</strong>
                  </a>
                  <small>
                    {issue.parse_error ||
                      (issue.stale
                        ? "Not seen in latest sync · inspect source"
                        : issue.state === "closed"
                          ? "Closed on GitHub · unverified"
                          : "Open")}
                  </small>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => openIssueEditor(project.id, project.name, issue)}
                  >
                    Edit
                  </Button>
                </div>
              ))}
            </div>
          )}
          {operationsQuery.data && operationsQuery.data.operations.length > 0 && (
            <div className="issue-receipts">
              <h3>Recent issue changes</h3>
              {operationsQuery.data.operations.map((operation) => (
                <div key={operation.id} className="issue-receipt">
                  <span>
                    {operation.kind === "create" ? "Created" : "Edited"} issue{" "}
                    {operation.result_number || operation.issue_number || ""}
                  </span>
                  <span>{operation.state}</span>
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
            </div>
          )}
        </div>
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
          <Button size="lg" onClick={openSetup}>
            <GitFork size={17} />
            Add repository
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
