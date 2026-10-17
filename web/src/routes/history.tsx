import { createFileRoute } from "@tanstack/react-router";
import { History } from "lucide-react";

export const Route = createFileRoute("/history")({ component: HistoryPage });
function HistoryPage() {
  return (
    <section className="page">
      <div className="page-title">
        <div>
          <p className="eyebrow">Results</p>
          <h1>History</h1>
          <p>Completed attempts, checks, and delivery receipts will appear here.</p>
        </div>
      </div>
      <div className="empty-state bordered-empty">
        <div className="empty-icon">
          <History size={26} strokeWidth={1.5} />
        </div>
        <h2>No attempts yet</h2>
        <p>There’s no execution history in this installation.</p>
      </div>
    </section>
  );
}
