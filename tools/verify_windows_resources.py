"""Verify PE architecture, hardening flags and embedded Windows resources."""

from __future__ import annotations

import argparse
import pathlib
import struct

from release_targets import RELEASE_TARGETS

WINDOWS_GUI = 2
EXPECTED_RESOURCE_TYPES = {3, 14, 16, 24}
REQUIRED_DLL_CHARACTERISTICS = {
    0x0040: "ASLR",
    0x0100: "NX/DEP",
    0x8000: "Terminal Server aware",
}


def parse_pe(
    path: pathlib.Path,
) -> tuple[int, dict[str, tuple[int, int, int, int]], bytes, int, int, int]:
    data = path.read_bytes()
    if len(data) < 0x40 or data[:2] != b"MZ":
        raise ValueError("not a PE file")
    pe = struct.unpack_from("<I", data, 0x3C)[0]
    if pe + 24 > len(data) or data[pe : pe + 4] != b"PE\0\0":
        raise ValueError("invalid PE signature")
    machine, sections = struct.unpack_from("<HH", data, pe + 4)
    timestamp = struct.unpack_from("<I", data, pe + 8)[0]
    optional_size = struct.unpack_from("<H", data, pe + 20)[0]
    optional = pe + 24
    if optional + optional_size > len(data):
        raise ValueError("truncated optional header")
    magic = struct.unpack_from("<H", data, optional)[0]
    subsystem = struct.unpack_from("<H", data, optional + 68)[0]
    dll_characteristics = struct.unpack_from("<H", data, optional + 70)[0]
    section_off = optional + optional_size
    result: dict[str, tuple[int, int, int, int]] = {}
    for index in range(sections):
        off = section_off + index * 40
        if off + 40 > len(data):
            raise ValueError("truncated section table")
        name = data[off : off + 8].split(b"\0", 1)[0].decode("ascii", "replace")
        virtual_size, virtual_address, raw_size, raw_offset = struct.unpack_from(
            "<IIII", data, off + 8
        )
        result[name] = (virtual_size, virtual_address, raw_size, raw_offset)
    return (
        machine,
        result,
        data,
        timestamp,
        magic,
        subsystem | (dll_characteristics << 16),
    )


def directory_entries(
    data: bytes, raw_offset: int, section_end: int, relative: int
) -> list[tuple[int, int]]:
    base = raw_offset + relative
    if base < raw_offset or base + 16 > section_end:
        raise ValueError("invalid resource directory offset")
    named, ids = struct.unpack_from("<HH", data, base + 12)
    count = named + ids
    if base + 16 + count * 8 > section_end:
        raise ValueError("truncated resource directory")
    return [
        struct.unpack_from("<II", data, base + 16 + index * 8) for index in range(count)
    ]


def resource_payloads(path: pathlib.Path, wanted_type: int) -> list[bytes]:
    _machine, sections, data, _timestamp, _magic, _flags = parse_pe(path)
    if ".rsrc" not in sections:
        return []
    _virtual_size, virtual_address, raw_size, raw_offset = sections[".rsrc"]
    section_end = raw_offset + raw_size
    if section_end > len(data):
        raise ValueError("truncated resource section")
    type_target = next(
        (
            target
            for name_or_id, target in directory_entries(
                data, raw_offset, section_end, 0
            )
            if name_or_id == wanted_type
        ),
        None,
    )
    if type_target is None or type_target & 0x80000000 == 0:
        return []
    payloads: list[bytes] = []
    for _resource_id, id_target in directory_entries(
        data, raw_offset, section_end, type_target & 0x7FFFFFFF
    ):
        if id_target & 0x80000000 == 0:
            raise ValueError("malformed resource id directory")
        for _language, data_target in directory_entries(
            data, raw_offset, section_end, id_target & 0x7FFFFFFF
        ):
            if data_target & 0x80000000:
                raise ValueError("unexpected nested resource directory")
            entry = raw_offset + data_target
            if entry < raw_offset or entry + 16 > section_end:
                raise ValueError("invalid resource data entry")
            rva, size, _codepage, _reserved = struct.unpack_from("<IIII", data, entry)
            start = raw_offset + (rva - virtual_address)
            end = start + size
            if rva < virtual_address or start < raw_offset or end > section_end:
                raise ValueError("invalid resource payload RVA")
            payloads.append(data[start:end])
    return payloads


def resource_type_ids(path: pathlib.Path) -> set[int]:
    _machine, sections, data, _timestamp, _magic, _flags = parse_pe(path)
    if ".rsrc" not in sections:
        return set()
    _virtual_size, _virtual_address, raw_size, raw_offset = sections[".rsrc"]
    section_end = raw_offset + raw_size
    return {
        name_or_id
        for name_or_id, _target in directory_entries(data, raw_offset, section_end, 0)
        if name_or_id & 0x80000000 == 0
    }


def decode_manifest(payload: bytes) -> str:
    for encoding in ("utf-8-sig", "utf-16"):
        try:
            return payload.decode(encoding)
        except UnicodeDecodeError:
            continue
    raise ValueError("manifest is not valid Unicode")


def version_tuple(version: str) -> tuple[int, int, int, int]:
    parts = version.split(".")
    if len(parts) not in (3, 4) or any(not part.isdigit() for part in parts):
        raise ValueError(
            "release version must contain three or four numeric components"
        )
    values = [int(part) for part in parts]
    if any(value > 0xFFFF for value in values):
        raise ValueError("release version component exceeds Windows resource range")
    while len(values) < 4:
        values.append(0)
    return values[0], values[1], values[2], values[3]


def verify_version_resource(payloads: list[bytes], version: str, goarch: str) -> None:
    if len(payloads) != 1:
        raise SystemExit(f"Windows version resource count is invalid ({goarch})")
    payload = payloads[0]
    required = (
        "Iris Online Database".encode("utf-16le"),
        "FileVersion".encode("utf-16le"),
        "ProductVersion".encode("utf-16le"),
        version.encode("utf-16le"),
    )
    if any(marker not in payload for marker in required):
        raise SystemExit(f"Windows version metadata is incomplete ({goarch})")

    signature = struct.pack("<I", 0xFEEF04BD)
    offsets = []
    start = 0
    while True:
        offset = payload.find(signature, start)
        if offset < 0:
            break
        offsets.append(offset)
        start = offset + 1
    if len(offsets) != 1 or offsets[0] + 52 > len(payload):
        raise SystemExit(f"VS_FIXEDFILEINFO is missing or ambiguous ({goarch})")

    fields = struct.unpack_from("<13I", payload, offsets[0])
    expected = version_tuple(version)
    expected_ms = (expected[0] << 16) | expected[1]
    expected_ls = (expected[2] << 16) | expected[3]
    if fields[2:6] != (expected_ms, expected_ls, expected_ms, expected_ls):
        raise SystemExit(f"Windows fixed version metadata is not {version} ({goarch})")


def verify_target(path: pathlib.Path, version: str, target) -> None:
    machine, sections, _data, timestamp, magic, packed_flags = parse_pe(path)
    subsystem = packed_flags & 0xFFFF
    dll_characteristics = packed_flags >> 16
    if (
        machine != target.pe_machine
        or magic != target.pe_magic
        or subsystem != WINDOWS_GUI
    ):
        raise SystemExit(
            f"release executable has an unexpected PE target ({target.goarch})"
        )
    if timestamp != 0:
        raise SystemExit(
            f"COFF timestamp prevents deterministic release output ({target.goarch})"
        )
    if ".rsrc" not in sections or sections[".rsrc"][2] == 0:
        raise SystemExit(f"Windows resource section is missing ({target.goarch})")

    required_flags = dict(REQUIRED_DLL_CHARACTERISTICS)
    if target.require_high_entropy_va:
        required_flags[0x0020] = "High Entropy VA"
    missing_flags = [
        label
        for flag, label in required_flags.items()
        if dll_characteristics & flag == 0
    ]
    if missing_flags:
        raise SystemExit(
            f"required PE hardening flags are missing ({target.goarch}): "
            + ",".join(missing_flags)
        )
    missing_types = EXPECTED_RESOURCE_TYPES - resource_type_ids(path)
    if missing_types:
        raise SystemExit(
            f"required Windows resource types are missing ({target.goarch})"
        )
    if len(resource_payloads(path, 14)) != 1:
        raise SystemExit(f"application icon group is missing ({target.goarch})")
    manifests = resource_payloads(path, 24)
    if len(manifests) != 1:
        raise SystemExit(f"application manifest is missing ({target.goarch})")
    manifest = decode_manifest(manifests[0])
    lowered_manifest = manifest.lower()
    for marker in ("permonitorv2", "asinvoker", "longpathaware"):
        if marker not in lowered_manifest:
            raise SystemExit(f"application manifest is incomplete ({target.goarch})")
    verify_version_resource(resource_payloads(path, 16), version, target.goarch)

    hardening = ["ASLR", "NX", "Terminal Server aware"]
    if target.require_high_entropy_va:
        hardening.insert(2, "High Entropy VA")
    print(
        f"{path.name}: PE/resources PASS "
        f"({target.goarch}, GUI, {', '.join(hardening)}, icon, version, PerMonitorV2)"
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", required=True, type=pathlib.Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    for target in RELEASE_TARGETS:
        path = args.directory / target.filename(args.version)
        if not path.is_file():
            raise SystemExit(f"release executable is missing: {path.name}")
        verify_target(path, args.version, target)


if __name__ == "__main__":
    main()
