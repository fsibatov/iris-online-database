"""Generate deterministic Windows COFF .syso resources for Go builds.

Uses only the Python standard library. It embeds one manifest and all images from
resources/icon.ico as RT_MANIFEST, RT_ICON and RT_GROUP_ICON resources.
"""

from __future__ import annotations

import argparse
import pathlib
import struct
from dataclasses import dataclass

MACHINES = {"386": 0x014C, "amd64": 0x8664, "arm64": 0xAA64}
RELOCS = {"386": 0x0007, "amd64": 0x0003, "arm64": 0x0002}
RT_ICON = 3
RT_GROUP_ICON = 14
RT_MANIFEST = 24
LANG_EN_US = 0x0409
SUBDIR = 0x80000000


@dataclass(frozen=True)
class IconEntry:
    width: int
    height: int
    color_count: int
    reserved: int
    planes: int
    bit_count: int
    data: bytes


def read_ico(path: pathlib.Path) -> list[IconEntry]:
    data = path.read_bytes()
    if len(data) < 6:
        raise ValueError("ICO header is truncated")
    reserved, kind, count = struct.unpack_from("<HHH", data, 0)
    if reserved != 0 or kind != 1 or count < 1:
        raise ValueError("not a Windows icon file")
    table_end = 6 + count * 16
    if table_end > len(data):
        raise ValueError("ICO directory is truncated")
    result: list[IconEntry] = []
    for index in range(count):
        off = 6 + index * 16
        width, height, colors, entry_reserved, planes, bits, size, image_off = (
            struct.unpack_from("<BBBBHHII", data, off)
        )
        if size == 0 or image_off < table_end or image_off + size > len(data):
            raise ValueError(f"ICO image {index} has invalid bounds")
        result.append(
            IconEntry(
                width,
                height,
                colors,
                entry_reserved,
                planes,
                bits,
                data[image_off : image_off + size],
            )
        )
    return result


def group_icon_data(entries: list[tuple[IconEntry, int]]) -> bytes:
    out = bytearray(struct.pack("<HHH", 0, 1, len(entries)))
    for icon, resource_id in entries:
        out += struct.pack(
            "<BBBBHHIH",
            icon.width,
            icon.height,
            icon.color_count,
            icon.reserved,
            icon.planes,
            icon.bit_count,
            len(icon.data),
            resource_id,
        )
    return bytes(out)


def build_directory(
    resources: list[tuple[int, int, bytes]],
) -> tuple[bytearray, list[int]]:
    """Return serialized resource directory and leaf DataEntry offsets in resource order."""
    by_type: dict[int, list[tuple[int, bytes]]] = {}
    for kind, resource_id, data in resources:
        by_type.setdefault(kind, []).append((resource_id, data))
    for values in by_type.values():
        values.sort(key=lambda item: item[0])

    types = sorted(by_type)
    buf = bytearray()
    leaves: list[tuple[int, int, int]] = []

    def reserve_dir(entry_count: int) -> tuple[int, list[int]]:
        base = len(buf)
        buf.extend(struct.pack("<IIHHHH", 0, 0, 0, 0, 0, entry_count))
        positions = []
        for _ in range(entry_count):
            positions.append(len(buf))
            buf.extend(b"\0" * 8)
        return base, positions

    _, top_entries = reserve_dir(len(types))
    for top_index, kind in enumerate(types):
        type_dir_off = len(buf)
        struct.pack_into(
            "<II", buf, top_entries[top_index], kind, SUBDIR | type_dir_off
        )
        items = by_type[kind]
        _, id_entries = reserve_dir(len(items))
        for item_index, (resource_id, _data) in enumerate(items):
            id_dir_off = len(buf)
            struct.pack_into(
                "<II", buf, id_entries[item_index], resource_id, SUBDIR | id_dir_off
            )
            _, lang_entries = reserve_dir(1)
            struct.pack_into("<I", buf, lang_entries[0], LANG_EN_US)
            leaves.append((lang_entries[0] + 4, kind, resource_id))

    order = {
        (kind, resource_id): idx
        for idx, (kind, resource_id, _data) in enumerate(resources)
    }

    sorted_keys = sorted(order, key=lambda key: (key[0], key[1]))
    data_index = {key: idx for idx, key in enumerate(sorted_keys)}
    data_entries_base = len(buf)
    for _ in sorted_keys:
        buf.extend(b"\0" * 16)
    for patch, kind, resource_id in leaves:
        struct.pack_into(
            "<I", buf, patch, data_entries_base + data_index[(kind, resource_id)] * 16
        )
    return buf, [data_entries_base + i * 16 for i in range(len(sorted_keys))]


def generate(
    icon_path: pathlib.Path,
    manifest_path: pathlib.Path,
    arch: str,
    output: pathlib.Path,
) -> None:
    icons = read_ico(icon_path)
    manifest = manifest_path.read_bytes()

    icon_pairs = [(icon, 3 + i) for i, icon in enumerate(icons)]
    resources: list[tuple[int, int, bytes]] = [(RT_MANIFEST, 1, manifest)]
    resources.extend(
        (RT_ICON, resource_id, icon.data) for icon, resource_id in icon_pairs
    )
    resources.append((RT_GROUP_ICON, 2, group_icon_data(icon_pairs)))
    resources.sort(key=lambda item: (item[0], item[1]))

    raw, data_entry_offsets = build_directory(resources)
    resource_data_offsets: list[int] = []
    for _kind, _resource_id, payload in resources:
        resource_data_offsets.append(len(raw))
        raw.extend(payload)
        raw.extend(b"\0" * ((-len(payload)) & 7))
    for i, (_kind, _resource_id, payload) in enumerate(resources):
        struct.pack_into(
            "<IIII",
            raw,
            data_entry_offsets[i],
            resource_data_offsets[i],
            len(payload),
            0,
            0,
        )

    file_header_size = 20
    section_header_size = 40
    raw_file_off = file_header_size + section_header_size
    reloc_file_off = raw_file_off + len(raw)
    reloc_type = RELOCS[arch]
    relocations = bytearray()
    for data_entry_off in data_entry_offsets:
        relocations += struct.pack("<IIH", data_entry_off, 0, reloc_type)
    symbol_file_off = reloc_file_off + len(relocations)

    file_header = struct.pack(
        "<HHIIIHH",
        MACHINES[arch],
        1,
        0,
        symbol_file_off,
        1,
        0,
        0x0104,
    )
    section_header = struct.pack(
        "<8sIIIIIIHHI",
        b".rsrc\0\0\0",
        0,
        0,
        len(raw),
        raw_file_off,
        reloc_file_off,
        0,
        len(data_entry_offsets),
        0,
        0x40000040,
    )
    symbol = struct.pack("<8sIhHBB", b".rsrc\0\0\0", 0, 1, 0, 3, 0)
    string_table = struct.pack("<I", 4)

    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(
        file_header + section_header + raw + relocations + symbol + string_table
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--icon", required=True, type=pathlib.Path)
    parser.add_argument("--manifest", required=True, type=pathlib.Path)
    parser.add_argument("--arch", required=True, choices=sorted(MACHINES))
    parser.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    generate(args.icon, args.manifest, args.arch, args.output)


if __name__ == "__main__":
    main()
