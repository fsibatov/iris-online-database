import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools" / "update_vk_news.py"
spec = importlib.util.spec_from_file_location("update_vk_news", SCRIPT)
module = importlib.util.module_from_spec(spec)
assert spec and spec.loader
spec.loader.exec_module(module)


class VKNewsUpdaterTests(unittest.TestCase):
    def fixture(self, name):
        return (ROOT / "tools" / "fixtures" / "vk" / name).read_text(encoding="utf-8")

    def test_latest_post_id_uses_highest_wall_post(self):
        values = [
            'href="/wall-59626511_62335"',
            "https://vk.ru/wall-59626511_62336?reply=1",
            "wall-123_999999",
        ]
        self.assertEqual(module.latest_post_id(values), 62336)

    def test_normalize_text(self):
        self.assertEqual(
            module.normalize_text("  Один  \\n\\n Два   слова "), "Один\nДва слова"
        )

    def test_normalize_text_caps_preview_at_700_characters(self):
        text = module.normalize_text("А" * 900)
        self.assertEqual(module.MAX_TEXT_LENGTH, 700)
        self.assertEqual(len(text), 700)
        self.assertTrue(text.endswith("…"))

    def test_post_text_fallback_selectors_cover_dom_and_metadata(self):
        self.assertIn('[data-testid="post_text"]', module.POST_TEXT_SELECTORS)
        self.assertIn('[data-testid="wall_post_text"]', module.POST_TEXT_SELECTORS)
        self.assertIn('meta[property="og:description"]', module.POST_META_SELECTORS)
        self.assertIn('meta[name="twitter:description"]', module.POST_META_SELECTORS)

    def test_http_body_decoder_handles_windows_1251_without_charset_header(self):
        raw = "<html><body>Актуальная запись ВКонтакте</body></html>".encode(
            "windows-1251"
        )
        decoded = module._decode_http_body(raw, {"content-type": "text/html"})
        self.assertIn("Актуальная запись ВКонтакте", decoded)

    def test_http_body_decoder_honours_declared_windows_1251_charset(self):
        raw = '<meta charset="windows-1251"><p>А</p>'.encode("windows-1251")
        self.assertIn(b"\xc0", raw)
        decoded = module._decode_http_body(
            raw, {"Content-Type": "text/html; charset=windows-1251"}
        )
        self.assertEqual(decoded, '<meta charset="windows-1251"><p>А</p>')

    def test_vk_request_uses_raw_body_instead_of_playwright_utf8_text(self):
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("response.body()", source)
        self.assertNotIn("return response.text(),", source)

    def test_metadata_from_html_extracts_preview_and_timestamp(self):
        raw = (
            "<html><head>"
            '<meta property="og:description" content="  Текст   новости  ">'
            '<meta property="article:published_time" content="2026-08-12T15:00:00+03:00">'
            "</head></html>"
        )
        text, published_at = module._metadata_from_html(raw)
        self.assertEqual(text, "Текст новости")
        self.assertEqual(published_at, "2026-08-12T12:00:00Z")

    def test_deterministic_html_fixtures(self):
        cases = {
            "text-post.html": (
                62337,
                "Обычная текстовая запись",
                "2026-08-12T12:00:00Z",
            ),
            "text-image-post.html": (62338, "Запись с текстом и изображением", ""),
            "changed-selector.html": (62340, "Текст доступен через OpenGraph", ""),
        }
        for filename, (post_id, text_prefix, published_at) in cases.items():
            with self.subTest(filename=filename):
                raw = self.fixture(filename)
                self.assertEqual(module.latest_post_id([raw]), post_id)
                text, timestamp = module._metadata_from_html(raw)
                self.assertTrue(text.startswith(text_prefix), text)
                self.assertEqual(timestamp, published_at)

    def test_attachment_only_and_empty_fixtures_fail_closed(self):
        for filename, post_id in (
            ("attachments-only.html", 62339),
            ("empty-response.html", 0),
        ):
            with self.subTest(filename=filename):
                raw = self.fixture(filename)
                self.assertEqual(module.latest_post_id([raw]), post_id)
                text, _ = module._metadata_from_html(raw)
                self.assertEqual(text, "")

    def test_timeout_and_aborted_errors_are_safely_classified(self):
        for message, expected in (
            (
                "Page.goto: net::ERR_ABORTED at https://vk.invalid/private",
                "net::ERR_ABORTED",
            ),
            ("Timeout 30000ms exceeded at https://vk.invalid/private", "Timeout"),
        ):
            with self.subTest(expected=expected):
                summary = module._error_summary(RuntimeError(message))
                self.assertEqual(summary, expected)
                self.assertNotIn("vk.invalid", summary)

    def test_error_summary_does_not_echo_navigation_url(self):
        error = RuntimeError(
            "Page.goto: net::ERR_ABORTED at https://example.invalid/private/path"
        )
        summary = module._error_summary(error)
        self.assertEqual(summary, "net::ERR_ABORTED")
        self.assertNotIn("example.invalid", summary)

    def test_safe_failure_category_never_echoes_raw_payload(self):
        payload = "/" + "home/" + "private-user/project?token=not-for-logs"
        category = module._safe_failure_category(
            RuntimeError(f"Page.goto failed at {payload}")
        )
        self.assertEqual(category, "UPDATE_FAILED")
        self.assertNotIn(payload, category)
        self.assertNotIn("private-user", category)

    def test_update_file_does_not_rewrite_unchanged_post(self):
        payload = {
            "schema": 1,
            "community_url": module.COMMUNITY_URL,
            "post_id": 62336,
            "post_url": "https://vk.ru/wall-59626511_62336",
            "text": "Новость",
            "published_at": "",
            "source_updated_at": "2026-08-11T20:00:00Z",
        }
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            self.assertTrue(module.update_file(path, payload))
            first = path.read_bytes()
            newer = dict(payload, source_updated_at="2026-08-12T01:00:00Z")
            self.assertFalse(module.update_file(path, newer))
            self.assertEqual(path.read_bytes(), first)
            loaded = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(loaded["post_id"], 62336)

    def test_update_file_rewrites_same_post_when_preview_appears(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            old = {
                "schema": 1,
                "community_url": module.COMMUNITY_URL,
                "post_id": 62336,
                "post_url": "https://vk.ru/wall-59626511_62336",
                "text": "",
                "published_at": "",
                "source_updated_at": "2026-08-11T20:00:00Z",
            }
            path.write_text(
                json.dumps(old, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )
            new = dict(
                old,
                text="Текст превью",
                source_updated_at="2026-08-12T01:00:00Z",
            )
            self.assertTrue(module.update_file(path, new))
            loaded = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(loaded["text"], "Текст превью")

    def test_update_file_preserves_last_known_good_on_empty_or_stale_payload(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            old = {
                "schema": 1,
                "community_url": module.COMMUNITY_URL,
                "post_id": 62337,
                "post_url": "https://vk.ru/wall-59626511_62337",
                "text": "Последнее корректное превью",
                "published_at": "",
                "source_updated_at": "2026-08-12T16:10:24Z",
            }
            path.write_text(json.dumps(old, ensure_ascii=False), encoding="utf-8")
            original = path.read_bytes()
            empty = dict(
                old,
                post_id=62338,
                post_url="https://vk.ru/wall-59626511_62338",
                text="",
            )
            with self.assertRaisesRegex(RuntimeError, "last-known-good"):
                module.update_file(path, empty)
            stale = dict(
                old,
                post_id=62336,
                post_url="https://vk.ru/wall-59626511_62336",
                text="Старое превью",
            )
            with self.assertRaisesRegex(RuntimeError, "older post"):
                module.update_file(path, stale)
            self.assertEqual(path.read_bytes(), original)

    def test_redirect_is_rejected_as_invalid_post_url(self):
        payload = {
            "schema": 1,
            "community_url": module.COMMUNITY_URL,
            "post_id": 62337,
            "post_url": "https://vk.ru/away.php?to=https://example.invalid",
            "text": "Новость",
            "published_at": "",
            "source_updated_at": "2026-08-12T16:10:24Z",
        }
        with self.assertRaisesRegex(RuntimeError, "invalid post URL"):
            module.validate_payload(payload)

    def test_workflow_has_schedule_manual_run_and_no_vk_secret(self):
        workflow = (ROOT / ".github" / "workflows" / "update-vk-news.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn("workflow_dispatch:", workflow)
        self.assertIn("cron: '7,17,27,37,47,57 * * * *'", workflow)
        self.assertIn("contents: write", workflow)
        self.assertIn("runs-on: windows-2025", workflow)
        self.assertIn("shell: powershell", workflow)
        self.assertIn("-r tools\\requirements-audit.txt", workflow)
        self.assertNotRegex(workflow, r"pip install[^\n]*playwright==")
        self.assertIn("tools\\verify_python_environment.py", workflow)
        self.assertIn("git diff --quiet -- data/latest-vk.json", workflow)
        self.assertIn("for ($Attempt = 1; $Attempt -le 3; $Attempt++)", workflow)
        self.assertIn("VK transient failure", workflow)
        self.assertIn("::warning::VK is temporarily unavailable", workflow)
        self.assertIn("VK updater failed with non-transient category", workflow)
        self.assertNotIn("VK_ACCESS_TOKEN", workflow)
        self.assertNotIn("ubuntu-", workflow)
        self.assertNotIn("shell: bash", workflow)

    def test_seed_news_file_is_valid(self):
        payload = json.loads(
            (ROOT / "data" / "latest-vk.json").read_text(encoding="utf-8")
        )
        self.assertEqual(payload["schema"], 1)
        self.assertGreaterEqual(payload["post_id"], 62336)
        self.assertEqual(
            payload["post_url"],
            f"https://vk.ru/wall-59626511_{payload['post_id']}",
        )
        self.assertTrue(payload["text"].strip())


if __name__ == "__main__":
    unittest.main()
