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

    def test_workflow_has_schedule_manual_run_and_no_vk_secret(self):
        workflow = (ROOT / ".github" / "workflows" / "update-vk-news.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn("workflow_dispatch:", workflow)
        self.assertIn("cron: '17 * * * *'", workflow)
        self.assertIn("contents: write", workflow)
        self.assertIn("playwright==1.61.0", workflow)
        self.assertIn("git diff --quiet -- data/latest-vk.json", workflow)
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
