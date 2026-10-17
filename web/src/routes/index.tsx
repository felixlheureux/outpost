import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ArrowUpRight, GitBranch, GitFork, ListOrdered, Play } from "lucide-react";
import { useSetup } from "@/components/shell";
import { Button } from "@/components/ui/button";
import { projectsQuery } from "@/lib/api";

export const Route = createFileRoute("/")({ component: Queue });

function Queue() {
  const { data } = useSuspenseQuery(projectsQuery);
  const openSetup = useSetup();
  if (data.projects.length > 0)
    return (
      <section className="page">
        <div className="page-title">
          <div>
            <p className="eyebrow">Repositories</p>
            <h1>Your projects</h1>
          </div>
          <Button onClick={openSetup}>Add repository</Button>
        </div>
        <div className="project-list">
          {data.projects.map((project) => (
            <article key={project.id}>
              <GitFork size={20} />
              <div>
                <h2>{project.name}</h2>
                <p>Setup: {project.setup_state}</p>
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
          ))}
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
