#!/usr/bin/env python3
"""Verify PE architecture and embedded Windows icon/manifest resources."""

from __future__ import annotations

import argparse
import hashlib
import pathlib
import struct

MACHINES = {"x64": 0x8664, "x86": 0x014C, "arm64": 0xAA64}
EXPECTED_TYPES = {3, 14, 24}


def parse_pe(
    path: pathlib.Path,
) -> tuple[int, dict[str, tuple[int, int, int, int]], bytes]:
    data = path.read_bytes()
    if len(data) < 0x40 or data[:2] != b"MZ":
        raise ValueError(f"{path.name}: not a PE file")
    pe = struct.unpack_from("<I", data, 0x3C)[0]
    if pe + 24 > len(data) or data[pe : pe + 4] != b"PE\0\0":
        raise ValueError(f"{path.name}: invalid PE signature")
    machine, sections = struct.unpack_from("<HH", data, pe + 4)
    optional_size = struct.unpack_from("<H", data, pe + 20)[0]
    section_off = pe + 24 + optional_size
    result: dict[str, tuple[int, int, int, int]] = {}
    for index in range(sections):
        off = section_off + index * 40
        if off + 40 > len(data):
            raise ValueError(f"{path.name}: truncated section table")
        name = data[off : off + 8].split(b"\0", 1)[0].decode("ascii", "replace")
        virtual_size, virtual_address, raw_size, raw_offset = struct.unpack_from(
            "<IIII", data, off + 8
        )
        result[name] = (virtual_size, virtual_address, raw_size, raw_offset)
    return machine, result, data


def resource_type_ids(path: pathlib.Path) -> set[int]:
    _machine, sections, data = parse_pe(path)
    if ".rsrc" not in sections:
        return set()
    _virtual_size, _virtual_address, raw_size, raw_offset = sections[".rsrc"]
    if raw_offset + min(raw_size, 16) > len(data):
        raise ValueError(f"{path.name}: truncated .rsrc")
    named, ids = struct.unpack_from("<HH", data, raw_offset + 12)
    count = named + ids
    if raw_offset + 16 + count * 8 > len(data):
        raise ValueError(f"{path.name}: truncated resource directory")
    types: set[int] = set()
    for index in range(count):
        name_or_id, _target = struct.unpack_from(
            "<II", data, raw_offset + 16 + index * 8
        )
        if name_or_id & 0x80000000 == 0:
            types.add(name_or_id)
    return types


def resource_payloads(path: pathlib.Path, wanted_type: int) -> list[bytes]:
    _machine, sections, data = parse_pe(path)
    _virtual_size, virtual_address, raw_size, raw_offset = sections[".rsrc"]
    section_end = raw_offset + raw_size

    def directory_entries(relative: int) -> list[tuple[int, int]]:
        base = raw_offset + relative
        if base < raw_offset or base + 16 > section_end:
            raise ValueError(f"{path.name}: invalid resource directory offset")
        named, ids = struct.unpack_from("<HH", data, base + 12)
        count = named + ids
        if base + 16 + count * 8 > section_end:
            raise ValueError(f"{path.name}: truncated resource directory entries")
        return [
            struct.unpack_from("<II", data, base + 16 + i * 8) for i in range(count)
        ]

    type_target = None
    for name_or_id, target in directory_entries(0):
        if name_or_id == wanted_type:
            type_target = target
            break
    if type_target is None or type_target & 0x80000000 == 0:
        return []

    payloads: list[bytes] = []
    type_dir = type_target & 0x7FFFFFFF
    for _resource_id, id_target in directory_entries(type_dir):
        if id_target & 0x80000000 == 0:
            raise ValueError(f"{path.name}: malformed resource id directory")
        id_dir = id_target & 0x7FFFFFFF
        for _language, data_target in directory_entries(id_dir):
            if data_target & 0x80000000:
                raise ValueError(f"{path.name}: unexpected nested language directory")
            entry = raw_offset + data_target
            if entry < raw_offset or entry + 16 > section_end:
                raise ValueError(f"{path.name}: invalid resource data entry")
            rva, size, _codepage, _reserved = struct.unpack_from("<IIII", data, entry)
            start = raw_offset + (rva - virtual_address)
            end = start + size
            if rva < virtual_address or start < raw_offset or end > section_end:
                raise ValueError(f"{path.name}: invalid resource payload RVA")
            payloads.append(data[start:end])
    return payloads


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", required=True, type=pathlib.Path)
    parser.add_argument("--version", default="1.1.0")
    parser.add_argument("--go-version", required=True)
    parser.add_argument("--diagnostic", action="store_true")
    args = parser.parse_args()
    suffix = f"-diagnostic-{args.go_version}" if args.diagnostic else ""
    icon_fingerprints: dict[str, tuple[list[str], str]] = {}
    for label, expected_machine in MACHINES.items():
        path = (
            args.directory / f"IrisOnlineDB-{args.version}{suffix}-Windows-{label}.exe"
        )
        machine, sections, _data = parse_pe(path)
        if machine != expected_machine:
            raise SystemExit(
                f"{path.name}: machine=0x{machine:04x}, expected 0x{expected_machine:04x}"
            )
        if ".rsrc" not in sections or sections[".rsrc"][2] == 0:
            raise SystemExit(f"{path.name}: missing non-empty .rsrc section")
        types = resource_type_ids(path)
        missing = EXPECTED_TYPES - types
        if missing:
            raise SystemExit(
                f"{path.name}: missing Windows resource types {sorted(missing)}"
            )
        icons = [
            hashlib.sha256(payload).hexdigest()
            for payload in resource_payloads(path, 3)
        ]
        groups = resource_payloads(path, 14)
        manifests = resource_payloads(path, 24)
        if not icons or len(groups) != 1 or len(manifests) != 1:
            raise SystemExit(f"{path.name}: incomplete icon/manifest resource payloads")
        icon_fingerprints[label] = (icons, hashlib.sha256(groups[0]).hexdigest())
        print(
            f"{path.name}: resource PASS (.rsrc, {len(icons)} icon images, group icon, manifest)"
        )
    values = list(icon_fingerprints.values())
    if any(value != values[0] for value in values[1:]):
        raise SystemExit("Windows icon resources differ between architectures")
    print("Windows icon payloads are identical across x64/x86/ARM64")


if __name__ == "__main__":
    main()
