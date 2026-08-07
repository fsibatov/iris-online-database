import hashlib
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DISCLAIMER = (
    "Iris Online Database — неофициальное фанатское приложение для Iris Online. "
    "Проект не связан с разработчиками, издателями или правообладателями игры. "
    "Все игровые материалы, названия, логотипы и товарные знаки принадлежат их соответствующим правообладателям."
)
GAME_DATA_SHA256 = "7c3698494233696f2f5728ef17f7e13953159191f966d77b90742dbced23875e"
RARITY_COLORS = {
    "unique": "#fff600",
    "epic": "#d800ff",
    "rare": "#00fffc",
    "normal": "#ffffff",
    "magic": "#00ff00",
    "shop": "#ffcd00",
}


class FanDisclaimerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.html = (ROOT / "web/index.html").read_text(encoding="utf-8")
        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")
        cls.readme = (ROOT / "README.md").read_text(encoding="utf-8")
        cls.notices = (ROOT / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")

    def test_footer_contains_required_russian_disclaimer(self):
        self.assertIn('class="app-footer"', self.html)
        self.assertIn("неофициальное фанатское приложение", self.html)
        self.assertIn("не связан с разработчиками, издателями или правообладателями", self.html)
        self.assertIn(DISCLAIMER, self.html)

    def test_official_link_is_safe_and_accessible(self):
        link = re.search(r'<a\s+[^>]*href="https://irisonline\.ru/"[^>]*>Официальный сайт игры: irisonline\.ru</a>', self.html)
        self.assertIsNotNone(link, "footer official-site link is missing")
        tag = link.group(0)
        self.assertIn('target="_blank"', tag)
        self.assertIn('rel="noopener noreferrer"', tag)
        self.assertIn('aria-label="Официальный сайт игры Iris Online', tag)

    def test_primary_brand_remains_iris_online(self):
        self.assertIn('<strong>Iris Online</strong>', self.html)
        self.assertIn('<title>Iris Online — база данных</title>', self.html)

    def test_footer_is_not_fixed_or_sticky(self):
        match = re.search(r'\.app-footer\s*\{([^}]*)\}', self.styles, re.S)
        self.assertIsNotNone(match, "footer CSS block is missing")
        declarations = match.group(1).lower()
        self.assertNotIn("position: fixed", declarations)
        self.assertNotIn("position: sticky", declarations)

    def test_about_dialog_contains_same_legal_meaning(self):
        self.assertIn(DISCLAIMER, self.script)
        self.assertIn('Официальный сайт игры: irisonline.ru', self.script)
        self.assertIn('rel="noopener noreferrer"', self.script)

    def test_readme_and_notices_are_consistent(self):
        for content in (self.readme, self.notices):
            self.assertIn("© 2026 Iris Online Database", content)
            self.assertIn(DISCLAIMER, content)
            self.assertIn("https://irisonline.ru/", content)

    def test_rarity_colors_are_exactly_preserved(self):
        for quality, color in RARITY_COLORS.items():
            pattern = rf'\.rarity-label\.quality-{quality}\s*\{{[^}}]*color:\s*{re.escape(color)}\s*;'
            self.assertRegex(self.styles, pattern)

    def test_embedded_game_database_is_unchanged(self):
        digest = hashlib.sha256((ROOT / "assets/game_data.json.gz").read_bytes()).hexdigest()
        self.assertEqual(digest, GAME_DATA_SHA256)


if __name__ == "__main__":
    unittest.main()
