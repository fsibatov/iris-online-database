import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "tools" / "update_vk_news.py"
SPEC = importlib.util.spec_from_file_location("update_vk_news", MODULE_PATH)
module = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(module)


class VKNewsUpdaterTests(unittest.TestCase):
    def test_extracts_json_ld_post(self):
        payload = {
            "@context": "https://schema.org",
            "@type": "SocialMediaPosting",
            "datePublished": "2026-08-11T12:34:56+03:00",
            "articleBody": "Текст новости",
            "url": "https://vk.ru/wall-59626511_62336",
        }
        html = (
            '<script type="application/ld+json">'
            + json.dumps(payload, ensure_ascii=False)
            + "</script>"
        )
        post = module.parse_json_ld(html)
        self.assertEqual(post["post_id"], 62336)
        self.assertEqual(post["text"], "Текст новости")
        self.assertEqual(post["published_at"], "2026-08-11T12:34:56+03:00")

    def test_extracts_plain_wall_post(self):
        html = """
        <div data-post-id="-59626511_62337">
          <div class="wall_post_text">Привет<br>мир &amp; Iris</div>
        </div>
        """
        post = module.parse_plain_html(html)
        self.assertEqual(post["post_id"], 62337)
        self.assertEqual(post["text"], "Привет\nмир & Iris")

    def test_extracts_meta_preview(self):
        html = """
        <meta property="og:url" content="https://vk.ru/wall-59626511_62337">
        <meta property="og:description" content="Текст &amp; новости">
        """
        post = module.parse_meta_preview(html)
        self.assertEqual(post["post_id"], 62337)
        self.assertEqual(post["text"], "Текст & новости")

    def test_extracts_wall_post_from_embedded_url(self):
        html = """
        <a href="https://vk.ru/wall-59626511_62338">запись</a>
        <meta property="og:description" content="Текст новости">
        """
        post = module.parse_embedded_post_url(html)
        self.assertEqual(post["post_id"], 62338)
        self.assertEqual(post["text"], "Текст новости")

    def test_decode_text_rejects_vk_shell_copy(self):
        self.assertEqual(module.clean_text("VK — крупнейшая социальная сеть"), "")

    def test_find_latest_post_uses_highest_valid_post_id(self):
        html = """
        <a href="/wall-59626511_62336">old</a>
        <a href="/wall-59626511_62338">new</a>
        <a href="/wall-59626511_62337">middle</a>
        """
        self.assertEqual(module.find_latest_post_id(html), 62338)

    def test_find_latest_post_ignores_other_communities(self):
        html = """
        <a href="/wall-59626511_62337">ours</a>
        <a href="/wall-99999999_99999">other</a>
        """
        self.assertEqual(module.find_latest_post_id(html), 62337)

    def test_find_latest_post_returns_zero_without_visible_posts(self):
        self.assertEqual(module.find_latest_post_id("<html></html>"), 0)

    def test_candidate_post_ids_are_sorted_newest_first(self):
        html = """
        /wall-59626511_62336
        /wall-59626511_62338
        /wall-59626511_62337
        /wall-59626511_62338
        """
        self.assertEqual(module.find_candidate_post_ids(html), [62338, 62337, 62336])

    def test_candidate_post_ids_are_bounded(self):
        html = "\n".join(
            f"/wall-59626511_{post_id}" for post_id in range(62000, 62100)
        )
        candidates = module.find_candidate_post_ids(html)
        self.assertLessEqual(len(candidates), module.MAX_CANDIDATE_POST_IDS)
        self.assertEqual(candidates[0], 62099)

    def test_payload_url_must_match_post_id(self):
        payload = {
            "schema": 1,
            "community_url": module.COMMUNITY_URL,
            "post_id": 62337,
            "post_url": "https://vk.ru/wall-59626511_62336",
            "text": "Новость",
            "published_at": "",
            "source_updated_at": "2026-08-12T16:10:24Z",
        }
        with self.assertRaisesRegex(RuntimeError, "invalid post URL"):
            module.validate_payload(payload)

    def test_payload_rejects_empty_text(self):
        payload = {
            "schema": 1,
            "community_url": module.COMMUNITY_URL,
            "post_id": 62337,
            "post_url": "https://vk.ru/wall-59626511_62337",
            "text": "",
            "published_at": "",
            "source_updated_at": "2026-08-12T16:10:24Z",
        }
        with self.assertRaisesRegex(RuntimeError, "empty preview"):
            module.validate_payload(payload)

    def test_update_preserves_last_known_good_on_empty_or_stale_payload(self):
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
        self.assertIn(
            "for ($Attempt = 1; $Attempt -le 3; $Attempt++)", workflow
        )
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
