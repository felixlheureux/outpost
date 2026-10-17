import { createFileRoute } from "@tanstack/react-router";
import { Terminal } from "lucide-react";

export const Route = createFileRoute("/work")({ component: Work });
function Work() {
  return (
    <section className="page">
      <div className="page-title">
        <div>
          <p className="eyebrow">Execution</p>
          <h1>Work</h1>
          <p>Live instructions and activity will belong to the active run.</p>
        </div>
      </div>
      <div className="empty-state bordered-empty">
        <div className="empty-icon">
          <Terminal size={26} strokeWidth={1.5} />
        </div>
        <h2>No active run</h2>
        <p>The worker isn’t connected in this starter. Nothing is running in the background.</p>
      </div>
    </section>
  );
}
