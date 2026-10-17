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
import { bootstrapQuery } from "@/lib/api";

const AddRepositoryDialog = lazy(() => import("@/features/projects/add-repository-dialog"));

const SetupContext = createContext<() => void>(() => {});
export function useSetup() {
  return useContext(SetupContext);
}

export function Shell({ children }: { children: ReactNode }) {
  const { data: bootstrap } = useSuspenseQuery(bootstrapQuery);
  const pathname = useRouterState({ select: (state) => state.location.pathname });
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
                <Button variant="outline" className="project-trigger" aria-label="Add repository">
                  <GitFork size={16} />
                  <span>Add repository</span>
                  <ChevronDown size={14} />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
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
              <span>Local development</span>
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
            <span className="mode-label">STARTER</span>
          </header>
          <div className="starter-notice">
            <span className="notice-dot" />
            Local preview · Repository setup and coding runs are still being built.
          </div>
          <main id="main-content">{children}</main>
          <footer className="run-bar">
            <div>
              <Circle size={8} />
              <strong>No active run</strong>
              <span>The execution slot is free.</span>
            </div>
            <span className="run-bar-label">Host-local preview</span>
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
