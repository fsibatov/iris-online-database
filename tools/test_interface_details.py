import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class InterfaceDetailTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        cls.html = (ROOT / "web/index.html").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")
        cls.server = (ROOT / "server.go").read_text(encoding="utf-8")

    def test_item_presentation_has_separate_groups(self):
        block = self.script[
            self.script.index("function itemPresentation") : self.script.index(
                "function monsterPresentation"
            )
        ]
        for field in (
            "classes",
            "baseStats",
            "bonusStats",
            "miscStats",
            "restrictions",
            "actions",
            "cardSlots",
            "price",
        ):
            self.assertIn(field, block)
        self.assertNotIn("const properties =", block)

    def test_class_is_badge_and_not_a_property_row(self):
        self.assertIn("itemClassBadge(presentation.classes)", self.script)
        presentation = self.script[
            self.script.index("function itemPresentation") : self.script.index(
                "function monsterPresentation"
            )
        ]
        self.assertNotIn("['Класс'", presentation)
        self.assertIn(".class-label", self.styles)

    def test_main_stats_precede_bonuses_and_source(self):
        game_block = self.script[
            self.script.index("function gameProperties") : self.script.index(
                "function meaningfulDescription"
            )
        ]
        self.assertLess(
            game_block.index("presentation.baseStats"),
            game_block.index("presentation.bonusStats"),
        )
        detail = self.script[
            self.script.index("function itemDetail") : self.script.index(
                "function effectLabel"
            )
        ]
        self.assertLess(
            detail.index("gameProperties(presentation"), detail.index("Лучший источник")
        )

    def test_bonus_stats_have_dedicated_green_role(self):
        self.assertIn("modifier === 'bonus'", self.script)
        self.assertIn(".property-row--bonus", self.styles)
        self.assertIn("--game-bonus:", self.styles)
        self.assertIn("--game-bonus-muted:", self.styles)
        self.assertIn("color: var(--game-bonus)", self.styles)
        self.assertIn("color: var(--game-bonus-muted)", self.styles)
        self.assertIn("property-row--penalty", self.styles)

    def test_price_is_separate_from_base_stats(self):
        presentation = self.script[
            self.script.index("function itemPresentation") : self.script.index(
                "function monsterPresentation"
            )
        ]
        base = presentation[
            presentation.index("const baseStats") : presentation.index(
                "const bonusStats"
            )
        ]
        self.assertNotIn("Цена продажи", base)
        self.assertIn("const price", presentation)
        self.assertIn("property-group--price", self.script)

    def test_card_slot_types_are_rendered_as_compact_game_labels(self):
        presentation = self.script[
            self.script.index("function itemPresentation") : self.script.index(
                "function monsterPresentation"
            )
        ]
        self.assertIn("const cardSlots =", presentation)
        self.assertNotIn("['Слоты карт'", presentation)
        self.assertIn("function cardSlotsRow", self.script)
        self.assertIn('class="card-slot-chip"', self.script)
        self.assertIn("values.join(', ')", self.script)
        self.assertIn(".property-card-slots", self.styles)
        self.assertIn(".card-slot-chip", self.styles)

    def test_empty_groups_are_not_rendered(self):
        game_block = self.script[
            self.script.index("function gameProperties") : self.script.index(
                "function meaningfulDescription"
            )
        ]
        self.assertIn("if (bonusContent)", game_block)
        for field in ("miscStats", "restrictions", "actions"):
            self.assertIn(f"presentation.{field}?.length", game_block)
        self.assertIn("cardSlotsRow(presentation.cardSlots)", game_block)
        self.assertIn("if (slots)", game_block)
        self.assertIn("if (inlineSet)", game_block)
        self.assertIn("return groups.length ?", game_block)

    def test_game_properties_are_not_table_layout(self):
        self.assertNotIn("detail-properties", self.script)
        self.assertNotIn(".detail-properties", self.styles)
        game_css = self.styles[
            self.styles.index(".game-properties") : self.styles.index(
                ".source-overview"
            )
        ]
        self.assertNotIn("grid-template-columns: repeat(2", game_css)
        self.assertNotRegex(game_css, r"\.property-row[^}]*border-bottom")
        self.assertRegex(
            game_css, r"\.property-row\s*\{[^}]*display:\s*flex", re.DOTALL
        )

    def test_monster_uses_same_compact_properties_and_keeps_id_technical(self):
        detail = self.script[
            self.script.index("function monsterDetail") : self.script.index(
                "function topMonsterDrops"
            )
        ]
        self.assertIn("monsterPresentation(monster)", detail)
        self.assertIn("gameProperties(presentation", detail)
        monster_presentation = self.script[
            self.script.index("function monsterPresentation") : self.script.index(
                "function itemTechnicalRows"
            )
        ]
        self.assertNotIn("['ID монстра'", monster_presentation)
        technical = self.script[
            self.script.index("function monsterTechnicalRows") : self.script.index(
                "function itemClassBadge"
            )
        ]
        self.assertIn("['ID монстра', formatNumber(monster.id)]", technical)
        self.assertIn("monsterTechnicalRows(monster)", detail)

    def test_set_is_inline_after_slots_and_not_accordion(self):
        detail = self.script[
            self.script.index("function itemDetail") : self.script.index(
                "function effectLabel"
            )
        ]
        self.assertIn("data.set ? setContent(item, data.set) : ''", detail)
        self.assertNotIn("accordion('Комплект'", detail)
        game_block = self.script[
            self.script.index("function gameProperties") : self.script.index(
                "function recipeMaterialsHTML"
            )
        ]
        self.assertLess(
            game_block.index("const slots = cardSlotsRow"),
            game_block.index("if (inlineSet)"),
        )
        self.assertLess(
            game_block.index("if (inlineSet)"),
            game_block.index("presentation.miscStats?.length"),
        )
        self.assertIn("set-member-link", self.script)
        self.assertIn('aria-current="page"', self.script)

    def test_home_has_resources_without_catalog_duplication(self):
        home = self.script[
            self.script.index("function homePage") : self.script.index(
                "function addHistory"
            )
        ]
        self.assertIn("Полезные ссылки", home)
        self.assertIn("Недавно просмотренные", home)
        self.assertIn("Последняя запись ВКонтакте", home)
        self.assertIn("Проверить новую запись", home)
        self.assertIn("The Original и Iris Kiss Kiss: в чём разница?", home)
        self.assertIn("The Original и Iris Kiss Kiss — два сервера Iris Online.", home)
        self.assertIn("После переключения база автоматически показывает", home)
        self.assertIn("The Original — 609 монстров", home)
        self.assertIn("Iris Kiss Kiss — 677 монстров", home)
        self.assertIn("способы получения предметов", home)
        self.assertGreater(home.rfind("${vkNews}"), home.rfind("home-database-status"))
        self.assertIn('class="section-tabs-row"', self.html)
        row = self.html[
            self.html.index('<div class="section-tabs-row">') : self.html.index(
                "</div>", self.html.index('id="checkUpdatesButton"')
            )
            + 6
        ]
        self.assertIn('id="sectionTabs"', row)
        self.assertIn('id="versionStatus"', row)
        self.assertNotIn("контейнер", home.lower())
        self.assertIn("Обсуждения", home)
        self.assertIn("https://vk.ru/board59626511", home)
        self.assertIn("https://github.com/fsibatov/iris-online-database", home)
        self.assertIn("GitHub проекта", home)
        self.assertIn("Сообщества", home)
        self.assertIn("Официальный статус этих площадок не подтверждён", home)
        self.assertNotIn("Быстрый переход", home)
        self.assertNotIn('href="#items"', home)
        self.assertNotIn('href="#monsters"', home)

    def test_version_status_and_vk_news_controls_are_visible(self):
        self.assertIn('id="versionStatus"', self.html)
        self.assertIn('id="checkUpdatesButton"', self.html)
        self.assertIn("Версия 1.1.0", self.html)
        self.assertIn("renderVersionStatus()", self.script)
        self.assertIn("?refresh=1", self.script)
        self.assertIn("/api/community-status", self.script)
        self.assertIn("vk-fallback.svg", self.script)
        self.assertIn("latestPostId", self.script)
        self.assertIn("latestPostText", self.script)
        self.assertIn("function vkNewsPreviewText", self.script)
        self.assertIn("limit = 700", self.script)
        self.assertIn("frame-src 'none'", self.server)

    def test_badges_center_internally_but_item_layout_stays_left_aligned(self):
        self.assertRegex(
            self.styles,
            r"\.rarity-label, \.meta-label\s*\{[^}]*display:\s*inline-flex[^}]*align-items:\s*center[^}]*justify-content:\s*center[^}]*box-sizing:\s*border-box",
            re.DOTALL,
        )
        self.assertRegex(
            self.styles,
            r"\.detail-labels\s*\{[^}]*justify-content:\s*flex-start",
            re.DOTALL,
        )
        self.assertRegex(
            self.styles,
            r"\.detail-heading--item\s*\{[^}]*text-align:\s*left",
            re.DOTALL,
        )
        self.assertRegex(
            self.styles,
            r"\.property-card-slots\s*\{[^}]*justify-content:\s*flex-start",
            re.DOTALL,
        )
        self.assertRegex(
            self.styles,
            r"\.card-slot-chip\s*\{[^}]*display:\s*inline-flex[^}]*align-items:\s*center[^}]*justify-content:\s*center[^}]*box-sizing:\s*border-box",
            re.DOTALL,
        )
        self.assertRegex(
            self.styles, r"\.item-inline-set h2\s*\{[^}]*text-align:\s*left", re.DOTALL
        )
        self.assertRegex(
            self.styles, r"\.set-name\s*\{[^}]*text-align:\s*left", re.DOTALL
        )
        self.assertRegex(
            self.styles, r"\.set-effects h3\s*\{[^}]*text-align:\s*left", re.DOTALL
        )

    def test_sale_price_has_dedicated_game_formatter(self):
        self.assertIn("function formatSalePrice(value)", self.script)
        self.assertIn("toLocaleString('en-US')", self.script)
        self.assertIn("`${number.toLocaleString('en-US')} тер`", self.script)
        self.assertIn("['Цена продажи', formatSalePrice(item.price)]", self.script)
        self.assertNotIn("тер.`", self.script)
        formatter = re.search(
            r"function formatSalePrice\(value\) \{.*?^  \}",
            self.script,
            re.DOTALL | re.MULTILINE,
        )
        self.assertIsNotNone(formatter)
        probe = (
            formatter.group(0)
            + "\nif (formatSalePrice(2460) !== '2,460 тер') process.exit(1);"
        )
        subprocess.run(["node", "-e", probe], check=True)

    def test_recently_viewed_supports_items_and_monsters_in_profile(self):
        self.assertIn("['item', 'monster'].includes(type)", self.script)
        self.assertIn("const key = `${type}:${numericID}:${server}`", self.script)
        self.assertIn("entry.type === 'monster'", self.script)
        self.assertIn("normalizeServerKey(entry.server)", self.script)
        self.assertIn("trackRecentlyViewed('item', item.id, item.name,", self.script)
        self.assertIn(
            "trackRecentlyViewed('monster', monster.id, monster.name,", self.script
        )
        self.assertIn("recent-viewed-list", self.script)
        self.assertNotIn("recent-viewed-card", self.script)
        self.assertIn("recentlyViewed: normalizedRecentViewedEntries()", self.script)
        self.assertIn("profile.recentlyViewed", self.script)
        self.assertIn("localRecentlyViewed", self.script)

    def test_items_have_known_source_filter(self):
        self.assertIn("knownSource: ''", self.script)
        self.assertIn('name="knownSource" type="checkbox" value="1"', self.script)
        self.assertIn("Известно, где получить", self.script)
        self.assertIn(
            "input.type === 'checkbox' ? (input.checked ? '1' : '')", self.script
        )
        self.assertIn("knownSource: 'Известно, где получить'", self.script)
        self.assertIn(".filter-checkbox", self.styles)

    def test_server_switch_refreshes_server_specific_monster_views(self):
        self.assertIn(
            "['home', 'monsters', 'favorites', 'search'].includes(activeRoute)",
            self.script,
        )
        self.assertIn("&server=${encodeURIComponent(state.server)}", self.script)

    def test_home_primary_matches_main_home_width(self):
        end = self.styles.index(".item-inline-set")
        start = self.styles.rfind(".home-page", 0, end)
        self.assertGreaterEqual(start, 0)
        final_home = self.styles[start:end]
        self.assertRegex(
            final_home,
            r"\.home-page\s*\{[^}]*width:\s*min\(100%,\s*980px\)[^}]*max-width:\s*980px",
            re.DOTALL,
        )
        self.assertRegex(
            final_home,
            r"\.home-primary\s*\{[^}]*width:\s*100%[^}]*max-width:\s*none",
            re.DOTALL,
        )
        self.assertRegex(
            final_home, r"\.home-primary\s*\{[^}]*overflow:\s*visible", re.DOTALL
        )
        self.assertNotIn(".home-primary::after", final_home)

    def test_drop_chance_formatter_preserves_small_nonzero_values(self):
        formatter = re.search(
            r"function formatChance\(value\) \{.*?^  \}",
            self.script,
            re.DOTALL | re.MULTILINE,
        )
        odds = re.search(
            r"function formatChanceOdds\(value\) \{.*?^  \}",
            self.script,
            re.DOTALL | re.MULTILINE,
        )
        self.assertIsNotNone(formatter)
        self.assertIsNotNone(odds)
        probe = (
            formatter.group(0)
            + "\n"
            + odds.group(0)
            + """
if (formatChance(0.0833) !== '0,0833%') process.exit(1);
if (formatChance(0.0042) !== '0,0042%') process.exit(2);
if (formatChance(0.0000034986) !== '0,0000035%') process.exit(3);
if (formatChance(0.0000000001) !== '0,0000000001%') process.exit(4);
if (!formatChanceOdds(0.0000034986).includes('28,6')) process.exit(5);
"""
        )
        subprocess.run(["node", "-e", probe], check=True)
        self.assertIn("Шанс группы:", self.script)
        self.assertIn("Если группа выбрана:", self.script)
        self.assertIn("за одну основную попытку:", self.script)
        self.assertNotIn("Вес предмета:", self.script)

    def test_public_ui_has_no_stale_version_label(self):
        combined = self.script + self.html
        legacy_versions = tuple(f"1.0.{patch}" for patch in range(7))
        for stale in ("IrisOnline" + "Preview", "audited", *legacy_versions):
            self.assertNotIn(stale, combined)
        self.assertNotIn(" preview", self.html.lower())
        self.assertIn("const APP_VERSION = '1.1.0'", self.script)
        self.assertIn("Версия 1.1.0", self.html)

    def test_selects_share_control_class(self):
        self.assertIn('class="control-select control-select--server"', self.html)
        for name in (
            "data-catalog-sort",
            'name="category"',
            'name="subcategory"',
            'name="quality"',
            'name="type"',
        ):
            position = self.script.index(name)
            self.assertIn(
                'class="control-select"',
                self.script[max(0, position - 80) : position + 30],
            )
        self.assertIn(".control-select {", self.styles)
        self.assertIn(".control-select--server", self.styles)

    def test_no_new_periodic_or_duplicate_event_mechanisms(self):
        self.assertEqual(self.script.count("setInterval("), 0)
        self.assertEqual(self.script.count("setTimeout("), 10)
        self.assertEqual(self.script.count("addEventListener("), 29)
        self.assertEqual(self.script.count("api("), 18)

    def test_sell_restriction_wording_is_global_for_selltype_zero(self):
        self.assertIn(
            "if (Number(item.sellType) === 0) actions.push({ text: 'Нельзя продать персонажу'",
            self.script,
        )
        self.assertNotIn("Нельзя продать в магазин", self.script)
        self.assertNotIn("Number(item.id) === 808030", self.script)

    def test_item_action_footer_matches_game_order_and_wording(self):
        start = self.script.index(
            "    const actions = [];", self.script.index("function itemPresentation")
        )
        end = self.script.index("    const price =", start)
        block = self.script[start:end]
        expected = [
            "Можно разобрать",
            "Можно перековать",
            "Функция печати (",
            "Запечатать невозможно",
            "Нельзя продать персонажу",
        ]
        positions = [block.index(text) for text in expected]
        self.assertLess(positions[0], positions[1])
        self.assertLess(positions[1], positions[2])
        self.assertLess(positions[2], positions[4])
        self.assertLess(positions[3], positions[4])
        self.assertNotIn("Печать доступна", block)
        self.assertNotIn("Можно запечатать", block)
        self.assertIn(
            "sealCount > 0 ? `Функция печати (${formatNumber(sealCount)})` : 'Запечатать невозможно'",
            block,
        )

    def test_update_notice_does_not_rerender_home_or_destroy_search_input(self):
        start = self.script.index("async function checkForUpdates")
        end = self.script.index("function vkNewsFallbackHTML", start)
        body = self.script[start:end]
        self.assertIn("refreshUpdateNotice()", body)
        self.assertNotIn("homePage()", body)

    def test_update_check_is_visible_bounded_and_non_downloading(self):
        self.assertIn("/api/update-check", self.script)
        self.assertIn("void checkForUpdates()", self.script)
        self.assertIn("Доступна версия", self.script)
        self.assertIn(
            "https://github.com/fsibatov/iris-online-database/releases/latest",
            self.script,
        )
        self.assertNotIn("releases/latest/download", self.script)
        self.assertIn("checkForUpdates({ force: true, notify: true })", self.script)
        block = self.script[
            self.script.index("async function checkForUpdates") : self.script.index(
                "function vkNewsFallbackHTML"
            )
        ]
        self.assertIn("refreshUpdateNotice()", block)
        self.assertNotIn("homePage()", block)

    def test_old_gift_chest_does_not_require_id_special_case(self):
        self.assertIn(
            "if (Number(item.sellType) === 0) actions.push({ text: 'Нельзя продать персонажу'",
            self.script,
        )
        self.assertNotIn("Number(item.id) === 808030", self.script)

    def test_chest_contents_and_sources_are_visible_on_both_sides(self):
        self.assertIn("function chestContentsHTML(chest)", self.script)
        self.assertIn("Содержимое сундука", self.script)
        self.assertIn("Шанс при открытии", self.script)
        self.assertIn("data.chest", self.script)
        self.assertIn("drop.containerId", self.script)
        self.assertIn("{ title: 'Сундуки', sources: ['Сундук'] }", self.script)
        self.assertIn(".chest-content-list", self.styles)

    def test_world_sources_expand_lazily_without_claiming_map_mapping(self):
        self.assertIn("data-world-source", self.script)
        self.assertIn("renderWorldSourceMonsters(details)", self.script)
        self.assertIn("/api/world-source-monsters", self.script)
        self.assertIn("data?.contextMatchKnown !== false", self.script)
        self.assertIn(
            "нет подтверждённой связи конкретного монстра с типом карты", self.script
        )

    def test_monster_world_drops_expand_lazily_without_claiming_location_mapping(self):
        self.assertIn("lazy-monster-world-drops", self.script)
        self.assertIn("renderMonsterWorldDropShell()", self.script)
        self.assertIn("/api/monster-world-drops", self.script)
        self.assertIn("по уровню и типу", self.script)
        self.assertIn("тип локации", self.script)
        self.assertIn("choicePosition", self.script)
        self.assertIn(".world-monster-list", self.styles)

    def test_sources_use_requested_section_order(self):
        block = self.script[
            self.script.index("function buildSourceSections") : self.script.index(
                "function chestSourceDetails"
            )
        ]
        labels = [
            "Монстры с подтверждённым выпадением",
            "Мировая добыча",
            "Сундуки",
            "Задания",
        ]
        positions = [block.index(label) for label in labels]
        self.assertEqual(positions, sorted(positions))
        self.assertNotIn(
            "sort((a, b) => baseAttemptChance(b) - baseAttemptChance(a))", self.script
        )

    def test_about_shows_author_and_github(self):
        about = self.script[
            self.script.index("if (type === 'about')") : self.script.index(
                "} else if (type === 'data')"
            )
        ]
        self.assertIn("Хоуп (The Original)", about)
        self.assertIn("https://github.com/fsibatov/iris-online-database", about)

    def test_descriptions_remain_conditional(self):
        self.assertIn("meaningfulDescription(item.tooltip, item.name)", self.script)
        self.assertIn("meaningfulDescription(monster.note, monster.name)", self.script)
        self.assertGreaterEqual(
            self.script.count("description ? accordion('Описание'"), 2
        )
        self.assertIn("function normalizeDisplayText(value)", self.script)
        self.assertIn("function multilineHTML(value)", self.script)
        self.assertGreaterEqual(self.script.count("multilineHTML(description)"), 2)
        self.assertIn("const rawText = normalizeDisplayText(active.text);", self.script)

    def test_recipe_catalog_section(self):
        self.assertIn(
            "{ route: 'recipes', label: 'Рецепты', icon: 'recipe' }", self.script
        )
        self.assertIn("function defaultRecipeFilters()", self.script)
        self.assertIn("function recipeRow(recipe, query = '')", self.script)
        self.assertIn(
            "path === 'items' || path === 'monsters' || path === 'recipes'", self.script
        )
        self.assertIn("path.startsWith('recipe/')", self.script)
        self.assertIn("Рецепты Iris Online и материалы для изготовления.", self.script)
        self.assertIn(
            "knownSource: ''",
            self.script[
                self.script.index(
                    "function defaultRecipeFilters()"
                ) : self.script.index("function resetTransientCatalogFilters()")
            ],
        )
        self.assertIn(
            "Только рецепты, для которых в выбранной базе указан источник получения.",
            self.script,
        )
        self.assertIn("recipe-source-preview", self.script)
        self.assertIn("recipe-source-preview", self.styles)
        self.assertIn("По уровню мастерства", self.script)
        self.assertIn("Уровень мастерства от", self.script)
        self.assertIn("Каллиграф", self.script)
        self.assertIn("recipe-material-label", self.script)
        self.assertIn("grid-template-columns: 18px minmax(0, 1fr);", self.styles)
        self.assertIn("single-source-block", self.script)
        self.assertIn("drops.length === 1", self.script)
        self.assertIn("grid-template-columns: repeat(5, minmax(0, 1fr));", self.styles)


if __name__ == "__main__":
    unittest.main()
