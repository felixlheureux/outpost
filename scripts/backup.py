#!/usr/bin/env python3
"""Create an idle, private Outpost backup with a consistent SQLite snapshot."""

import argparse
import json
import os
import pathlib
import shutil
import sqlite3
import stat
import subprocess
import sys
import tempfile
from datetime import datetime, timezone


def require_idle(database: pathlib.Path) -> None:
    active = subprocess.run(
        ["systemctl", "--user", "is-active", "outpost.service"],
        capture_output=True,
        text=True,
        check=False,
    )
    if active.returncode == 0 or active.stdout.strip() not in {"inactive", "failed", "unknown"}:
        raise RuntimeError("stop outpost.service and confirm the user systemd bus before backup")
    units = subprocess.run(
        ["systemctl", "--user", "list-units", "--type=service", "--state=active", "--plain", "--no-legend", "outpost-run_*.service"],
        capture_output=True,
        text=True,
        check=True,
    )
    if units.stdout.strip():
        raise RuntimeError("an Outpost worker unit is active; settle it before backup")
    with sqlite3.connect(f"file:{database}?mode=ro", uri=True) as source:
        if source.execute("SELECT count(*) FROM runs WHERE reservation_state='reserved'").fetchone()[0]:
            raise RuntimeError("an execution reservation remains; reconcile it before backup")


def copy_entry(source: pathlib.Path, target: pathlib.Path) -> None:
    mode = source.lstat().st_mode
    if stat.S_ISDIR(mode):
        shutil.copytree(source, target, symlinks=True)
    elif stat.S_ISREG(mode):
        shutil.copy2(source, target, follow_symlinks=False)
    elif stat.S_ISLNK(mode):
        target.symlink_to(os.readlink(source))
    else:
        raise RuntimeError(f"unsupported state entry: {source}")


def main() -> None:
    home = pathlib.Path.home()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--state-dir", type=pathlib.Path, default=home / ".local/state/outpost")
    parser.add_argument("--config", type=pathlib.Path, default=home / ".config/outpost/config.json")
    parser.add_argument("--workspaces-dir", type=pathlib.Path, default=home / ".local/share/outpost/workspaces")
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    for path in (args.state_dir, args.config, args.workspaces_dir, args.output):
        if not path.is_absolute():
            parser.error("all paths must be absolute")
    if args.output.exists() or args.output.is_symlink():
        parser.error("backup output already exists")
    if not args.output.parent.is_dir():
        parser.error("backup parent directory does not exist")
    for source in (args.state_dir, args.config, args.workspaces_dir):
        if source.is_symlink() or not source.exists():
            parser.error(f"source is missing or a symlink: {source}")
        if args.output.resolve().is_relative_to(source.resolve()):
            parser.error("backup output must be outside the copied source directories")
    database = args.state_dir / "outpost.sqlite"
    if not database.is_file() or database.is_symlink():
        parser.error("private Outpost database is missing")
    require_idle(database)
    staging = pathlib.Path(tempfile.mkdtemp(prefix=".outpost-backup-", dir=args.output.parent))
    try:
        os.chmod(staging, 0o700)
        state = staging / "state"
        state.mkdir(mode=0o700)
        with sqlite3.connect(f"file:{database}?mode=ro", uri=True) as source:
            with sqlite3.connect(state / "outpost.sqlite") as snapshot:
                source.backup(snapshot)
        os.chmod(state / "outpost.sqlite", 0o600)
        for entry in args.state_dir.iterdir():
            if entry.name in {"outpost.sqlite", "outpost.sqlite-wal", "outpost.sqlite-shm", "backups", "locks"}:
                continue
            copy_entry(entry, state / entry.name)
        shutil.copy2(args.config, staging / "config.json")
        os.chmod(staging / "config.json", 0o600)
        copy_entry(args.workspaces_dir, staging / "workspaces")
        with sqlite3.connect(f"file:{state / 'outpost.sqlite'}?mode=ro", uri=True) as check:
            if check.execute("PRAGMA integrity_check").fetchone()[0] != "ok":
                raise RuntimeError("backup SQLite integrity check failed")
            schema = check.execute("SELECT version_id FROM goose_db_version WHERE is_applied=1 ORDER BY id DESC LIMIT 1").fetchone()[0]
        manifest = {
            "created_at": datetime.now(timezone.utc).isoformat(),
            "schema_version": schema,
            "state_dir": str(args.state_dir),
            "config": str(args.config),
            "workspaces_dir": str(args.workspaces_dir),
        }
        (staging / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
        os.chmod(staging / "manifest.json", 0o600)
        staging.rename(args.output)
    except BaseException:
        shutil.rmtree(staging)
        raise
    print(f"Backup ready: {args.output} (schema {schema})")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, sqlite3.Error, subprocess.CalledProcessError) as error:
        print(f"Backup stopped: {error}", file=sys.stderr)
        raise SystemExit(1)
