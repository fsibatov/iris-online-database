#!/usr/bin/env python3
"""Lifecycle smoke tests for startup, sessions, signals and port reuse."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import signal
import socket
import stat
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from smoke_common import (
    RunningApp,
    free_port,
    json_request,
    require_binary,
    wait_health,
)


def open_session(app: RunningApp, session_id: str = "") -> str:
    status, payload = json_request(
        app.base_url,
        "/api/session/open",
        method="POST",
        payload={"id": session_id},
    )
    assert status == 200, payload
    return str(payload["id"])


def close_session(app: RunningApp, session_id: str) -> None:
    status, payload = json_request(
        app.base_url,
        "/api/session/close",
        method="POST",
        payload={"id": session_id},
    )
    assert status == 204, payload


def assert_port_closed(port: int) -> None:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.25)
        if sock.connect_ex(("127.0.0.1", port)) == 0:
            raise AssertionError(f"listen socket still open on {port}")


class OldVersionHealthHandler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:  # noqa: N802 - stdlib handler contract
        if self.path != "/api/health":
            self.send_error(404)
            return

        body = json.dumps(
            {
                "status": "ok",
                "application": "iris-online-database",
                "version": "0.9",
                "release": "IrisOnlineRelease/0.9",
            }
        ).encode("utf-8")

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args) -> None:
        return


def fake_browser_env() -> tuple[tempfile.TemporaryDirectory, dict[str, str]]:
    temp = tempfile.TemporaryDirectory(prefix="iris-browser-")
    temp_path = Path(temp.name)

    if os.name == "nt":
        # Iris Online opens the system browser on Windows through:
        #
        #   rundll32 url.dll,FileProtocolHandler <url>
        #
        # Put a harmless executable with the same filename first in PATH.
        # cmd.Start() succeeds, so normal browser-mode lifecycle remains active,
        # but no real browser window/tab is opened during the smoke test.
        system_root = Path(os.environ.get("SystemRoot", r"C:\Windows"))
        source = system_root / "System32" / "where.exe"

        if not source.is_file():
            raise RuntimeError(
                f"Windows browser stub source not found: {source}"
            )

        shutil.copy2(source, temp_path / "rundll32.exe")
    else:
        command = "open" if sys.platform == "darwin" else "xdg-open"
        script = temp_path / command
        script.write_text(
            "#!/bin/sh\nexit 0\n",
            encoding="utf-8",
        )
        script.chmod(script.stat().st_mode | stat.S_IXUSR)

    return temp, {
        "PATH": f"{temp.name}{os.pathsep}{os.environ.get('PATH', '')}",
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()

    binary = require_binary(args.binary)

    browser_dir, browser_env = fake_browser_env()

    try:
        # No browser session appears: startup timeout must terminate the app.
        with RunningApp(
            binary,
            ["-startup-timeout=250ms", "-idle-grace=100ms"],
            env=browser_env,
        ) as app:
            app.wait_ready()
            assert app.wait_exit(3) == 0, app.stderr()

        # A timely session cancels the startup timeout. Closing the confirmed
        # session then shuts the application down normally.
        with RunningApp(
            binary,
            ["-startup-timeout=1s", "-idle-grace=150ms"],
            env=browser_env,
        ) as app:
            app.wait_ready()

            session_id = open_session(app)

            time.sleep(1.2)

            assert (
                app.process.poll() is None
            ), "timely session did not cancel startup timer"

            close_session(app, session_id)

            assert app.wait_exit(4) == 0, app.stderr()

        # pagehide may race an in-flight session/open request. A close marked
        # as pending-open must prevent that late open from creating an orphan,
        # while an ordinary unknown close must remain harmless.
        with RunningApp(
            binary,
            ["-startup-timeout=1s", "-idle-grace=100ms"],
            env=browser_env,
        ) as app:
            app.wait_ready()

            pending_id = "pending-open-race-session-0001"
            status, payload = json_request(
                app.base_url,
                "/api/session/close",
                method="POST",
                payload={"id": pending_id, "pendingOpen": True},
            )
            assert status == 204, payload
            status, _payload = json_request(
                app.base_url,
                "/api/session/open",
                method="POST",
                payload={"id": pending_id},
            )
            assert status == 409, "late open after pending close was not rejected"

            ordinary_id = "ordinary-unknown-close-session-01"
            close_session(app, ordinary_id)
            reopened = open_session(app, ordinary_id)
            assert reopened == ordinary_id, "ordinary unknown close incorrectly blocked a later open"
            close_session(app, reopened)
            assert app.wait_exit(4) == 0, app.stderr()

        # A recently expired real browser session remains a bounded tombstone.
        # Its later explicit close must still be authoritative, while a random
        # unknown ID cannot stop the process.
        with RunningApp(
            binary,
            [
                "-startup-timeout=1s",
                "-heartbeat-timeout=200ms",
                "-idle-grace=100ms",
            ],
            env=browser_env,
        ) as app:
            app.wait_ready()

            session_id = open_session(
                app,
                "expired-close-session-0001",
            )

            time.sleep(0.55)

            assert (
                app.process.poll() is None
            ), "heartbeat expiry alone stopped normal browser mode"

            close_session(
                app,
                "random-unknown-session-0001",
            )

            time.sleep(0.3)

            assert (
                app.process.poll() is None
            ), "unknown session close stopped the backend"

            close_session(app, session_id)

            assert app.wait_exit(4) == 0, app.stderr()

        # Two-session ordering: closing an expired session cannot stop an active
        # second tab. Closing the last confirmed tab then stops cleanly.
        with RunningApp(
            binary,
            [
                "-startup-timeout=1s",
                "-heartbeat-timeout=200ms",
                "-idle-grace=100ms",
            ],
            env=browser_env,
        ) as app:
            app.wait_ready()

            first = open_session(
                app,
                "expired-A-session-0001",
            )
            second = open_session(
                app,
                "active-B-session-00001",
            )

            for _ in range(3):
                status, payload = json_request(
                    app.base_url,
                    "/api/session/heartbeat",
                    method="POST",
                    payload={"id": second},
                )
                assert status == 204, payload
                time.sleep(0.12)

            time.sleep(0.25)

            close_session(app, first)

            assert (
                app.process.poll() is None
            ), "expired A close stopped active B"

            close_session(app, second)

            assert app.wait_exit(4) == 0, app.stderr()

        # A browser may suspend a background tab longer than the heartbeat
        # lease. Normal browser mode must keep the backend alive so the tab can
        # reopen its session when it wakes up.
        with RunningApp(
            binary,
            [
                "-startup-timeout=1s",
                "-heartbeat-timeout=250ms",
                "-idle-grace=150ms",
            ],
            env=browser_env,
        ) as app:
            app.wait_ready()

            session_id = open_session(app)

            time.sleep(0.8)

            assert (
                app.process.poll() is None
            ), "suspended browser heartbeat stopped the backend"

            session_id = open_session(app, session_id)
            close_session(app, session_id)

            assert app.wait_exit(4) == 0, app.stderr()

        # -no-browser must not inherit normal browser startup timeout behavior.
        with RunningApp(
            binary,
            ["-no-browser", "-startup-timeout=200ms"],
        ) as app:
            app.wait_ready()

            time.sleep(0.45)

            assert (
                app.process.poll() is None
            ), "-no-browser incorrectly used startup timer"

        # Two explicit sessions in shutdown-when-idle mode. Closing one must
        # not stop the backend; closing the final session must stop it.
        with RunningApp(
            binary,
            [
                "-no-browser",
                "-shutdown-when-idle",
                "-idle-grace=150ms",
            ],
        ) as app:
            app.wait_ready()

            first = open_session(app)
            second = open_session(app)

            close_session(app, first)

            time.sleep(0.5)

            assert (
                app.process.poll() is None
            ), "closing one of two sessions stopped the app"

            close_session(app, second)
            close_session(app, second)  # idempotency

            assert app.wait_exit(4) == 0, app.stderr()

        # In shutdown-when-idle mode an abandoned session may expire and allow
        # normal idle shutdown.
        with RunningApp(
            binary,
            [
                "-no-browser",
                "-shutdown-when-idle",
                "-heartbeat-timeout=250ms",
                "-idle-grace=150ms",
            ],
        ) as app:
            app.wait_ready()
            open_session(app)

            assert app.wait_exit(4) == 0, app.stderr()

        # Python's subprocess.send_signal(SIGINT) is not supported for this
        # ordinary Windows process. Keep the signal-specific regression on
        # platforms where SIGINT has the required subprocess semantics.
        if os.name != "nt":
            with RunningApp(
                binary,
                ["-no-browser"],
            ) as app:
                app.wait_ready()

                app.process.send_signal(signal.SIGINT)

                assert app.wait_exit(5) == 0, app.stderr()

        # Graceful shutdown while requests are in flight. On Windows use the
        # application's real browser-session shutdown path instead of POSIX
        # SIGTERM semantics.
        port = free_port()

        shutdown_args = ["-no-browser"]

        if os.name == "nt":
            shutdown_args += [
                "-shutdown-when-idle",
                "-idle-grace=150ms",
            ]

        with RunningApp(
            binary,
            shutdown_args,
            port=port,
        ) as app:
            app.wait_ready()

            shutdown_session = None

            if os.name == "nt":
                shutdown_session = open_session(app)

            profile = {
                "schemaVersion": 1,
                "migrated": True,
                "settings": {
                    "server": "original",
                    "theme": "light",
                    "view": "cards",
                },
                "itemFilters": {
                    "q": "гнев предков",
                    "sort": "name",
                },
                "monsterFilters": {
                    "sort": "level",
                },
                "favorites": [
                    "item:80592",
                ],
                "history": [
                    "гнев предков",
                ],
            }

            status, _ = json_request(
                app.base_url,
                "/api/user-data",
                method="PUT",
                payload=profile,
            )
            assert status == 200

            request_error: list[Exception] = []

            def request_loop() -> None:
                try:
                    for _ in range(50):
                        urllib.request.urlopen(
                            app.base_url
                            + "/api/items?page=1&pageSize=48",
                            timeout=2,
                        ).read()
                except Exception as error:
                    request_error.append(error)

            worker = threading.Thread(
                target=request_loop,
            )
            worker.start()

            if os.name == "nt":
                assert shutdown_session is not None
                close_session(
                    app,
                    shutdown_session,
                )
            else:
                app.process.send_signal(
                    signal.SIGTERM,
                )

            assert app.wait_exit(5) == 0, app.stderr()

            worker.join(timeout=3)

            profile_files = list(
                app.root.glob(
                    "config/**/profile.json"
                )
            )

            assert (
                profile_files
            ), "profile was not persisted before shutdown"

            saved = json.loads(
                profile_files[0].read_text(
                    encoding="utf-8"
                )
            )

            assert (
                saved["settings"]["server"] == "original"
            )

        # Listener must be reusable immediately after shutdown.
        with RunningApp(
            binary,
            ["-no-browser"],
            port=port,
        ) as second_run:
            second_run.wait_ready()

            assert (
                second_run.process.poll() is None
            ), "port was not reusable after shutdown"

        assert_port_closed(port)

        # A second launch of the exact same build must reuse the running
        # backend, not start a competing writer.
        same_port = free_port()

        with RunningApp(
            binary,
            ["-no-browser"],
            port=same_port,
        ) as first_run:
            first_run.wait_ready()

            with RunningApp(
                binary,
                ["-no-browser"],
                port=same_port,
            ) as duplicate_run:
                assert (
                    duplicate_run.wait_exit(4) == 0
                ), duplicate_run.stderr()

            assert (
                first_run.process.poll() is None
            ), "same-build probe stopped the active instance"

        assert_port_closed(same_port)

        # Single-instance is an application invariant, not merely a port
        # invariant. Two different loopback ports must not create competing
        # profile/log writers.
        with tempfile.TemporaryDirectory(
            prefix="iris-shared-instance-"
        ) as shared_root:
            shared_env = os.environ.copy()

            shared_env.update({
                "HOME": str(Path(shared_root) / "home"),
                "XDG_CONFIG_HOME": str(Path(shared_root) / "config"),
                "XDG_CACHE_HOME": str(Path(shared_root) / "cache"),
                "APPDATA": str(Path(shared_root) / "config"),
                "LOCALAPPDATA": str(Path(shared_root) / "cache"),
            })

            first_port = free_port()
            second_port = free_port()

            first_process = subprocess.Popen(
                [
                    str(binary),
                    f"-addr=127.0.0.1:{first_port}",
                    "-no-browser",
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                encoding="utf-8",
                errors="strict",
                env=shared_env,
            )

            try:
                wait_health(
                    f"http://127.0.0.1:{first_port}"
                )

                second_process = subprocess.Popen(
                    [
                        str(binary),
                        f"-addr=127.0.0.1:{second_port}",
                        "-no-browser",
                    ],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    encoding="utf-8",
                    errors="strict",
                    env=shared_env,
                )

                try:
                    assert (
                        second_process.wait(timeout=4) == 1
                    ), "different-port duplicate did not fail"

                    second_error = (
                        second_process.stderr.read()
                        if second_process.stderr
                        else ""
                    )

                    assert (
                        "Iris Online уже запущена"
                        in second_error
                    ), second_error

                    assert (
                        first_process.poll() is None
                    ), (
                        "different-port duplicate "
                        "stopped active instance"
                    )

                    assert_port_closed(second_port)

                finally:
                    if second_process.poll() is None:
                        second_process.kill()
                        second_process.wait(
                            timeout=2
                        )

            finally:
                if first_process.poll() is None:
                    # This is cleanup for the duplicate-instance test,
                    # not a graceful-shutdown assertion.
                    if os.name == "nt":
                        first_process.terminate()
                    else:
                        first_process.send_signal(
                            signal.SIGTERM
                        )

                    first_process.wait(
                        timeout=5
                    )

            assert_port_closed(first_port)
            assert_port_closed(second_port)

        # A different Iris Online version already listening on the requested
        # port must be reported and must not cause profile/log maintenance or a
        # second listener.
        old_port = free_port()

        old_server = ThreadingHTTPServer(
            ("127.0.0.1", old_port),
            OldVersionHealthHandler,
        )

        old_thread = threading.Thread(
            target=old_server.serve_forever,
            daemon=True,
        )
        old_thread.start()

        try:
            with RunningApp(
                binary,
                ["-no-browser"],
                port=old_port,
            ) as mismatched:
                assert (
                    mismatched.wait_exit(4) == 1
                ), (
                    "different-version probe "
                    "did not fail visibly"
                )

                stderr = mismatched.stderr()

                assert (
                    "Закройте уже запущенную Iris Online 0.9"
                    in stderr
                ), stderr

                assert not list(
                    mismatched.root.glob(
                        "**/profile.json"
                    )
                ), "mismatch touched profile"

                assert not list(
                    mismatched.root.glob(
                        "**/application.log"
                    )
                ), "mismatch touched logs"

        finally:
            old_server.shutdown()
            old_server.server_close()
            old_thread.join(timeout=2)

        assert_port_closed(old_port)

        # Ten bounded start/stop cycles catch leaked listeners/processes
        # without a long soak test.
        cycle_port = free_port()

        for _ in range(10):
            if os.name == "nt":
                with RunningApp(
                    binary,
                    [
                        "-no-browser",
                        "-shutdown-when-idle",
                        "-idle-grace=100ms",
                    ],
                    port=cycle_port,
                ) as cycle:
                    cycle.wait_ready()

                    cycle_session = open_session(
                        cycle
                    )

                    close_session(
                        cycle,
                        cycle_session,
                    )

                    assert (
                        cycle.wait_exit(4) == 0
                    ), cycle.stderr()

            else:
                with RunningApp(
                    binary,
                    ["-no-browser"],
                    port=cycle_port,
                ) as cycle:
                    cycle.wait_ready()

                    assert (
                        cycle.stop(4) == 0
                    ), cycle.stderr()

            assert_port_closed(cycle_port)

    finally:
        browser_dir.cleanup()

    print("Lifecycle smoke test: PASS")


if __name__ == "__main__":
    main()