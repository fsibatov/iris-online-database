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

    def test_post_text_fallback_selectors_cover_dom_and_metadata(self):
        self.assertIn('[data-testid="post_text"]', module.POST_TEXT_SELECTORS)
        self.assertIn('[data-testid="wall_post_text"]', module.POST_TEXT_SELECTORS)
        self.assertIn('meta[property="og:description"]', module.POST_META_SELECTORS)
        self.assertIn('meta[name="twitter:description"]', module.POST_META_SELECTORS)

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

    def test_error_summary_does_not_echo_navigation_url(self):
        error = RuntimeError(
            "Page.goto: net::ERR_ABORTED at https://example.invalid/private/path"
        )
        summary = module._error_summary(error)
        self.assertEqual(summary, "net::ERR_ABORTED")
        self.assertNotIn("example.invalid", summary)

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

    def test_workflow_has_schedule_manual_run_and_no_vk_secret(self):
        workflow = (ROOT / ".github" / "workflows" / "update-vk-news.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn("workflow_dispatch:", workflow)
        self.assertIn("cron: '7,17,27,37,47,57 * * * *'", workflow)
        self.assertIn("contents: write", workflow)
        self.assertIn("-r tools/requirements-audit.txt", workflow)
        self.assertNotIn("playwright==", workflow)
        self.assertIn("git diff --quiet -- data/latest-vk.json", workflow)
        self.assertIn("for attempt in 1 2 3", workflow)
        self.assertIn("VK update failed after 3 attempts", workflow)
        self.assertNotIn("VK_ACCESS_TOKEN", workflow)

    def test_seed_news_file_is_valid(self):
        payload = json.loads(
            (ROOT / "data" / "latest-vk.json").read_text(encoding="utf-8")
        )
        self.assertEqual(payload["schema"], 1)
        self.assertEqual(payload["post_id"], 62336)
        self.assertEqual(payload["post_url"], "https://vk.ru/wall-59626511_62336")


if __name__ == "__main__":
    unittest.main()
