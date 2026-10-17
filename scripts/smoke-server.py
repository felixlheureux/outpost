#!/usr/bin/env python3
"""Serve an already-built binary against disposable state; never touch user state."""
import pathlib
import signal
import subprocess
import tempfile

binary = pathlib.Path(__file__).resolve().parents[1] / "bin/outpost"
with tempfile.TemporaryDirectory(prefix="outpost-smoke-") as directory:
    arguments = [str(binary), "--state-dir", directory]
    subprocess.run([*arguments, "migrate"], check=True)
    process = subprocess.Popen([*arguments, "serve", "--dev", "--listen", "127.0.0.1:17331"])

    def stop(_signum, _frame):
        process.terminate()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        raise SystemExit(process.wait())
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
