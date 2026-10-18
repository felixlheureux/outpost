import { useSuspenseQuery } from "@tanstack/react-query";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  ChevronDown,
  Circle,
  GitFork,
  History,
  ListOrdered,
  Moon,
  Plus,
  Radio,
  Sun,
  Terminal,
} from "lucide-react";
import { createContext, lazy, type ReactNode, Suspense, useContext, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { bootstrapQuery, projectsQuery } from "@/lib/api";

const AddRepositoryDialog = lazy(() => import("@/features/projects/add-repository-dialog"));

const SetupContext = createContext<() => void>(() => {});
export function useSetup() {
  return useContext(SetupContext);
}

export function Shell({ children }: { children: ReactNode }) {
  const { data: bootstrap } = useSuspenseQuery(bootstrapQuery);
  const { data: projectData } = useSuspenseQuery({
    ...projectsQuery,
    refetchInterval: (query) =>
      query.state.data?.projects.some((project) =>
        ["reserved", "inspecting", "cloning", "configuring"].includes(project.setup_stage),
      )
        ? 1500
        : false,
  });
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const selectedProjectID = useRouterState({ select: (state) => state.location.search.project });
  const selectedProject =
    projectData.projects.find((project) => project.id === selectedProjectID) ??
    projectData.projects[0];
  const runningProject = projectData.projects.find(
    (project) => project.id === bootstrap.current_run?.project_id,
  );
  const [open, setOpen] = useState(false);
  const [hasOpened, setHasOpened] = useState(false);
  const [url, setURL] = useState("");
  const [dark, setDark] = useState(() => document.documentElement.classList.contains("dark"));
  const title = pathname === "/work" ? "Work" : pathname === "/history" ? "History" : "Queue";
  function toggleTheme() {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    try {
      localStorage.setItem("outpost-theme", next ? "dark" : "light");
    } catch {
      /* Optional preference. */
    }
  }
  function openSetup() {
    setHasOpened(true);
    setOpen(true);
  }
  return (
    <SetupContext value={openSetup}>
      <div className="app-shell">
        <aside className="sidebar">
          <Link to="/" className="brand">
            <span className="brand-mark">
              <Radio size={20} />
            </span>
            Outpost<span className="brand-version">ALPHA</span>
          </Link>
          <div className="project-picker">
            <p className="eyebrow">Repository</p>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  className="project-trigger"
                  aria-label="Choose repository"
                >
                  <GitFork size={16} />
                  <span>{selectedProject?.name ?? "Add repository"}</span>
                  <ChevronDown size={14} />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                {projectData.projects.map((project) => (
                  <DropdownMenuItem key={project.id} asChild>
                    <Link to="/" search={{ project: project.id }}>
                      {project.name}
                    </Link>
                  </DropdownMenuItem>
                ))}
                <DropdownMenuItem onSelect={openSetup}>
                  <Plus size={16} />
                  Add repository
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          <nav aria-label="Main navigation">
            {[
              { to: "/" as const, label: "Queue", icon: ListOrdered },
              { to: "/work" as const, label: "Work", icon: Terminal },
              { to: "/history" as const, label: "History", icon: History },
            ].map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                search={{ project: selectedProject?.id }}
                activeOptions={{ exact: true }}
                activeProps={{ className: "nav-link active", "aria-current": "page" }}
                inactiveProps={{ className: "nav-link" }}
              >
                <Icon size={18} />
                {label}
              </Link>
            ))}
          </nav>
          <div className="sidebar-bottom">
            <div className="local-status">
              <Circle size={7} fill="currentColor" />
              <span>{bootstrap.mode === "private" ? "Private host" : "Local development"}</span>
            </div>
            <div className="sidebar-meta">
              <span>{bootstrap.version}</span>
              <Button
                variant="ghost"
                size="icon"
                aria-label={dark ? "Use light theme" : "Use dark theme"}
                onClick={toggleTheme}
              >
                {dark ? <Sun size={16} /> : <Moon size={16} />}
              </Button>
            </div>
          </div>
        </aside>
        <div className="main-column">
          <header className="topbar">
            <div>
              <span className="breadcrumb-root">Workspace</span>
              <span className="breadcrumb-slash">/</span>
              <span>{title}</span>
            </div>
            <span className="mode-label">
              {bootstrap.mode === "private" ? "PRIVATE" : "DEVELOPMENT"}
            </span>
          </header>
          <div className="starter-notice">
            <span className="notice-dot" />
            Repository checkout setup is available. Issue queue and coding runs are still being
            built.
          </div>
          <main id="main-content">{children}</main>
          <footer className="run-bar">
            <div>
              <Circle size={8} />
              <strong>
                {bootstrap.current_run
                  ? (runningProject?.name ?? bootstrap.current_run.id)
                  : "No active run"}
              </strong>
              <span>
                {bootstrap.current_run
                  ? `${bootstrap.current_run.state} · worker ${bootstrap.current_run.liveness}`
                  : "The execution slot is free."}
              </span>
            </div>
            <span className="run-bar-label">
              {bootstrap.mode === "private" ? "Private host" : "Local preview"}
            </span>
          </footer>
        </div>
        {hasOpened && (
          <Suspense
            fallback={
              <p role="status" className="loading-dialog">
                Opening repository setup…
              </p>
            }
          >
            <AddRepositoryDialog
              open={open}
              onOpenChange={setOpen}
              url={url}
              onURLChange={setURL}
            />
          </Suspense>
        )}
      </div>
    </SetupContext>
  );
}
