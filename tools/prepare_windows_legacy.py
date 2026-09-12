from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess  # nosec B404
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
COMPAT = ROOT / "tools" / "compat"


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def compatibility_config() -> dict:
    config = json.loads((COMPAT / "windows7.json").read_text(encoding="utf-8"))
    if (
        config["schema"] != 1
        or config["go_version"]
        != (ROOT / ".go-version").read_text(encoding="ascii").strip()
    ):
        raise ValueError("Windows 7 compatibility patch does not match pinned Go")
    patch = COMPAT / config["patch"]
    if patch.parent != COMPAT or digest(patch) != config["patch_sha256"]:
        raise ValueError("Windows 7 compatibility patch checksum mismatch")
    return config


def compatibility_marker(config: dict | None = None) -> str:
    config = config or compatibility_config()
    return f"IrisWindowsLegacy/go{config['go_version']}/{config['patch_sha256']}"


def verified_sources(goroot: Path, config: dict) -> dict[str, Path]:
    sources = {}
    for relative, hashes in config["files"].items():
        path = goroot / relative
        if not path.resolve().is_relative_to(goroot.resolve() / "src"):
            raise ValueError("Compatibility source path escapes Go source directory")
        if digest(path) != hashes["original_sha256"]:
            raise ValueError(f"Official Go source checksum mismatch: {relative}")
        sources[relative] = path
    return sources


def validate_overlay(directory: Path, sources: dict[str, Path], config: dict) -> Path:
    overlay = directory / "overlay.json"
    expected = {
        str(source.resolve()): str((directory / relative).resolve())
        for relative, source in sources.items()
    }
    document = json.loads(overlay.read_text(encoding="utf-8"))
    if document != {"Replace": expected}:
        raise ValueError("Compatibility overlay contains unexpected replacements")
    for relative, hashes in config["files"].items():
        if digest(directory / relative) != hashes["patched_sha256"]:
            raise ValueError(f"Compatibility source checksum mismatch: {relative}")
    return overlay


def prepare_overlay(go: str, directory: Path) -> Path:
    config = compatibility_config()
    environment = os.environ.copy()
    environment["GOTOOLCHAIN"] = "local"
    environment.pop("GOFLAGS", None)
    info = subprocess.run(  # nosec B603
        [go, "env", "-json", "GOVERSION", "GOROOT"],
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
        timeout=30,
        env=environment,
    )
    values = json.loads(info.stdout)
    if values["GOVERSION"] != "go" + config["go_version"]:
        raise ValueError("Compatibility build requires the pinned official Go version")
    goroot = Path(values["GOROOT"]).resolve()
    sources = verified_sources(goroot, config)
    directory = directory.resolve()
    if directory.is_relative_to(ROOT) or directory.is_relative_to(goroot):
        raise ValueError("Compatibility output must be outside project and official Go")
    if directory.exists():
        return validate_overlay(directory, sources, config)
    git = shutil.which("git")
    if not git:
        raise ValueError("Git executable is unavailable")
    directory.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(
        prefix="iris-windows7-", dir=directory.parent
    ) as temporary:
        staging = Path(temporary) / "overlay"
        for relative, source in sources.items():
            target = staging / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, target)
        patch_environment = {
            name: value
            for name, value in environment.items()
            if not name.startswith("GIT_")
        }
        patch_environment.update(
            GIT_CONFIG_NOSYSTEM="1",
            GIT_CONFIG_GLOBAL=os.devnull,
            GIT_ATTR_NOSYSTEM="1",
            GIT_CEILING_DIRECTORIES=str(staging.parent),
        )
        patch = str(COMPAT / config["patch"])
        for arguments in (("--check", patch), (patch,)):
            subprocess.run(  # nosec B603
                [
                    git,
                    "-c",
                    "core.autocrlf=false",
                    "-c",
                    "core.eol=lf",
                    "-c",
                    f"core.attributesFile={os.devnull}",
                    "apply",
                    "--whitespace=nowarn",
                    *arguments,
                ],
                cwd=staging,
                env=patch_environment,
                check=True,
                capture_output=True,
                timeout=30,
            )
        for relative, hashes in config["files"].items():
            if digest(staging / relative) != hashes["patched_sha256"]:
                raise ValueError(f"Applied patch checksum mismatch: {relative}")
        replacements = {
            str(source.resolve()): str(directory / relative)
            for relative, source in sources.items()
        }
        (staging / "overlay.json").write_text(
            json.dumps({"Replace": replacements}, indent=2) + "\n", encoding="utf-8"
        )
        staging.rename(directory)
    return validate_overlay(directory, sources, config)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--go", default="go")
    parser.add_argument("--directory", type=Path, required=True)
    args = parser.parse_args()
    go = shutil.which(args.go)
    if not go:
        parser.error("Go executable is unavailable")
    try:
        prepare_overlay(go, args.directory)
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit(
            f"Windows 7 compatibility preparation: FAIL: {error}"
        ) from error
    print("Windows 7 compatibility overlay: PASS")


if __name__ == "__main__":
    main()
