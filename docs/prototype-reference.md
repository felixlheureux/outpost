# Interactive UX reference

Open `../Outpost-UX-Reference.html` in a modern browser. The exported reference
bundles its UI and icons and does not require Vite, Node, GitHub access, or a backend.
It follows the browser's light/dark preference. Reload resets illustrative state.

Explore task selection and movement, Inbox/task creation, Queue/Work/History,
Activity/Changes/Checks, instruction entry, Finish task & stop/Undo, and
Interrupt → saved/fresh Continue. Settings and connection panels illustrate where
secondary controls belong.

The content is fictional and in memory. This reference does not execute work,
write GitHub Issues, implement real checks, authenticate users, preserve edits
across reload, or cover every production failure/empty/conflict state. It is not
evidence of actual Codex/systemd/Tailscale integration.

Use it for layout density and interaction intent. Rebuild in React/shadcn using
the real API/state owners. The export wrapper and its local event handlers are
not the proposed production component architecture.

The spec takes precedence where the concept simplifies behavior: responsive
inspector sheet/detail route, truthful remote saving/conflict/unknown states,
source outages, message receipts, and landing in Work when active. Sample issue
numbers never seed the actual queue.

The export was checked for complete structure and JavaScript syntax. Production
browser/device accessibility and responsive acceptance remain explicit tasks;
this file alone does not establish those checks.
