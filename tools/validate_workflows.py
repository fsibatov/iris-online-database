"""Validate GitHub workflow YAML before a release reaches GitHub."""

from __future__ import annotations

import re
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
ACTION_PIN = re.compile(r"^\s*uses:\s*[^@\s]+@([0-9a-f]{40})(?:\s|$)", re.MULTILINE)
ACTION_REFERENCE = re.compile(r"^\s*uses:\s*[^@\s]+@([^\s#]+)", re.MULTILINE)


def main() -> int:
    failures = 0
    workflows = sorted((ROOT / ".github" / "workflows").glob("*.yml"))
    if not workflows:
        print("Workflow validation: FAIL [WF001] count=1")
        return 1

    for path in workflows:
        text = path.read_text(encoding="utf-8")
        try:
            document = yaml.safe_load(text)
        except yaml.YAMLError:
            failures += 1
            continue
        if not isinstance(document, dict) or not isinstance(document.get("jobs"), dict):
            failures += 1
            continue
        references = ACTION_REFERENCE.findall(text)
        pins = ACTION_PIN.findall(text)
        if len(references) != len(pins):
            failures += 1

    if failures:
        print(f"Workflow validation: FAIL [WF001] count={failures}")
        return 1
    print(f"Workflow validation: PASS files={len(workflows)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
