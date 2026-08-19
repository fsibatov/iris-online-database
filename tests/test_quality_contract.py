import html
import re
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class QualityContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")
        cls.html = (ROOT / "web/index.html").read_text(encoding="utf-8")
        cls.readme = (ROOT / "README.md").read_text(encoding="utf-8")
        cls.wails = (ROOT / "wails.json").read_text(encoding="utf-8")
        cls.windows_info = (ROOT / "build/windows/info.json").read_text(encoding="utf-8")
        cls.release_gate = (ROOT / "scripts/release-gate.sh").read_text(encoding="utf-8")
        cls.windows_release_tools = (ROOT / "scripts/windows/IrisTools.ps1").read_text(encoding="utf-8")
        cls.contributing = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
        cls.vk_workflow = (ROOT / ".github/workflows/update-vk-news.yml").read_text(
            encoding="utf-8"
        )

    def test_version_is_consistent_in_user_facing_sources(self):
        version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
        app_match = re.search(r"const APP_VERSION = '([^']+)';", self.script)
        self.assertIsNotNone(app_match)
        self.assertEqual(version, app_match.group(1))
        self.assertIn(f"Версия {version}", self.html)
        self.assertIn(f"v{version}", self.readme)
        self.assertIn(f'"productVersion": "{version}"', self.wails)
        self.assertIn(f'"file_version": "{version}.0"', self.windows_info)
        self.assertIn(f'"product_version": "{version}.0"', self.windows_info)
        self.assertIn(f'"FileVersion": "{version}"', self.windows_info)
        self.assertIn(f'"ProductVersion": "{version}"', self.windows_info)

    def test_release_gates_run_current_python_regression_suite(self):
        self.assertIn("unittest discover -s tests -p 'test_*.py'", self.release_gate)
        self.assertNotIn("unittest discover -s tools", self.release_gate)
        self.assertIn('"discover", "-s", "tests", "-p", "test_*.py"', self.windows_release_tools)
        self.assertNotIn('"discover", "-s", "tools", "-p", "test_*.py"', self.windows_release_tools)
        self.assertIn('bandit" -q -r tools', self.release_gate)
        self.assertIn('@("-q", "-r", "tools")', self.windows_release_tools)

    def test_item_page_does_not_duplicate_recipe_materials(self):
        detail = self.script[
            self.script.index("function itemDetail") : self.script.index(
                "function effectLabel"
            )
        ]
        self.assertIn(
            "${recipeContext ? recipeMaterialsHTML(data.recipe) : ''}", detail
        )
        self.assertNotIn("\n      ${recipeMaterialsHTML(data.recipe)}", detail)

    def test_recipe_item_routes_are_canonicalized_to_recipe_view(self):
        route = self.script[
            self.script.index("async function renderRoute") : self.script.index(
                "function openMoreMenu"
            )
        ]
        self.assertIn("Array.isArray(data.recipe) && data.recipe.length", route)
        self.assertIn(
            "const recipeRoute = `recipe/${Number(data.item?.id || id)}`", route
        )
        self.assertIn("itemDetail(data, 'recipes')", route)

    def test_async_navigation_keeps_current_page_until_target_is_ready(self):
        route = self.script[
            self.script.index("async function renderRoute") : self.script.index(
                "function openMoreMenu"
            )
        ]
        self.assertIn("const visiblePage = main.querySelector('.page');", route)
        self.assertIn("const preservePageTransition = Boolean(", route)
        self.assertIn(
            "preserveItemDetail || preserveCatalogPage || preservePageTransition", route
        )
        self.assertIn("if (!preserveVisiblePage) loadingPage();", route)
        self.assertIn("visiblePage.setAttribute('inert', '');", route)

    def test_routine_navigation_does_not_force_smooth_scrolling(self):
        self.assertNotIn("scroll-behavior: smooth", self.styles)
        self.assertNotIn("behavior: 'smooth'", self.script)

    def test_home_copy_is_concise_and_has_no_hard_coded_monster_counts(self):
        home = self.script[
            self.script.index("function homePage") : self.script.index(
                "function addHistory"
            )
        ]
        self.assertIn("Рецепты — в отдельном разделе.", home)
        self.assertIn('<h2 id="serverDifferenceTitle">Сервер</h2>', home)
        self.assertIn("Характеристики предметов одинаковы", home)
        self.assertNotIn("The Original — 609 монстров", home)
        self.assertNotIn("Iris Kiss Kiss — 677 монстров", home)
        self.assertNotIn("The Original и Iris Kiss Kiss: в чём разница?", home)

    def test_formatters_are_reused(self):
        self.assertIn("const decimalFormatter = new Intl.NumberFormat", self.script)
        self.assertIn("const dateFormatter = new Intl.DateTimeFormat", self.script)
        self.assertEqual(self.script.count("new Intl.DateTimeFormat('ru-RU'"), 1)
        self.assertEqual(
            self.script.count(
                "new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 })"
            ),
            1,
        )

    def test_vk_workflow_ignores_cosmetic_variants_of_same_post(self):
        for marker in (
            "def semantic_text(value):",
            'BR_TAG.sub("\\n"',
            "DECORATIVE_SYMBOL.sub",
            "MARKUP.sub",
            "same_identity and same_text",
            "VK: без изменений",
        ):
            self.assertIn(marker, self.vk_workflow)
        self.assertNotIn('"source_updated_at",\n          )', self.vk_workflow)

    def test_vk_semantic_comparison_treats_cosmetic_variants_as_equal(self):
        start = self.vk_workflow.index("          BR_TAG = re.compile")
        end = self.vk_workflow.index("          current_id = validate(current)")
        code = textwrap.dedent(self.vk_workflow[start:end])
        namespace = {"html": html, "re": re}
        exec(code, namespace)
        plain = "Новый экспериментальный режим (Vulkan)\n* **Важно:** режим тестовый."
        rich = "⚡ Новый экспериментальный режим (Vulkan)<br><br>* ⚠ **Важно:** режим тестовый."
        self.assertEqual(
            namespace["semantic_text"](plain), namespace["semantic_text"](rich)
        )

    def test_project_rules_require_language_and_noise_review(self):
        self.assertIn("## Постоянные правила качества", self.contributing)
        self.assertIn(
            "орфография → пунктуация → грамматика → фактический смысл → понятность → краткость → единообразие терминологии → соответствие элементу интерфейса",
            self.contributing,
        )
        self.assertIn("старых и слабых ПК", self.contributing)
        self.assertIn("Минимизируйте шум в GitHub", self.contributing)


if __name__ == "__main__":
    unittest.main()
