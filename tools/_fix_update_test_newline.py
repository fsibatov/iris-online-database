from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
path = ROOT / "update_check_test.go"
text = path.read_text(encoding="utf-8")
old = '\t\tfmt.Fprintf(w, `{"tag_name":"v%s"}\\n`, appVersion)'
new = '\t\tfmt.Fprintf(w, `{"tag_name":"v%s"}`+"\\n", appVersion)'
if text.count(old) != 1:
    raise SystemExit(f"expected exactly one mock JSON marker, found {text.count(old)}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")
Path(__file__).unlink()
