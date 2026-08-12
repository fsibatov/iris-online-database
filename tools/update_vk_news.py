#!/usr/bin/env python3
"""Update data/latest-vk.json from the public Iris Online VK wall.

The script intentionally uses a real browser instead of VK API credentials.
If VK is unavailable or its page structure changes, the existing JSON file is
left untouched and the process exits with an error.
"""

from __future__ import annotations

import argparse
import html
import json
import os
import re
import sys
from collections.abc import Iterable
from datetime import datetime, timezone
from pathlib import Path

try:
    from playwright.sync_api import Error as PlaywrightError
    from playwright.sync_api import sync_playwright
except ImportError:
    PlaywrightError = RuntimeError
    sync_playwright = None

COMMUNITY_ID = "59626511"
COMMUNITY_URL = f"https://vk.ru/wall-{COMMUNITY_ID}"
WALL_URLS = (
    COMMUNITY_URL,
    f"https://vk.com/wall-{COMMUNITY_ID}",
    f"https://m.vk.com/wall-{COMMUNITY_ID}",
)
POST_PATTERN = re.compile(rf"wall-{re.escape(COMMUNITY_ID)}_(\d+)", re.IGNORECASE)
MAX_TEXT_LENGTH = 4000
GENERIC_DESCRIPTIONS = (
    "вконтакте — универсальное средство",
    "vk — крупнейшая",
    "vk объединяет",
)


def normalize_text(value: str) -> str:
    value = (
        html.unescape(str(value or ""))
        .replace("\\n", "\n")
        .replace("\r\n", "\n")
        .replace("\r", "\n")
    )
    lines = []
    for line in value.split("\n"):
        line = " ".join(line.strip().split())
        if line:
            lines.append(line)
    result = "\n".join(lines).strip()
    if len(result) > MAX_TEXT_LENGTH:
        result = result[:MAX_TEXT_LENGTH].rstrip() + "…"
    return result


def post_ids(values: Iterable[str]) -> list[int]:
    found: set[int] = set()
    for value in values:
        for match in POST_PATTERN.finditer(str(value or "")):
            try:
                post_id = int(match.group(1))
            except ValueError:
                continue
            if post_id > 0:
                found.add(post_id)
    return sorted(found)


def latest_post_id(values: Iterable[str]) -> int:
    ids = post_ids(values)
    return ids[-1] if ids else 0


def clean_published_at(value: str) -> str:
    value = str(value or "").strip()
    if not value:
        return ""
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return ""
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return (
        parsed.astimezone(timezone.utc)
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


def useful_text(value: str) -> str:
    value = normalize_text(value)
    lowered = value.lower()
    if not value or any(marker in lowered for marker in GENERIC_DESCRIPTIONS):
        return ""
    return value


def _first_locator_text(page, selectors: tuple[str, ...]) -> str:
    for selector in selectors:
        try:
            locator = page.locator(selector)
            count = min(locator.count(), 8)
            for index in range(count):
                candidate = locator.nth(index)
                if not candidate.is_visible(timeout=500):
                    continue
                text = useful_text(candidate.inner_text(timeout=1500))
                if text:
                    return text
        except PlaywrightError:
            locator = None
        if locator is None:
            continue
    return ""


def _meta_content(page, selectors: tuple[str, ...]) -> str:
    for selector in selectors:
        try:
            value = page.locator(selector).first.get_attribute("content", timeout=1000)
        except PlaywrightError:
            value = None
        value = useful_text(value or "")
        if value:
            return value
    return ""


def scrape_latest_post() -> dict[str, object]:
    if sync_playwright is None:
        raise RuntimeError(
            "Playwright не установлен. Выполните: pip install playwright"
        )

    with sync_playwright() as playwright:
        executable = os.environ.get("PLAYWRIGHT_CHROMIUM_EXECUTABLE", "").strip()
        launch_args = {"headless": True}
        if executable:
            launch_args["executable_path"] = executable
        browser = playwright.chromium.launch(**launch_args)
        context = browser.new_context(
            locale="ru-RU",
            timezone_id="Europe/Moscow",
            viewport={"width": 1280, "height": 900},
            user_agent=(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/149.0.0.0 Safari/537.36"
            ),
        )
        page = context.new_page()
        page.set_default_timeout(5000)
        found_id = 0
        last_error = None
        for wall_url in WALL_URLS:
            try:
                page.goto(wall_url, wait_until="domcontentloaded", timeout=45000)
                page.wait_for_timeout(5000)
                hrefs = page.locator("a[href]").evaluate_all(
                    "els => els.map(el => el.getAttribute('href') || '')"
                )
                found_id = latest_post_id([*hrefs, page.content()])
                if found_id:
                    break
            except PlaywrightError as exc:
                last_error = exc
        if not found_id:
            browser.close()
            suffix = f": {last_error}" if last_error else ""
            raise RuntimeError(
                f"Не удалось определить последнюю запись на публичной стене VK{suffix}"
            )

        post_url = f"https://vk.ru/wall-{COMMUNITY_ID}_{found_id}"
        try:
            page.goto(post_url, wait_until="domcontentloaded", timeout=45000)
            page.wait_for_timeout(2500)
        except PlaywrightError as exc:
            last_error = exc

        text = _first_locator_text(
            page,
            (
                '[data-testid="post_text"]',
                ".wall_post_text",
                '[class*="wall_post_text"]',
                '[class*="PostText"]',
                '[class*="post_text"]',
            ),
        )
        if not text:
            text = _meta_content(
                page,
                (
                    'meta[property="og:description"]',
                    'meta[name="description"]',
                ),
            )

        published_at = ""
        for selector, attr in (
            ('meta[property="article:published_time"]', "content"),
            ("time[datetime]", "datetime"),
        ):
            try:
                raw = (
                    page.locator(selector).first.get_attribute(attr, timeout=1000) or ""
                )
            except PlaywrightError:
                raw = ""
            published_at = clean_published_at(raw)
            if published_at:
                break

        browser.close()
        return {
            "schema": 1,
            "community_url": COMMUNITY_URL,
            "post_id": found_id,
            "post_url": post_url,
            "text": text,
            "published_at": published_at,
            "source_updated_at": datetime.now(timezone.utc)
            .replace(microsecond=0)
            .isoformat()
            .replace("+00:00", "Z"),
        }


def comparable(payload: dict[str, object]) -> dict[str, object]:
    return {
        key: payload.get(key, "")
        for key in (
            "schema",
            "community_url",
            "post_id",
            "post_url",
            "text",
            "published_at",
        )
    }


def update_file(output: Path, payload: dict[str, object]) -> bool:
    old: dict[str, object] = {}
    if output.exists():
        try:
            old = json.loads(output.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            old = {}
    if comparable(old) == comparable(payload):
        print(f"VK: без изменений, последняя запись #{payload['post_id']}")
        return False
    output.parent.mkdir(parents=True, exist_ok=True)
    temp = output.with_suffix(output.suffix + ".tmp")
    temp.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    temp.replace(output)
    print(f"VK: обновлена запись #{payload['post_id']} -> {output}")
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/latest-vk.json"))
    args = parser.parse_args()
    try:
        payload = scrape_latest_post()
        update_file(args.output, payload)
    except (OSError, RuntimeError, PlaywrightError) as exc:
        print(f"Ошибка обновления VK: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
