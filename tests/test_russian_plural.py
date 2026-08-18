import json
import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
APP = (ROOT / "web" / "app.js").read_text(encoding="utf-8")


def extract_function(name: str) -> str:
    match = re.search(
        rf"  function {re.escape(name)}\([^\n]*\) \{{\n(?:.*\n)*?  \}}", APP
    )
    if not match:
        raise AssertionError(f"function {name} not found")
    return match.group(0).replace("  function", "function", 1)


class RussianPluralTests(unittest.TestCase):
    def test_numeric_declensions(self):
        plural = extract_function("russianPlural")
        cases = {
            0: "предметов",
            1: "предмет",
            2: "предмета",
            4: "предмета",
            5: "предметов",
            11: "предметов",
            12: "предметов",
            14: "предметов",
            20: "предметов",
            21: "предмет",
            22: "предмета",
            24: "предмета",
            25: "предметов",
            101: "предмет",
            111: "предметов",
            112: "предметов",
            114: "предметов",
            121: "предмет",
            122: "предмета",
            125: "предметов",
        }
        script = (
            plural
            + "\n"
            + f"console.log(JSON.stringify(Object.fromEntries({json.dumps(list(cases.keys()))}.map(n => [n, russianPlural(n, 'предмет', 'предмета', 'предметов')]))));"
        )
        result = subprocess.run(
            ["node", "-e", script],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
        actual = {int(k): v for k, v in json.loads(result.stdout).items()}
        self.assertEqual(actual, cases)

    def test_node_output_is_decoded_as_utf8(self):
        source = Path(__file__).read_text(encoding="utf-8")
        self.assertIn('encoding="utf-8"', source)

    def test_all_visible_count_labels_use_shared_helper(self):
        self.assertNotIn("group.required === 1", APP)
        self.assertNotIn("${slots.length} вариантов", APP)
        self.assertNotIn("${formatNumber(drops.length)} вариантов", APP)
        self.assertNotIn("предметов: ${formatNumber(count)}", APP)
        for forms in (
            "'предмет', 'предмета', 'предметов'",
            "'вариант', 'варианта', 'вариантов'",
            "'источник', 'источника', 'источников'",
            "'запись', 'записи', 'записей'",
            "'дополнительная попытка', 'дополнительные попытки', 'дополнительных попыток'",
        ):
            self.assertIn(forms, APP)


if __name__ == "__main__":
    unittest.main()
