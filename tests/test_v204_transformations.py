from __future__ import annotations

import gzip
import importlib.util
import json
import math
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

SPEC = importlib.util.spec_from_file_location(
    "build_transformation_cards_asset",
    ROOT / "tools/build_transformation_cards_asset.py",
)
BUILDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILDER)


def read_gzip_json(path: Path) -> dict:
    with gzip.open(path, "rt", encoding="utf-8") as source:
        return json.load(source)


class V204TransformationRegressionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.asset = read_gzip_json(ROOT / "assets/transformation_cards.json.gz")
        cls.cards = cls.asset["cards"]
        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")
        cls.transformations_go = (ROOT / "transformations.go").read_text(
            encoding="utf-8"
        )
        cls.storage_go = (ROOT / "storage.go").read_text(encoding="utf-8")
        cls.main_go = (ROOT / "main.go").read_text(encoding="utf-8")
        cls.enhancements_go = (ROOT / "enhancements.go").read_text(encoding="utf-8")

    def all_skills(self):
        for card in self.cards:
            for skill in card.get("skills", []):
                yield card, skill

    def test_mushroom_ally_buff_exposes_size_percent(self):
        matches = [
            (card, skill)
            for card, skill in self.all_skills()
            if skill.get("name") == "Впитываемость гриба"
            and skill.get("effectText") == "50% к росту"
        ]
        self.assertTrue(matches)
        self.assertTrue(
            any(
                card.get("name") == "Карта превращения спорного гриба"
                for card, _ in matches
            )
        )
        for _, skill in matches:
            self.assertEqual(skill.get("applyType"), 3)
            self.assertTrue(skill.get("isFriendlyBuff"))
            self.assertIn(
                {
                    "name": "Размер",
                    "value": 50,
                    "percent": True,
                    "positive": True,
                    "text": "50% к росту",
                },
                skill.get("characteristics", []),
            )

    def test_poison_immunity_variants_have_one_canonical_label(self):
        self.assertEqual(
            BUILDER.characteristics("Иммунитет к яду")[0]["name"], "Иммунитет к ядам"
        )
        self.assertEqual(
            BUILDER.characteristics("Иммунитет к ядам")[0]["name"], "Иммунитет к ядам"
        )
        self.assertIn('"Иммунитет к ядам"', self.transformations_go)
        self.assertNotIn('return "Иммунитет к яду"', self.transformations_go)

    def test_filter_characteristics_have_no_tooltip_noise_or_broken_brackets(self):
        forbidden = (
            "во время превращения",
            "общий / подтип",
            "число основной атаки",
            "от максимума",
            "пока ваши ом",
            "расходует ману вместо здоровья",
            "превращение критика",
            "npc превращение",
            "[увеличение]",
        )
        names: set[str] = set()
        for card in self.cards:
            for row in card.get("formCharacteristics", []):
                if row.get("positive"):
                    names.add(str(row.get("name") or ""))
            for skill in card.get("skills", []):
                if not (skill.get("isFriendlyBuff") or skill.get("isSelfBuff")):
                    continue
                for row in skill.get("characteristics", []):
                    if row.get("positive"):
                        names.add(str(row.get("name") or ""))
        self.assertTrue(names)
        for name in names:
            low = name.casefold()
            self.assertNotIn("[", name)
            self.assertNotIn("]", name)
            for fragment in forbidden:
                self.assertNotIn(fragment, low, name)

    def test_ally_target_filter_has_real_source_rows(self):
        ally_cards = {
            card["itemId"]
            for card in self.cards
            if any(skill.get("applyType") == 3 for skill in card.get("skills", []))
        }
        self.assertEqual(len(ally_cards), 33)
        self.assertIn(1070014, ally_cards)
        self.assertIn("func transformationHasAllySkill", self.transformations_go)
        self.assertIn(
            'ally == "1" && !transformationHasAllySkill(card)', self.transformations_go
        )
        self.assertIn('name="ally" type="checkbox" value="1"', self.script)
        self.assertIn("Цель — союзник", self.script)

    def test_useful_ally_buffs_are_not_lost(self):
        expected = {
            "Впитываемость гриба": {"Размер"},
            "Дразнить": {"Меткость", "Физическая меткость"},
            "Защита тьмы": {"Выносливость", "Максимум здоровья"},
            "Катать снежки": {"Физическая защита"},
            "Остановить духа": {"Магический урон"},
            "Поток очищения": {"Снятие отрицательных эффектов"},
            "Штурм стены": {"Иммунитет к обездвиживанию"},
        }
        found: dict[str, set[str]] = {name: set() for name in expected}
        for _, skill in self.all_skills():
            name = skill.get("name")
            if name not in expected or skill.get("applyType") != 3:
                continue
            self.assertTrue(skill.get("isFriendlyBuff"), name)
            found[name].update(
                row.get("name")
                for row in skill.get("characteristics", [])
                if row.get("name")
            )
        for skill_name, characteristics in expected.items():
            self.assertTrue(characteristics <= found[skill_name], skill_name)

    def test_enemy_target_skills_are_never_marked_as_friendly_buffs(self):
        enemy = [skill for _, skill in self.all_skills() if skill.get("applyType") == 2]
        self.assertTrue(enemy)
        self.assertFalse(any(skill.get("isFriendlyBuff") for skill in enemy))
        self.assertIn(
            "skill.IsFriendlyBuff || skill.IsSelfBuff", self.transformations_go
        )

    def test_semantic_aliases_remain_mechanically_specific(self):
        cases = {
            "Иммунитет к обездвижению": "Иммунитет к обездвиживанию",
            "Снятие негативных эффектов": "Снятие отрицательных эффектов",
            "Насыщение маг. разрушений +20%": "Магическое поглощение",
            "[Защита от понижения скорости]": "Защита от снижения скорости",
        }
        for source, expected in cases.items():
            with self.subTest(source=source):
                rows = BUILDER.characteristics(source)
                self.assertTrue(rows)
                self.assertEqual(rows[0]["name"], expected)

    def test_catalog_cards_show_deduplicated_skill_statuses(self):
        self.assertIn("skillStatuses", self.transformations_go)
        self.assertIn("function transformationStatusHTML(statuses)", self.script)
        self.assertIn("Бафф", self.script)
        self.assertIn("Эффект", self.script)
        self.assertIn("Дебафф", self.script)
        self.assertIn(".transformation-status--effect", self.styles)
        self.assertIn("new Set(rows.filter", self.script)
        self.assertIn(
            "ещё ${formatCount(more, 'эффект', 'эффекта', 'эффектов')}", self.script
        )
        self.assertNotIn("` +${more}`", self.script)
        self.assertIn("transformationStatusHTML(card.skillStatuses)", self.script)

    def test_periodic_status_wording_is_canonicalized_for_catalog_summary(self):
        self.assertIn(
            'case "оз каждую секунду", "оз каждые секунды":', self.transformations_go
        )
        self.assertIn('return "Потеря здоровья"', self.transformations_go)
        self.assertIn(
            'case "ом каждую секунду", "ом каждые секунды":', self.transformations_go
        )
        self.assertIn('return "Потеря маны"', self.transformations_go)
        self.assertIn('return "Потеря здоровья и маны"', self.transformations_go)

    def test_recently_viewed_has_safe_transformation_icon_and_recipe_type(self):
        self.assertIn("function recentViewedTypeIcon(type)", self.script)
        self.assertIn("type === 'transformation' ? icons.transform", self.script)
        self.assertNotIn("${icons[entry.type]}", self.script)
        self.assertNotIn("${icons[type]}", self.script)
        self.assertIn("'recipe'", self.script)
        self.assertIn('entry.Type != "recipe"', self.storage_go)

    def test_zero_value_never_renders_as_false_plus_zero(self):
        block = self.script[
            self.script.index("function transformationRow") : self.script.index(
                "function itemRow"
            )
        ]
        self.assertIn("selectedValue !== 0", block)
        enhancement = self.script[
            self.script.index("function enhancementBonusText") : self.script.index(
                "function itemEnhancementPrefixHTML"
            )
        ]
        self.assertIn("if (!bonus) return '';", enhancement)

    def test_transformations_use_distinct_favorite_namespace_and_persist(self):
        self.assertIn("`transformation:${Number(card.id)}`", self.script)
        self.assertIn("`transformation:${id}`", self.script)
        self.assertIn('data-favorite="${key}"', self.script)
        self.assertIn('case "transformation":', self.main_go)
        self.assertIn('fmt.Sprintf("transformation:%d", id)', self.main_go)
        self.assertIn('parts[0] != "transformation"', self.storage_go)
        self.assertIn('entry.Type != "transformation"', self.storage_go)
        self.assertIn("isTransformationItem(id)", self.main_go)
        self.assertIn("migratedKeys[key] = canonical", self.main_go)

    def test_transformation_star_layout_is_mobile_safe(self):
        self.assertIn("64px minmax(0, 1fr) 52px", self.styles)
        self.assertIn("50px minmax(0, 1fr) 46px", self.styles)
        self.assertIn("44px minmax(0, 1fr) 44px", self.styles)


class V204EnhancementRegressionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.game = read_gzip_json(ROOT / "assets/game_data.json.gz")
        cls.abilities = read_gzip_json(ROOT / "assets/item_abilities.json.gz")
        cls.enhancements = read_gzip_json(ROOT / "assets/item_enhancements.json.gz")
        cls.recipes = read_gzip_json(ROOT / "assets/item_recipes.json.gz")
        cls.script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web/styles.css").read_text(encoding="utf-8")
        cls.go = (ROOT / "enhancements.go").read_text(encoding="utf-8")
        cls.main_go = (ROOT / "main.go").read_text(encoding="utf-8")

    def test_screenshot_staff_plus_two_matches_client_math(self):
        item = next(row for row in self.game["items"] if row.get("id") == 81407)
        patch = self.abilities["items"]["81407"]
        self.assertEqual(item["magicMin"], 670)
        self.assertEqual(item["magicMax"], 745)
        self.assertEqual(item["heal"], 320)
        self.assertEqual(patch["enhancedIndex"], 13)
        profile = self.enhancements["profiles"]["13"]
        plus_two = {row["type"]: row["values"][1] for row in profile}
        self.assertEqual(math.trunc(item["magicMax"] * plus_two[1202] / 100), 149)
        self.assertEqual(math.trunc(item["heal"] * plus_two[1220] / 100), 64)

    def test_attack_bonus_uses_maximum_base_and_defense_heal_use_own_base(self):
        self.assertIn(
            'return "Физическая атака", item.PhysicalMin, item.PhysicalMax, item.PhysicalMax, true',
            self.go,
        )
        self.assertIn(
            'return "Магическая атака", item.MagicMin, item.MagicMax, item.MagicMax, true',
            self.go,
        )
        self.assertIn(
            'return "Физическая защита", 0, 0, item.PhysicalDefense, false', self.go
        )
        self.assertIn('return "Лечение", 0, 0, item.Heal, false', self.go)
        self.assertIn("math.Trunc(float64(base) * percent / 100.0)", self.go)

    def test_item_usage_skill_matches_screenshot_and_is_shown_after_weight(self):
        self.assertEqual(self.recipes["schemaVersion"], 2)
        self.assertEqual(self.recipes["usedSkills"]["81407"], [4])
        self.assertIn('"usedSkills": store.itemUsedSkills[id]', self.main_go)
        self.assertIn("itemPresentation(item, bonuses, data.usedSkills)", self.script)
        weight = self.script.index("['Вес', hasPositiveStat(item.weight)")
        skill = self.script.index("'Используемое умение'")
        rank = self.script.index("['Ранг', itemRankRequirement(item)]")
        self.assertLess(weight, skill)
        self.assertLess(skill, rank)
        self.assertIn("Ремесленник", self.script)
        self.assertIn("от ${formatNumber(minimum)} и выше", self.script)
        item = next(row for row in self.game["items"] if row.get("id") == 81407)
        self.assertEqual(item.get("seal"), 54)
        self.assertIn("Функция печати (5 раз)", self.script)
        self.assertIn("Функция печати (без ограничений)", self.script)
        self.assertIn("Требуется печатей: ${formatNumber(sealSet)}", self.script)

    def test_enhancement_is_compact_title_prefix_and_plus_zero_is_quiet(self):
        self.assertIn("function applyEnhancementLevel(enhancement, level)", self.script)
        self.assertIn("function itemEnhancementPrefixHTML(enhancement)", self.script)
        self.assertIn('class="enhancement-prefix"', self.script)
        self.assertIn('aria-label="Уровень усиления предмета"', self.script)
        self.assertIn(
            ".item-title-line > h1:first-child { grid-column: 1 / -1; }", self.styles
        )
        self.assertIn("align-items: center;", self.styles)
        self.assertNotIn(
            ".enhancement-prefix { align-self: start; margin-top: 2px; }", self.styles
        )
        self.assertIn("width: 80px;", self.styles)
        self.assertIn("min-width: 80px;", self.styles)
        self.assertIn("font-variant-numeric: tabular-nums;", self.styles)
        self.assertIn("grid-template-columns: minmax(0, 1fr);", self.styles)
        self.assertIn("enhancement-inline-bonus", self.script)
        self.assertIn("selectedLevel > 0", self.script)
        self.assertIn(
            "function enhancementSourceNoteHTML(enhancement, drops)", self.script
        )
        self.assertIn("Обычные источники относятся к предмету +0", self.script)
        self.assertIn("уровень явно указан у сундука", self.script)
        self.assertIn("data-enhancement-source-note hidden", self.script)
        self.assertNotIn("item-enhancement", self.script)
        self.assertNotIn("enhancement-note", self.script)
        self.assertNotIn("data-enhancement-result", self.script)
        self.assertNotIn("enhancement-level-summary", self.script)


if __name__ == "__main__":
    unittest.main()
