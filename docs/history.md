# History and recovery

History reads durable run, schedule, assignment, attempt, provider, check,
delivery, reporting, activity, and artifact receipts. It includes runs that
failed before assignment admission. Run details are read only; only the exact
currently reserved run links back to Work for controls. Historical attempts
retain their own outcome when a later continuation succeeds or fails.

The current Work attention panel shows the recorded reason and current worker
observation. An unknown observation means ownership has not been proved either
way. Provider interruption or failure uses the existing scoped continuation
control. Missing saved session identity offers only a fresh linked conversation.
Queue/source failures link to Queue sync, check failures to Checks output,
checkout/baseline failures to Changes, and unsettled process ownership requires
observation or the scoped Work stop flow. Reporting is independent of coding:
after a run's reservation is released, History's **Retry pending GitHub
reports** reuses the durable report marker and original close-issue policy.
Delivery recovery still uses `outpost delivery recover --run-id ID` because it
requires the workspace lock and exact process checks.

The contextual Connection panel observes the configured private URL and allowed
login, current Tailscale node and Serve root, web unit, host CLI sign-ins,
project source cache state, private state permissions, available disk space,
and retained output size. These are current observations, not a guarantee that
a device can reach the dashboard or that a particular GitHub write will pass.
Unavailable host calls are reported as unknown or unavailable, never stopped.
The panel does not return paths, tokens, command output, or source error text.

History lists at most 30 runs per page, 100 assignments/attempts in a detail,
100 recent activity events, and 100 artifact records. Earlier activity is
available through the paged activity API. Retained provider output can be
downloaded by artifact ID. **Remove output** requires a released run and a
settled assignment; the server's allowlisted artifact path guard and retention
state are authoritative. Removal keeps run, attempt, message, check, delivery,
and report receipts, and the artifact manifest records its removed state.
