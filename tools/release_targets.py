"""Canonical Windows release targets shared by release verification tools."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ReleaseTarget:
    platform: str
    goarch: str
    asset_suffix: str
    level_name: str
    level_value: str
    pe_machine: int
    pe_magic: int
    require_high_entropy_va: bool

    def filename(self, version: str) -> str:
        return f"IrisOnlineDB-{version}-Windows-{self.asset_suffix}.exe"

    @property
    def build_level_marker(self) -> str:
        return f"{self.level_name}={self.level_value}"


PE32 = 0x10B
PE32_PLUS = 0x20B
IMAGE_FILE_MACHINE_I386 = 0x014C
IMAGE_FILE_MACHINE_AMD64 = 0x8664
IMAGE_FILE_MACHINE_ARM64 = 0xAA64

RELEASE_TARGETS = (
    ReleaseTarget(
        platform="windows/amd64",
        goarch="amd64",
        asset_suffix="x64",
        level_name="GOAMD64",
        level_value="v1",
        pe_machine=IMAGE_FILE_MACHINE_AMD64,
        pe_magic=PE32_PLUS,
        require_high_entropy_va=True,
    ),
    ReleaseTarget(
        platform="windows/386",
        goarch="386",
        asset_suffix="x86",
        level_name="GO386",
        level_value="sse2",
        pe_machine=IMAGE_FILE_MACHINE_I386,
        pe_magic=PE32,
        require_high_entropy_va=False,
    ),
    ReleaseTarget(
        platform="windows/arm64",
        goarch="arm64",
        asset_suffix="arm64",
        level_name="GOARM64",
        level_value="v8.0",
        pe_machine=IMAGE_FILE_MACHINE_ARM64,
        pe_magic=PE32_PLUS,
        require_high_entropy_va=True,
    ),
)

TARGET_BY_GOARCH = {target.goarch: target for target in RELEASE_TARGETS}
