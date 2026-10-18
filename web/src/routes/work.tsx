import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute, Link, useRouterState } from "@tanstack/react-router";
import { Terminal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StartPanel } from "@/features/queue/queue-manifest";
import { bootstrapQuery, projectIssuesQuery, projectsQuery } from "@/lib/api";

export const Route = createFileRoute("/work")({ component: Work });

function Work() {
  const { data: projects } = useSuspenseQuery(projectsQuery);
  const { data: bootstrap } = useSuspenseQuery({ ...bootstrapQuery, refetchInterval: 5000 });
  const selectedProjectID = useRouterState({ select: (state) => state.location.search.project });
  const project =
    projects.projects.find((item) => item.id === selectedProjectID) ?? projects.projects[0];
  const issues = useQuery({
    ...projectIssuesQuery(project?.id ?? ""),
    enabled: Boolean(project),
  });

  return (
    <section className="page">
      <div className="page-title">
        <div>
          <p className="eyebrow" title={project?.name}>
            {project?.name ?? "Execution"}
          </p>
          <h1>Work</h1>
          <p>{bootstrap.current_run ? "Current execution" : "Review what will run next."}</p>
        </div>
      </div>
      {!project ? (
        <div className="empty-state bordered-empty">
          <div className="empty-icon">
            <Terminal size={26} strokeWidth={1.5} />
          </div>
          <h2>Add a repository first</h2>
          <p>Work begins with a repository and a queued task.</p>
          <Button asChild>
            <Link to="/">Open Queue</Link>
          </Button>
        </div>
      ) : bootstrap.current_run ? (
        <div className="work-state" role="status">
          <p className="eyebrow">Execution slot occupied</p>
          <h2>
            {bootstrap.current_run.project_id === project.id
              ? project.name
              : "Another repository is running"}
          </h2>
          <p>
            Run {bootstrap.current_run.id} · {bootstrap.current_run.state} · worker{" "}
            {bootstrap.current_run.liveness}
          </p>
          <p>The start form is available when this run finishes.</p>
          {bootstrap.current_run.project_id !== project.id && (
            <Button asChild variant="outline">
              <Link to="/work" search={{ project: bootstrap.current_run.project_id }}>
                View active repository
              </Link>
            </Button>
          )}
        </div>
      ) : project.setup_state !== "ready" ? (
        <div className="work-state" role="status">
          <h2>Repository setup is still in progress</h2>
          <p>Finish setup before reviewing an assignment.</p>
          <Button asChild variant="outline">
            <Link to="/" search={{ project: project.id }}>
              Open Queue
            </Link>
          </Button>
        </div>
      ) : (
        <>
          <StartPanel
            key={project.id}
            projectID={project.id}
            projectName={project.name}
            issues={issues.data?.issues ?? []}
          />
          <p className="work-queue-link">
            Need to change the order or edit a task?{" "}
            <Link to="/" search={{ project: project.id }}>
              Open Queue
            </Link>
          </p>
        </>
      )}
    </section>
  );
}
