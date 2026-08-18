import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools" / "update_vk_news.py"
spec = importlib.util.spec_from_file_location("update_vk_news_deleted_posts", SCRIPT)
module = importlib.util.module_from_spec(spec)
assert spec and spec.loader
spec.loader.exec_module(module)


def payload(post_id: int, text: str = "Новость", **extra):
    value = {
        "schema": 1,
        "community_url": module.COMMUNITY_URL,
        "post_id": post_id,
        "post_url": f"https://vk.ru/wall-{module.COMMUNITY_ID}_{post_id}",
        "text": text,
        "published_at": "",
        "source_updated_at": "2026-08-18T00:00:00Z",
    }
    value.update(extra)
    return value


class VKDeletedPostTests(unittest.TestCase):
    def test_page_post_id_uses_visible_links_without_page_content_fallback(self):
        class Locator:
            def evaluate_all(self, script):
                self.assertions(script)
                return [
                    "/wall-59626511_62336",
                    "/wall-59626511_62337",
                ]

            @staticmethod
            def assertions(script):
                if "getBoundingClientRect" not in script:
                    raise AssertionError("visible-link filtering is missing")
                if "запись удалена" not in script:
                    raise AssertionError("deleted-post filtering is missing")

        class Page:
            def locator(self, selector):
                self.selector = selector
                return Locator()

        page = Page()
        self.assertEqual(module._post_id_from_page(page), 62337)
        self.assertEqual(page.selector, "a[href]")

    def test_lower_id_is_still_rejected_without_trusted_wall_snapshot(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            path.write_text(
                json.dumps(payload(62352), ensure_ascii=False), encoding="utf-8"
            )
            original = path.read_bytes()

            with self.assertRaisesRegex(RuntimeError, "older post"):
                module.update_file(path, payload(62337))

            self.assertEqual(path.read_bytes(), original)

    def test_deleted_previous_post_can_roll_back_to_trusted_visible_latest(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            path.write_text(
                json.dumps(payload(62352), ensure_ascii=False), encoding="utf-8"
            )

            current = payload(
                62337,
                _visible_wall_ids=[62335, 62336, 62337],
                _wall_snapshot_trusted=True,
            )
            self.assertTrue(module.update_file(path, current))

            saved = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(saved["post_id"], 62337)
            self.assertNotIn("_visible_wall_ids", saved)
            self.assertNotIn("_wall_snapshot_trusted", saved)

    def test_lower_id_is_rejected_when_previous_post_is_still_visible(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "latest-vk.json"
            path.write_text(
                json.dumps(payload(62352), ensure_ascii=False), encoding="utf-8"
            )

            current = payload(
                62337,
                _visible_wall_ids=[62337, 62352],
                _wall_snapshot_trusted=True,
            )
            with self.assertRaisesRegex(RuntimeError, "older post"):
                module.update_file(path, current)


if __name__ == "__main__":
    unittest.main()
