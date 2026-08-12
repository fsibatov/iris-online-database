from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path


def active_lines(path: Path):
    for raw in path.read_bytes().splitlines():
        line = raw.decode("latin1").strip()
        if not line or line.startswith("//"):
            continue
        if "//" in line:
            line = line.split("//", 1)[0].strip()
        if line:
            yield line


def monster_ids(directory: Path) -> list[int]:
    result: set[int] = set()
    for path in sorted(directory.glob("*.txt"), key=lambda p: p.name.lower()):
        lines = list(active_lines(path))
        i = 0
        while i < len(lines):
            try:
                int(lines[i].split()[0])
            except (ValueError, IndexError):
                i += 1
                continue
            i += 1
            if i >= len(lines) or lines[i] != "{":
                continue
            i += 1
            if i >= len(lines):
                break
            i += 1
            while i < len(lines) and lines[i] != "}":
                parts = lines[i].split()
                if parts:
                    try:
                        result.add(int(parts[0]))
                    except ValueError:
                        pass
                i += 1
            if i < len(lines):
                i += 1
    return sorted(result)


def build(original: Path, kiss: Path):
    return {
        "schemaVersion": 1,
        "servers": {
            "original": monster_ids(original),
            "kiss": monster_ids(kiss),
        },
    }


def write_gzip_json(path: Path, data) -> None:
    payload = json.dumps(
        data, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    with (
        path.open("wb") as raw,
        gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz,
    ):
        gz.write(payload)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Build server-specific monster visibility from MonsterRegen directories."
    )
    parser.add_argument("--original-dir", required=True, type=Path)
    parser.add_argument("--kiss-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    data = build(args.original_dir, args.kiss_dir)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    write_gzip_json(args.output, data)
    print(
        f"original={len(data['servers']['original'])} kiss={len(data['servers']['kiss'])} output={args.output}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
