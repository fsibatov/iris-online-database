from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def replace_once(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"Expected exactly one match in {path}, found {count}: {old[:80]!r}")
    target.write_text(text.replace(old, new, 1), encoding="utf-8")


# web/app.js: no duplicate recipe block on ordinary item pages.
replace_once(
    "web/app.js",
    "      ${recipeMaterialsHTML(data.recipe)}\n",
    "      ${recipeContext ? recipeMaterialsHTML(data.recipe) : ''}\n",
)

# web/app.js: preserve the currently rendered page while async navigation resolves.
replace_once(
    "web/app.js",
    "    const visibleDetail = main.querySelector('.detail-page');\n",
    "    const visiblePage = main.querySelector('.page');\n"
    "    const visibleDetail = main.querySelector('.detail-page');\n",
)
replace_once(
    "web/app.js",
    "    const preserveVisiblePage = preserveItemDetail || preserveCatalogPage;\n",
    "    const preservePageTransition = Boolean(visiblePage && !preserveItemDetail && !preserveCatalogPage && targetPath !== 'home');\n"
    "    const preserveVisiblePage = preserveItemDetail || preserveCatalogPage || preservePageTransition;\n",
)
replace_once(
    "web/app.js",
    "    if (preserveItemDetail) visibleDetail.setAttribute('aria-busy', 'true');\n"
    "    if (preserveCatalogPage) {\n"
    "      visibleCatalog.setAttribute('aria-busy', 'true');\n"
    "      visibleCatalog.setAttribute('inert', '');\n"
    "    }\n",
    "    if (preserveVisiblePage && visiblePage) {\n"
    "      visiblePage.setAttribute('aria-busy', 'true');\n"
    "      visiblePage.setAttribute('inert', '');\n"
    "    }\n",
)
replace_once(
    "web/app.js",
    "        visibleDetail?.removeAttribute('aria-busy');\n"
    "        visibleCatalog?.removeAttribute('aria-busy');\n"
    "        visibleCatalog?.removeAttribute('inert');\n"
    "        state.route = visibleRoute;\n"
    "        replaceRouteHash(visibleRoute);\n"
    "        renderNavigation();\n"
    "        showToast(preserveItemDetail ? 'Не удалось открыть предмет. Повторите переход.' : 'Не удалось открыть каталог. Повторите переход.');\n",
    "        visiblePage?.removeAttribute('aria-busy');\n"
    "        visiblePage?.removeAttribute('inert');\n"
    "        state.route = visibleRoute;\n"
    "        replaceRouteHash(visibleRoute);\n"
    "        renderNavigation();\n"
    "        const failureMessage = preserveItemDetail\n"
    "          ? 'Не удалось открыть предмет. Повторите переход.'\n"
    "          : preserveCatalogPage\n"
    "            ? 'Не удалось открыть каталог. Повторите переход.'\n"
    "            : 'Не удалось открыть страницу. Повторите переход.';\n"
    "        showToast(failureMessage);\n",
)

# web/app.js: fewer short-lived Intl objects on detail/news rendering.
replace_once(
    "web/app.js",
    "  const numberFormatter = new Intl.NumberFormat('ru-RU');\n",
    "  const numberFormatter = new Intl.NumberFormat('ru-RU');\n"
    "  const decimalFormatter = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 });\n"
    "  const dateFormatter = new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });\n",
)
replace_once(
    "web/app.js",
    "    return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(milliseconds / 1000)} с.`;\n",
    "    return `${decimalFormatter.format(milliseconds / 1000)} с.`;\n",
)
replace_once(
    "web/app.js",
    "    return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(raw * 0.01);\n",
    "    return decimalFormatter.format(raw * 0.01);\n",
)
replace_once(
    "web/app.js",
    "    return new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' }).format(parsed);\n",
    "    return dateFormatter.format(parsed);\n",
)

# web/app.js: remove decorative smooth scrolling from routine UI actions.
replace_once(
    "web/app.js",
    "target.scrollIntoView({ block: 'start', behavior: 'smooth' });",
    "target.scrollIntoView({ block: 'start', behavior: 'auto' });",
)

# web/app.js: concise server explanation without stale hard-coded counters.
replace_once(
    "web/app.js",
    "    const serverDifference = `<section class=\"home-server-difference home-compact-section\" aria-labelledby=\"serverDifferenceTitle\">\n"
    "      <h2 id=\"serverDifferenceTitle\">The Original и Iris Kiss Kiss: в чём разница?</h2>\n"
    "      <p>The Original и Iris Kiss Kiss — два сервера Iris Online. Выберите вверху тот сервер, на котором играете.</p>\n"
    "      <p>Предметы и их характеристики одинаковы. Могут отличаться монстры и способы получения предметов: выпадение с монстров, мировая добыча, награды за задания и содержимое сундуков.</p>\n"
    "      <p>После переключения база автоматически показывает монстров и источники получения для выбранного сервера.</p>\n"
    "      <p class=\"home-server-counts\">Сейчас в базе: The Original — 609 монстров · Iris Kiss Kiss — 677 монстров.</p>\n"
    "    </section>`;\n",
    "    const serverDifference = `<section class=\"home-server-difference home-compact-section\" aria-labelledby=\"serverDifferenceTitle\">\n"
    "      <h2 id=\"serverDifferenceTitle\">Сервер</h2>\n"
    "      <p>Выберите The Original или Iris Kiss Kiss в верхней панели. База автоматически меняет монстров и источники получения; характеристики предметов общие.</p>\n"
    "    </section>`;\n",
)

# web/styles.css: do not force a global animation on every scroll operation.
replace_once(
    "web/styles.css",
    "html { min-width: 320px; min-height: 100%; background: var(--bg); scroll-behavior: smooth; }",
    "html { min-width: 320px; min-height: 100%; background: var(--bg); scroll-behavior: auto; }",
)

# tools/update_vk_news.py: normalize cosmetic VK variants to one stable plain-text form.
replace_once(
    "tools/update_vk_news.py",
    "CHARSET_PATTERN = re.compile(r\"charset\\s*=\\s*[\\\\'\\\"]?([A-Za-z0-9._-]+)\", re.IGNORECASE)\n\n\ndef normalize_text(value: str) -> str:\n    value = (\n        html.unescape(str(value or \"\"))\n        .replace(\"\\\\n\", \"\\n\")\n        .replace(\"\\r\\n\", \"\\n\")\n        .replace(\"\\r\", \"\\n\")\n    )\n",
    "CHARSET_PATTERN = re.compile(r\"charset\\s*=\\s*[\\\\'\\\"]?([A-Za-z0-9._-]+)\", re.IGNORECASE)\n"
    "BR_TAG_PATTERN = re.compile(r\"<br\\s*/?>\", re.IGNORECASE)\n"
    "DECORATIVE_SYMBOL_PATTERN = re.compile(\"[\\u2600-\\u27BF\\U0001F300-\\U0001FAFF\\uFE0F\\u200D]\")\n\n\n"
    "def normalize_text(value: str) -> str:\n"
    "    value = BR_TAG_PATTERN.sub(\"\\n\", str(value or \"\"))\n"
    "    value = (\n"
    "        DECORATIVE_SYMBOL_PATTERN.sub(\"\", html.unescape(value))\n"
    "        .replace(\"\\\\n\", \"\\n\")\n"
    "        .replace(\"\\r\\n\", \"\\n\")\n"
    "        .replace(\"\\r\", \"\\n\")\n"
    "    )\n",
)

# Permanent project rules: performance, no duplication, text/version hygiene, low-noise Git history.
contributing_path = ROOT / "CONTRIBUTING.md"
contributing = contributing_path.read_text(encoding="utf-8")
marker = "## Постоянные правила качества"
if marker not in contributing:
    contributing = contributing.rstrip() + """

## Постоянные правила качества

- Производительность — требование по умолчанию. Сохраняйте текущий DOM во время асинхронных переходов, отменяйте устаревшие запросы, используйте пагинацию/ленивую отрисовку и не добавляйте декоративные анимации или зависимости без измеримой пользы. Интерфейс должен оставаться пригодным для старых и слабых ПК.
- Не дублируйте одну и ту же информацию в нескольких разделах без необходимости. Держите один понятный источник истины и связывайте сущности ссылками вместо повторения больших блоков данных.
- При каждом изменении пользовательского интерфейса перепроверяйте видимый русский текст, даты и версии. `VERSION`, версия в `web/app.js`, `web/index.html` и заголовок `README.md` должны оставаться согласованными; номер версии меняется только в рамках реального релиза.
- Минимизируйте шум в GitHub: один логический набор изменений — один PR/squash-commit в `main`; автоматика не должна создавать коммиты из-за одного timestamp, косметического формата или эквивалентного текста.
"""
    contributing_path.write_text(contributing + "\n", encoding="utf-8")

# Focused regression contract for the cleanup rules.
test_path = ROOT / "tests/test_cleanup_contract.py"
test_path.write_text(
    '''import sys\nimport unittest\nfrom pathlib import Path\n\nROOT = Path(__file__).resolve().parents[1]\nsys.path.insert(0, str(ROOT / "tools"))\n\nfrom update_vk_news import normalize_text  # noqa: E402\n\n\nclass CleanupContractTests(unittest.TestCase):\n    @classmethod\n    def setUpClass(cls):\n        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")\n        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")\n        cls.html = (ROOT / "web/index.html").read_text(encoding="utf-8")\n        cls.readme = (ROOT / "README.md").read_text(encoding="utf-8")\n        cls.contributing = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")\n\n    def test_item_page_does_not_duplicate_recipe_materials(self):\n        detail = self.script[\n            self.script.index("function itemDetail") : self.script.index("function effectLabel")\n        ]\n        self.assertIn("${recipeContext ? recipeMaterialsHTML(data.recipe) : ''}", detail)\n        self.assertNotIn("\\n      ${recipeMaterialsHTML(data.recipe)}", detail)\n\n    def test_navigation_preserves_visible_page_until_async_target_is_ready(self):\n        route = self.script[\n            self.script.index("async function renderRoute") : self.script.index("function openMoreMenu")\n        ]\n        self.assertIn("const visiblePage = main.querySelector('.page')", route)\n        self.assertIn("const preservePageTransition", route)\n        self.assertIn("targetPath !== 'home'", route)\n        self.assertIn("if (!preserveVisiblePage) loadingPage();", route)\n        self.assertIn("visiblePage.setAttribute('inert', '')", route)\n\n    def test_routine_ui_does_not_force_smooth_scrolling(self):\n        self.assertNotIn("scroll-behavior: smooth", self.styles)\n        self.assertNotIn("behavior: 'smooth'", self.script)\n\n    def test_formatters_are_reused(self):\n        self.assertIn("const decimalFormatter = new Intl.NumberFormat", self.script)\n        self.assertIn("const dateFormatter = new Intl.DateTimeFormat", self.script)\n        self.assertIn("decimalFormatter.format(milliseconds / 1000)", self.script)\n        self.assertIn("decimalFormatter.format(raw * 0.01)", self.script)\n        self.assertIn("return dateFormatter.format(parsed);", self.script)\n\n    def test_home_copy_has_no_stale_hardcoded_monster_counts(self):\n        home = self.script[\n            self.script.index("function homePage") : self.script.index("function addHistory")\n        ]\n        self.assertNotIn("609 монстров", home)\n        self.assertNotIn("677 монстров", home)\n        self.assertIn("<h2 id=\\\"serverDifferenceTitle\\\">Сервер</h2>", home)\n\n    def test_vk_cosmetic_variants_normalize_to_same_text(self):\n        rich = "⚡ Новый режим<br><br>* ⚠ **Важно:**<br>Текст 🛠"\n        plain = "Новый режим\\n* **Важно:**\\nТекст"\n        self.assertEqual(normalize_text(rich), normalize_text(plain))\n        self.assertNotIn("<br", normalize_text(rich).lower())\n\n    def test_release_version_literals_stay_consistent(self):\n        version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()\n        self.assertIn(f"const APP_VERSION = '{version}'", self.script)\n        self.assertIn(f"Версия {version}", self.html)\n        self.assertIn(f"# Iris Online Database {version}", self.readme)\n\n    def test_project_rules_require_performance_dedup_and_low_noise(self):\n        self.assertIn("Производительность — требование по умолчанию", self.contributing)\n        self.assertIn("Не дублируйте одну и ту же информацию", self.contributing)\n        self.assertIn("перепроверяйте видимый русский текст, даты и версии", self.contributing)\n        self.assertIn("Минимизируйте шум в GitHub", self.contributing)\n\n\nif __name__ == "__main__":\n    unittest.main()\n''',
    encoding="utf-8",
)
