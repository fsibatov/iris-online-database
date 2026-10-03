import gzip
import json
import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class Version205HeaderRaritySortTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        cls.html = (ROOT / "web" / "index.html").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web" / "styles.css").read_text(encoding="utf-8")
        cls.main_go = (ROOT / "main.go").read_text(encoding="utf-8")

    def test_battleground_timer_shows_the_event_and_countdown(self):
        self.assertIn('id="battlegroundStatus"', self.html)
        self.assertIn('id="battlegroundCountdown"', self.html)
        self.assertIn('id="battlegroundName"', self.html)
        self.assertNotIn('id="battlegroundStart"', self.html)
        self.assertIn('role="timer" aria-live="off"', self.html)
        self.assertIn("const BATTLEGROUND_INTERVAL_MS = 30 * 60 * 1000;", self.script)
        self.assertIn("const BATTLEGROUND_FIRST_START_MS = 3 * 60 * 1000;", self.script)
        timer_markup = self.html.split('id="battlegroundStatus"', 1)[1].split(
            "</div>", 1
        )[0]
        self.assertNotRegex(timer_markup, r"БГ|UTC")
        self.assertIn(
            "window.setTimeout(scheduleBattlegroundStatus, 1000 - (Date.now() % 1000))",
            self.script,
        )

    def test_battleground_schedule_matches_all_48_daily_slots(self):
        constants = "\n".join(
            line.strip()
            for line in self.script.splitlines()
            if line.strip().startswith("const BATTLEGROUND_")
        )
        start = self.script.index("function battlegroundState")
        end = self.script.index("function renderBattlegroundStatus", start)
        function = self.script[start:end]
        timestamps = []
        expected = []
        for slot in range(48):
            minute_of_day = 3 + slot * 30
            hour, minute = divmod(minute_of_day, 60)
            server_iso = f"2026-09-04T{hour:02d}:{minute:02d}:00+03:00"
            timestamps.append(server_iso)
            expected.append(
                {
                    "name": ("Противостояние", "Захват флага", "Горнило")[slot % 3],
                    "countdown": "00:00",
                }
            )
        probe = (
            constants
            + "\n"
            + function
            + "\nconsole.log(JSON.stringify("
            + json.dumps(timestamps, ensure_ascii=False)
            + ".map(value => battlegroundState(new Date(value)))));"
        )
        completed = subprocess.run(
            ["node", "-e", probe],
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
        self.assertEqual(json.loads(completed.stdout), expected)

    def test_battleground_schedule_boundaries_and_day_rollover(self):
        constants = "\n".join(
            line.strip()
            for line in self.script.splitlines()
            if line.strip().startswith("const BATTLEGROUND_")
        )
        start = self.script.index("function battlegroundState")
        end = self.script.index("function renderBattlegroundStatus", start)
        function = self.script[start:end]
        timestamps = [
            "2026-09-03T21:02:00Z",
            "2026-09-03T21:03:01Z",
            "2026-09-04T20:33:00Z",
            "2026-09-04T20:33:01Z",
        ]
        probe = (
            constants
            + "\n"
            + function
            + "\nconsole.log(JSON.stringify("
            + json.dumps(timestamps, ensure_ascii=False)
            + ".map(value => battlegroundState(new Date(value)))));"
        )
        completed = subprocess.run(
            ["node", "-e", probe],
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
        self.assertEqual(
            json.loads(completed.stdout),
            [
                {"name": "Противостояние", "countdown": "01:00"},
                {"name": "Захват флага", "countdown": "29:59"},
                {"name": "Горнило", "countdown": "00:00"},
                {"name": "Противостояние", "countdown": "29:59"},
            ],
        )

    def test_rarity_labels_are_masculine_and_magic_color_is_exact(self):
        expected_by_id = {
            0: "Покупной",
            1: "Низкий",
            2: "Обычный",
            3: "Магический",
            4: "Редкий",
            5: "Уникальный",
            6: "PvP",
            7: "Эпический",
            8: "Особый",
            9: "Ивентовый",
        }
        for quality_id, label in expected_by_id.items():
            self.assertIn(f"[{quality_id}, '{label}']", self.script)
        for source, label in {
            "не указано": "Покупной",
            "низкое": "Низкий",
            "обычное": "Обычный",
            "необычное": "Магический",
            "редкое": "Редкий",
            "уникальное": "Уникальный",
            "pvp": "PvP",
            "эпическое": "Эпический",
            "особое": "Особый",
            "событийное": "Ивентовый",
        }.items():
            self.assertIn(f"['{source}', '{label}']", self.script)
        self.assertIn("if (id === 3) return 'quality-magic';", self.script)
        self.assertRegex(
            self.styles, r"\.rarity-label\.quality-magic\s*\{[^}]*color:\s*#00ff00\s*;"
        )

    def test_embedded_rarity_ids_have_one_source_label_each(self):
        with gzip.open(
            ROOT / "assets" / "game_data.json.gz", "rt", encoding="utf-8"
        ) as source:
            game = json.load(source)
        labels_by_id = {}
        for item in game["items"]:
            labels_by_id.setdefault(int(item.get("qualityId") or 0), set()).add(
                str(item.get("quality") or "").strip()
            )
        conflicts = {
            quality_id: labels
            for quality_id, labels in labels_by_id.items()
            if len(labels) != 1
        }
        self.assertEqual(conflicts, {})
        self.assertEqual(
            labels_by_id,
            {
                0: {"Не указано"},
                1: {"Низкое"},
                2: {"Обычное"},
                3: {"Необычное"},
                4: {"Редкое"},
                5: {"Уникальное"},
                6: {"PvP"},
                7: {"Эпическое"},
                8: {"Особое"},
                9: {"Событийное"},
            },
        )

    def test_catalog_sort_order_defaults_to_ascending(self):
        defaults = re.findall(
            r"function default(?:Item|Monster|Recipe|Title|Transformation)Filters\(\) \{\n    return \{([^}]+)\};",
            self.script,
        )
        self.assertEqual(len(defaults), 5)
        self.assertTrue(all("order: 'asc'" in block for block in defaults))
        self.assertIn(
            "const options = [['asc', 'По возрастанию'], ['desc', 'По убыванию']];",
            self.script,
        )
        self.assertIn("order: filters.order || 'asc'", self.script)
        sort_order = self.main_go[
            self.main_go.index("func querySortOrder") : self.main_go.index(
                "func orderedIntLess"
            )
        ]
        self.assertIn('return "asc", true', sort_order)
        self.assertNotIn('return "desc", true\n\t}', sort_order)

    def test_server_change_does_not_steal_focus_into_home_search(self):
        home_start = self.script.index("function homePage()")
        home = self.script[
            home_start : self.script.index("function addHistory", home_start)
        ]
        self.assertNotIn("globalSearch.focus", home)
        server_start = self.script.index("async function changeServer()")
        server_change = self.script[
            server_start : self.script.index(
                "moreButton.addEventListener", server_start
            )
        ]
        self.assertIn(
            "if (state.server === nextServer && !applicationClosing) serverSelect.focus({ preventScroll: true });",
            server_change,
        )

    def test_header_statuses_reserve_space_and_keep_accessible_labels(self):
        for selector in (".battleground-status", ".version-status"):
            rules = re.search(rf"{re.escape(selector)}\s*\{{([^}}]*)\}}", self.styles)
            self.assertIsNotNone(rules)
            self.assertRegex(rules[1], r"\bwidth:\s*\d+px;")
        self.assertNotIn(".version-status-text { display: none; }", self.styles)
        version_text = re.search(r"\.version-status-text\s*\{([^}]*)\}", self.styles)
        self.assertIsNotNone(version_text)
        self.assertNotRegex(version_text[1], r"overflow|height|ellipsis")
        self.assertIn("versionStatus.setAttribute('aria-label', label);", self.script)
        self.assertNotIn("versionStatus.title", self.script)
        self.assertNotIn("battlegroundStatus.title", self.script)
        self.assertIn("${next.name}: до начала ${next.countdown}", self.script)

    def test_header_status_pills_share_one_visual_component(self):
        self.assertIn('class="header-status-pill battleground-status"', self.html)
        self.assertIn('class="header-status-pill version-status"', self.html)
        shared = self.styles[
            self.styles.index(".header-status-pill {") : self.styles.index(
                ".battleground-status {", self.styles.index(".header-status-pill {")
            )
        ]
        self.assertIn("min-height: var(--header-status-height);", shared)
        self.assertIn("--header-status-height: 34px;", self.styles)
        self.assertIn("display: inline-flex;", shared)
        self.assertIn("padding: 3px 9px;", shared)
        self.assertIn("line-height: 18px;", shared)
        self.assertIn("border: 1px solid var(--border);", shared)
        self.assertIn("border-radius: 999px;", shared)
        self.assertIn("background: var(--surface-2);", shared)
        self.assertNotRegex(self.styles, r"\.battleground-status\s*\{[^}]*height:")
        self.assertNotRegex(self.styles, r"\.version-status\s*\{[^}]*height:")

    def test_topbar_controls_use_one_geometry(self):
        self.assertIn("--header-control-height: 44px;", self.styles)
        self.assertIn("min-height: var(--header-control-height);", self.styles)
        self.assertIn("width: var(--header-control-height);", self.styles)
        server_start = self.styles.index(".control-select--server {")
        server_end = self.styles.index("}", server_start)
        server = self.styles[server_start:server_end]
        self.assertIn("min-height: var(--header-control-height);", server)
        self.assertIn("border-color: var(--border);", server)
        self.assertIn("border-radius: var(--radius);", server)

    def test_data_freshness_disclaimer_is_explicit_and_consistent(self):
        disclaimer = (
            "данные встроены в приложение и не синхронизируются автоматически "
            "с игровыми серверами Iris Online"
        )
        self.assertIn(disclaimer, self.script)
        self.assertIn(
            "После обновлений игры сведения могут временно отличаться", self.script
        )
        self.assertIn("Даты обновления и ограничения", self.html)
        self.assertIn(".data-sync-note {", self.styles)


if __name__ == "__main__":
    unittest.main()
