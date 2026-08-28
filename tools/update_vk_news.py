"""Update data/latest-vk.json from the public Iris Online VK wall.

The script intentionally uses a real browser instead of VK API credentials.
If VK is unavailable or its page structure changes, the existing JSON file is
left untouched and the process exits with an error.
"""

from __future__ import annotations

import argparse
import contextlib
import html
import json
import os
import re
import sys
from collections.abc import Iterable
from datetime import datetime, timezone
from html.parser import HTMLParser
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
MAX_TEXT_LENGTH = 700
MIN_PREVIEW_LENGTH = 2
MIN_VISIBLE_POSTS_FOR_ROLLBACK = 2
NAVIGATION_TIMEOUT_MS = 30000
DOM_CONTENT_TIMEOUT_MS = 8000
DOM_SETTLE_MS = 1500
USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/149.0.0.0 Safari/537.36"
)
GENERIC_DESCRIPTIONS = (
    "вконтакте — универсальное средство",
    "vk — крупнейшая",
    "vk объединяет",
)
POST_TEXT_SELECTORS = (
    '[data-testid="post_text"]',
    '[data-testid="wall_post_text"]',
    ".wall_post_text",
    '[class*="wall_post_text"]',
    '[class*="PostText"]',
    '[class*="post_text"]',
)
POST_META_SELECTORS = (
    'meta[property="og:description"]',
    'meta[name="twitter:description"]',
    'meta[name="description"]',
)
PERSISTED_PAYLOAD_KEYS = (
    "schema",
    "community_url",
    "post_id",
    "post_url",
    "text",
    "published_at",
    "source_updated_at",
)
CHARSET_PATTERN = re.compile(r"charset\s*=\s*[\\'\"]?([A-Za-z0-9._-]+)", re.IGNORECASE)


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
        result = result[: MAX_TEXT_LENGTH - 1].rstrip() + "…"
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


def validate_payload(payload: dict[str, object]) -> None:
    post_id = payload.get("post_id")
    text = useful_text(str(payload.get("text") or ""))
    if not isinstance(post_id, int) or isinstance(post_id, bool) or post_id <= 0:
        raise RuntimeError("VK returned an invalid post identifier")
    if len(text) < MIN_PREVIEW_LENGTH:
        raise RuntimeError(
            f"VK post #{post_id} has no usable public text preview; "
            "last-known-good data was preserved"
        )
    expected_url = f"https://vk.ru/wall-{COMMUNITY_ID}_{post_id}"
    if str(payload.get("post_url") or "") != expected_url:
        raise RuntimeError("VK returned an invalid post URL")
    payload["text"] = text


class _MetaParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.values: dict[str, str] = {}

    def handle_starttag(self, tag: str, attrs) -> None:
        if tag.lower() != "meta":
            return
        values = {str(key).lower(): str(value or "") for key, value in attrs}
        key = (values.get("property") or values.get("name") or "").lower()
        content = values.get("content", "")
        if key and content and key not in self.values:
            self.values[key] = content


def _metadata_from_html(raw_html: str) -> tuple[str, str]:
    parser = _MetaParser()
    try:
        parser.feed(str(raw_html or ""))
    except (ValueError, TypeError):
        return "", ""
    text = ""
    for key in ("og:description", "twitter:description", "description"):
        text = useful_text(parser.values.get(key, ""))
        if text:
            break
    published_at = clean_published_at(parser.values.get("article:published_time", ""))
    return text, published_at


def _error_summary(exc: Exception) -> str:
    message = " ".join(str(exc).split())
    for marker in (
        "net::ERR_ABORTED",
        "net::ERR_TIMED_OUT",
        "net::ERR_CONNECTION_RESET",
        "net::ERR_CONNECTION_CLOSED",
        "Timeout",
    ):
        if marker.lower() in message.lower():
            return marker
    return exc.__class__.__name__


def _safe_failure_category(exc: Exception) -> str:
    message = str(exc).lower()
    categories = (
        ("no usable public text", "EMPTY_PREVIEW"),
        ("older post", "STALE_POST"),
        ("invalid post", "INVALID_PAYLOAD"),
        ("playwright не установлен", "MISSING_DEPENDENCY"),
        ("не удалось определить последнюю запись", "VK_UNAVAILABLE"),
    )
    for marker, category in categories:
        if marker in message:
            return category
    if isinstance(exc, OSError):
        return "LOCAL_IO_FAILURE"
    if sync_playwright is not None and isinstance(exc, PlaywrightError):
        return f"BROWSER_{_error_summary(exc).upper()}"
    return "UPDATE_FAILED"


def _navigate_page(page, url: str) -> str:
    """Navigate without treating an interrupted post-commit load as immediate failure."""
    error = ""
    try:
        page.goto(url, wait_until="commit", timeout=NAVIGATION_TIMEOUT_MS)
    except PlaywrightError as exc:
        error = _error_summary(exc)
    with contextlib.suppress(PlaywrightError):
        page.wait_for_load_state("domcontentloaded", timeout=DOM_CONTENT_TIMEOUT_MS)
    with contextlib.suppress(PlaywrightError):
        page.wait_for_timeout(DOM_SETTLE_MS)
    return error


def _visible_post_ids_from_page(page) -> list[int]:
    """Return only public post links that are actually visible on the wall."""
    values: list[str] = []
    with contextlib.suppress(PlaywrightError):
        values.extend(
            page.locator("a[href]").evaluate_all(
                """
                els => els
                    .filter(el => {
                        const href = el.getAttribute('href') || '';
                        if (!href.includes('wall-59626511_')) return false;

                        if (el.closest('[hidden], [aria-hidden="true"]')) return false;
                        const style = window.getComputedStyle(el);
                        const rect = el.getBoundingClientRect();
                        if (
                            style.display === 'none' ||
                            style.visibility === 'hidden' ||
                            style.opacity === '0' ||
                            rect.width <= 0 ||
                            rect.height <= 0
                        ) return false;

                        let node = el;
                        for (
                            let depth = 0;
                            node && depth < 8;
                            depth += 1, node = node.parentElement
                        ) {
                            const text = (node.innerText || '').toLowerCase();
                            if (
                                text.includes('запись удалена') ||
                                text.includes('post deleted') ||
                                text.includes('post has been deleted')
                            ) return false;
                        }
                        return true;
                    })
                    .map(el => el.getAttribute('href') || '')
                """
            )
        )
    return post_ids(values)


def _post_id_from_page(page) -> int:
    ids = _visible_post_ids_from_page(page)
    return ids[-1] if ids else 0


def _decode_http_body(payload: bytes, headers: dict[str, str] | None = None) -> str:
    """Decode an HTML response without assuming that VK always returns UTF-8."""

    if not payload:
        return ""

    declared: list[str] = []
    for key, value in (headers or {}).items():
        if str(key).lower() != "content-type":
            continue
        match = CHARSET_PATTERN.search(str(value))
        if match:
            declared.append(match.group(1))

    # HTML charset declarations are ASCII-compatible even when the document body
    # itself is Windows-1251, so inspecting a small prefix is safe.
    prefix = payload[:8192].decode("ascii", errors="ignore")
    for match in CHARSET_PATTERN.finditer(prefix):
        declared.append(match.group(1))

    candidates = [*declared, "utf-8", "windows-1251"]
    seen: set[str] = set()
    for encoding in candidates:
        normalized = encoding.strip().lower()
        if not normalized or normalized in seen:
            continue
        seen.add(normalized)
        try:
            return payload.decode(normalized)
        except (LookupError, UnicodeDecodeError):
            continue

    # Never let an unexpected upstream encoding turn the scheduled updater into
    # a traceback. Replacement characters are preferable to discarding the LKG
    # update path entirely; downstream parsing still validates the extracted post.
    return payload.decode("utf-8", errors="replace")


def _request_html(context, url: str) -> tuple[str, str]:
    response = None
    try:
        response = context.request.get(
            url,
            timeout=NAVIGATION_TIMEOUT_MS,
            fail_on_status_code=False,
            headers={
                "User-Agent": USER_AGENT,
                "Accept-Language": "ru-RU,ru;q=0.9,en;q=0.5",
                "Cache-Control": "no-cache",
                "Pragma": "no-cache",
            },
        )
        if response.status >= 400:
            return "", f"HTTP {response.status}"
        return _decode_http_body(response.body(), response.headers), ""
    except PlaywrightError as exc:
        return "", _error_summary(exc)
    finally:
        if response is not None:
            response.dispose()


def _published_at_from_page(page) -> str:
    for selector, attr in (
        ('meta[property="article:published_time"]', "content"),
        ("time[datetime]", "datetime"),
    ):
        try:
            raw = page.locator(selector).first.get_attribute(attr, timeout=1000) or ""
        except PlaywrightError:
            raw = ""
        published_at = clean_published_at(raw)
        if published_at:
            return published_at
    return ""


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


def _post_text_from_wall(page, post_id: int) -> str:
    """Read the matching post text from the already loaded wall when possible."""
    needle = f"wall-{COMMUNITY_ID}_{post_id}"
    try:
        links = page.locator(f'a[href*="{needle}"]')
        count = min(links.count(), 20)
        for index in range(count):
            link = links.nth(index)
            if not link.is_visible(timeout=500):
                continue
            value = link.evaluate(
                """
                anchor => {
                    let node = anchor;
                    const selector = [
                        '[data-testid="post_text"]',
                        '[data-testid="wall_post_text"]',
                        '.wall_post_text',
                        '[class*="wall_post_text"]',
                        '[class*="PostText"]',
                        '[class*="post_text"]'
                    ].join(', ');

                    for (
                        let depth = 0;
                        node && depth < 9;
                        depth += 1, node = node.parentElement
                    ) {
                        const candidate = node.matches?.(selector)
                            ? node
                            : node.querySelector?.(selector);
                        if (!candidate) continue;
                        const text = candidate.innerText || candidate.textContent || '';
                        if (text.trim()) return text;
                    }
                    return '';
                }
                """
            )
            text = useful_text(value or "")
            if text:
                return text
    except PlaywrightError:
        return ""
    return ""


def _wait_for_post_text(page, fallback: str = "") -> str:
    """Wait briefly for dynamically rendered post text, then use wall fallback."""
    for _ in range(8):
        text = _first_locator_text(page, POST_TEXT_SELECTORS)
        if not text:
            text = _meta_content(page, POST_META_SELECTORS)
        if text:
            return text
        page.wait_for_timeout(750)
    return useful_text(fallback)


def _find_latest_wall_post(context):
    """Probe every public wall variant and keep the highest post identifier found."""
    found_id = 0
    wall_page = None
    visible_wall_ids: list[int] = []
    wall_snapshot_trusted = False
    diagnostics: list[str] = []

    for wall_url in WALL_URLS:
        page = context.new_page()
        page.set_default_timeout(5000)
        navigation_error = _navigate_page(page, wall_url)
        page_ids = _visible_post_ids_from_page(page)
        page_id = page_ids[-1] if page_ids else 0
        if navigation_error:
            diagnostics.append(f"browser {wall_url.split('/')[2]}: {navigation_error}")

        raw_html, request_error = _request_html(context, wall_url)
        if request_error:
            diagnostics.append(f"http {wall_url.split('/')[2]}: {request_error}")
        http_id = latest_post_id([raw_html]) if raw_html else 0
        candidate_id = max(page_id, http_id)

        candidate_has_page = page_id > 0 and page_id == candidate_id
        should_replace = candidate_id > found_id
        should_adopt_equal_page = (
            candidate_id > 0
            and candidate_id == found_id
            and candidate_has_page
            and wall_page is None
        )

        if should_replace or should_adopt_equal_page:
            if wall_page is not None and wall_page is not page:
                wall_page.close()
            found_id = candidate_id
            if candidate_has_page:
                wall_page = page
                visible_wall_ids = page_ids
                wall_snapshot_trusted = (
                    not navigation_error
                    and len(page_ids) >= MIN_VISIBLE_POSTS_FOR_ROLLBACK
                )
            else:
                wall_page = None
                visible_wall_ids = []
                wall_snapshot_trusted = False
                page.close()
        elif page is not wall_page:
            page.close()

    return (
        found_id,
        wall_page,
        visible_wall_ids,
        wall_snapshot_trusted,
        diagnostics,
    )


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
            user_agent=USER_AGENT,
        )

        try:
            (
                found_id,
                wall_page,
                visible_wall_ids,
                wall_snapshot_trusted,
                diagnostics,
            ) = _find_latest_wall_post(context)

            if not found_id:
                detail = "; ".join(diagnostics[-6:])
                if detail:
                    detail = f" ({detail})"
                raise RuntimeError(
                    "Не удалось определить последнюю запись на публичной стене VK"
                    + detail
                )

            wall_text = (
                _post_text_from_wall(wall_page, found_id)
                if wall_page is not None
                else ""
            )
            post_url = f"https://vk.ru/wall-{COMMUNITY_ID}_{found_id}"
            post_page = wall_page if wall_page is not None else context.new_page()
            post_page.set_default_timeout(5000)
            navigation_error = _navigate_page(post_page, post_url)
            if navigation_error:
                diagnostics.append(f"post browser: {navigation_error}")

            text = _wait_for_post_text(post_page, fallback=wall_text)
            published_at = _published_at_from_page(post_page)

            if not text or not published_at:
                raw_post, request_error = _request_html(context, post_url)
                if request_error:
                    diagnostics.append(f"post http: {request_error}")
                if raw_post:
                    metadata_text, metadata_published_at = _metadata_from_html(raw_post)
                    if not text:
                        text = metadata_text
                    if not published_at:
                        published_at = metadata_published_at

            if diagnostics:
                print(
                    "VK: использованы резервные пути: " + "; ".join(diagnostics[-6:]),
                    file=sys.stderr,
                )

            if text:
                print(f"VK: запись #{found_id}, получено {len(text)} символов текста")
            else:
                print(
                    f"VK: запись #{found_id} найдена, но текст извлечь не удалось",
                    file=sys.stderr,
                )

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
                "_visible_wall_ids": visible_wall_ids,
                "_wall_snapshot_trusted": wall_snapshot_trusted,
            }
        finally:
            browser.close()


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


def _persisted_payload(payload: dict[str, object]) -> dict[str, object]:
    return {key: payload.get(key, "") for key in PERSISTED_PAYLOAD_KEYS}


def _trusted_deleted_post_rollback(
    payload: dict[str, object], old_id: int, new_id: int
) -> bool:
    if payload.get("_wall_snapshot_trusted") is not True:
        return False
    raw_ids = payload.get("_visible_wall_ids")
    if not isinstance(raw_ids, list):
        return False
    visible_ids = sorted(
        {
            value
            for value in raw_ids
            if isinstance(value, int) and not isinstance(value, bool) and value > 0
        }
    )
    return (
        len(visible_ids) >= MIN_VISIBLE_POSTS_FOR_ROLLBACK
        and new_id == visible_ids[-1]
        and old_id not in visible_ids
    )


def update_file(output: Path, payload: dict[str, object]) -> bool:
    persisted = _persisted_payload(payload)
    validate_payload(persisted)
    old: dict[str, object] = {}
    if output.exists():
        try:
            old = json.loads(output.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            old = {}
    old_id = old.get("post_id")
    new_id = persisted.get("post_id")
    if isinstance(old_id, int) and isinstance(new_id, int) and new_id < old_id:
        if _trusted_deleted_post_rollback(payload, old_id, new_id):
            print(
                f"VK: предыдущая запись #{old_id} отсутствует среди видимых "
                f"записей стены; актуальная запись #{new_id} восстановлена"
            )
        else:
            raise RuntimeError(
                f"VK returned older post #{new_id} while last-known-good is #{old_id}; "
                "existing data was preserved"
            )
    if comparable(old) == comparable(persisted):
        print(f"VK: без изменений, последняя запись #{persisted['post_id']}")
        return False
    output.parent.mkdir(parents=True, exist_ok=True)
    temp = output.with_suffix(output.suffix + ".tmp")
    temp.write_text(
        json.dumps(persisted, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    temp.replace(output)
    print(f"VK: обновлена запись #{persisted['post_id']}")
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/latest-vk.json"))
    args = parser.parse_args()
    try:
        payload = scrape_latest_post()
        update_file(args.output, payload)
    except (OSError, RuntimeError, PlaywrightError) as exc:
        print(
            f"Ошибка обновления VK [{_safe_failure_category(exc)}]",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
