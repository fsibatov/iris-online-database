from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
path = ROOT / "tools/frontend_smoke_test.py"
text = path.read_text(encoding="utf-8")

root_marker = 'ROOT = Path(__file__).resolve().parents[1]\nWEB_ROOT = ROOT / "web"\n'
root_replacement = 'ROOT = Path(__file__).resolve().parents[1]\nCURRENT_VERSION = (ROOT / "VERSION").read_text(encoding="utf-8").strip()\nWEB_ROOT = ROOT / "web"\n'
if text.count(root_marker) != 1:
    raise SystemExit(f"frontend smoke ROOT marker count={text.count(root_marker)}")
text = text.replace(root_marker, root_replacement, 1)

replacements = (
    ('"currentVersion": "2.0.0"', '"currentVersion": CURRENT_VERSION'),
    ('"latestVersion": "2.0.0"', '"latestVersion": CURRENT_VERSION'),
    ('== "Версия 2.0.0"', '== f"Версия {CURRENT_VERSION}"'),
)
for old, new in replacements:
    if text.count(old) != 1:
        raise SystemExit(f"frontend smoke marker count={text.count(old)}: {old}")
    text = text.replace(old, new, 1)

path.write_text(text, encoding="utf-8")
Path(__file__).unlink()
