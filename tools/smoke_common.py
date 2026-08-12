"""Shared helpers for reproducible Iris Online smoke tests."""

from __future__ import annotations

import contextlib
import json
import os
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Self
from urllib.parse import urlsplit


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def open_loopback_url(
    target: str | urllib.request.Request,
    *,
    timeout: float,
):
    """Open an HTTP request only to the local smoke-test server."""
    raw_url = (
        target.full_url if isinstance(target, urllib.request.Request) else str(target)
    )
    parsed = urlsplit(raw_url)
    if parsed.scheme != "http" or parsed.hostname not in {
        "127.0.0.1",
        "localhost",
        "::1",
    }:
        raise ValueError(
            f"smoke request must use http and target loopback only: {raw_url!r}"
        )
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("smoke request URL must not contain userinfo")
    # B310 is suppressed only at this single sink because scheme and host are
    # explicitly allowlisted immediately above.
    return urllib.request.urlopen(target, timeout=timeout)  # nosec B310


def json_request(
    base_url: str,
    path: str,
    *,
    method: str = "GET",
    payload: Any | None = None,
    timeout: float = 5.0,
) -> tuple[int, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(
        base_url + path, data=data, headers=headers, method=method
    )
    try:
        with open_loopback_url(request, timeout=timeout) as response:
            raw = response.read()
            parsed = json.loads(raw.decode("utf-8")) if raw else None
            return response.status, parsed
    except urllib.error.HTTPError as error:
        raw = error.read()
        try:
            parsed = json.loads(raw.decode("utf-8")) if raw else None
        except (UnicodeDecodeError, json.JSONDecodeError):
            parsed = raw.decode("utf-8", errors="replace")
        return error.code, parsed


def wait_health(base_url: str, timeout: float = 10.0) -> None:
    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            status, payload = json_request(base_url, "/api/health", timeout=1.0)
            if status == 200 and payload and payload.get("status") == "ok":
                return
        except (
            OSError,
            TimeoutError,
            ValueError,
        ) as error:
            last_error = error
        time.sleep(0.05)
    raise RuntimeError(f"health endpoint did not become ready: {last_error}")


@dataclass
class RunningApp:
    binary: Path
    args: list[str]
    env: dict[str, str] | None = None
    port: int | None = None

    def __post_init__(self) -> None:
        self.port = self.port or free_port()
        self.base_url = f"http://127.0.0.1:{self.port}"
        self.temp = tempfile.TemporaryDirectory(prefix="iris-smoke-")
        root = Path(self.temp.name)
        process_env = os.environ.copy()
        process_env.update(
            {
                "HOME": str(root / "home"),
                "XDG_CONFIG_HOME": str(root / "config"),
                "XDG_CACHE_HOME": str(root / "cache"),
                "APPDATA": str(root / "config"),
                "LOCALAPPDATA": str(root / "cache"),
            }
        )
        if self.env:
            process_env.update(self.env)
        command = [str(self.binary), f"-addr=127.0.0.1:{self.port}", *self.args]
        self.process = subprocess.Popen(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            errors="strict",
            env=process_env,
        )

    @property
    def root(self) -> Path:
        return Path(self.temp.name)

    def wait_ready(self, timeout: float = 10.0) -> None:
        wait_health(self.base_url, timeout)

    def wait_exit(self, timeout: float = 10.0) -> int:
        try:
            return self.process.wait(timeout=timeout)
        except subprocess.TimeoutExpired as error:
            raise RuntimeError("application did not exit in time") from error

    def stop(self, timeout: float = 10.0) -> int:
        if self.process.poll() is None:
            self.process.send_signal(signal.SIGTERM)
        try:
            return self.process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            self.process.kill()
            return self.process.wait(timeout=2)

    def stderr(self) -> str:
        return self.process.stderr.read() if self.process.stderr else ""

    def close(self) -> None:
        with contextlib.suppress(Exception):
            self.stop()
        self.temp.cleanup()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()


def require_binary(path: str) -> Path:
    binary = Path(path).resolve()
    if not binary.is_file():
        raise SystemExit(f"binary not found: {binary}")
    return binary
