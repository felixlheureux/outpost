---
planning_id: OP-002
status: published
order: 2
depends_on: ["OP-001"]
source: bootstrap-handoff
github_issue: 2
---
# OP-002 — Make Tailscale access and supervised execution real

**Dependencies:** #1 (OP-001)  
**Specification:** §§6, 12, 15, 22–23, 25.  
**Acceptance owners:** AC-01–04, AC-12–17, AC-25–26 at supervision level.

**Canonical task:** https://github.com/felixlheureux/outpost/issues/2  
**Repository copy:** Published planning snapshot; update the issue for current requirements.

### Outcome

The application is reachable privately through Tailscale, and a supervised fixture worker keeps running when the web service or browser goes away. Ownership uncertainty cannot launch a second writer.

### Implementation requirements

1. Add installation/configuration and user-systemd integration. Separate the long-lived web unit from uniquely named run worker units invoking the same executable with a run identity.
2. Inspect current Tailscale status/Serve configuration and select a route without replacing unrelated routes. Bind Echo to loopback. Print the actual private HTTPS address and allowed identity after verified setup.
3. Trust Tailscale identity headers only through the configured local access boundary; deny missing/disallowed identities across HTML, API, SSE, and artifacts. Add exact-origin/CSRF protections to mutations. Do not open a public port or enable Funnel.
4. Persist run reservation/launch intent before D-Bus effects. Record unique run token/unit identity and check worker ownership before repository or agent effects. Do not hold a database transaction during systemd calls.
5. Reconcile missing/lost launch acknowledgements by observing the recorded unit and process identity. Replayed starts return the existing receipt; duplicate worker claims fail safely.
6. Define worker/check process-group settlement and OS workspace-lock lifetime. An owned surviving child retains exclusion even if its parent disappears. Normal recovery uses observed ownership, never deleting a lock file.
7. Add install/status/stop diagnostics for absent user bus, disabled lingering, unavailable Tailscale, unavailable CLI prerequisites, unwritable directories, and unknown owned processes. Keep requested host changes explicit and narrow.

### Failure and edge cases

Browser disconnection proves nothing about worker state. A successful systemd job acknowledgement does not prove task completion. A web restart must not kill the worker; a VM reboot must not silently resume an arbitrary effect. Keep reservation until known worker/check ownership is reconciled.

### Acceptance criteria

- [ ] A real user-systemd fixture survives web-service restart and can be observed/stopped afterward.
- [ ] Concurrent/replayed starts establish one reservation and one worker.
- [ ] Lost launch acknowledgement attaches to the existing unit rather than launching twice.
- [ ] A surviving check fixture prevents another writer and exposes a relevant stop action.
- [ ] Existing Serve routes survive installation; allowed/private access succeeds and denied identity/origin controls fail.
- [ ] Closing the browser leaves execution unaffected.

### Verification and handoff

Use deterministic local process fixtures for ownership races, then a real Linux user-systemd/Tailscale smoke journey. Label unavailable host checks explicitly. Handoff exposes the supervisor interface, reservation lifecycle, process identity receipt, and current liveness projection for the scheduler and UI.
