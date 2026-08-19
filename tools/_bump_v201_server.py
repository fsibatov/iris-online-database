from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
server = ROOT / "server.go"
text = server.read_text(encoding="utf-8")
old = 'var appVersion = "2.0.0"'
new = 'var appVersion = "2.0.1"'
if text.count(old) != 1:
    raise SystemExit(f"expected exactly one backend version marker, found {text.count(old)}")
server.write_text(text.replace(old, new, 1), encoding="utf-8")
Path(__file__).unlink()
