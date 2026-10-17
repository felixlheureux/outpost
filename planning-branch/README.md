# Planning branch seed

The adjacent `queue.json` is an empty seed. The live copy belongs at the root of
the `outpost/queue` branch, not under this directory and not on the coding branch.

Use `scripts/init_planning.py --repo OWNER/REPOSITORY` from the starter repository
after an initial commit exists. The helper reads the default branch, creates the
planning branch if missing, and creates the root manifest only if missing. It does
not overwrite an existing queue or change your local checkout.

See `../docs/queue.md` for schema, ordering, SHA conflicts, and completion behavior.
JSON has no comments; keep explanations in Markdown instead of invalidating the
runtime manifest with comments or extra pseudo-fields.
