from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
path = ROOT / "update_check_test.go"
text = path.read_text(encoding="utf-8")
old_server = '\t\tfmt.Fprintln(w, `{"tag_name":"v2.0.1"}`)'
new_server = '\t\tfmt.Fprintf(w, `{"tag_name":"v%s"}\\n`, appVersion)'
old_assert = '\tif !strings.Contains(rec.Body.String(), `"latestVersion":"2.0.1"`) || !strings.Contains(rec.Body.String(), `"updateAvailable":true`) {'
new_assert = '\tif !strings.Contains(rec.Body.String(), `"latestVersion":"`+appVersion+`"`) || !strings.Contains(rec.Body.String(), `"updateAvailable":false`) {'
for old, new in ((old_server, new_server), (old_assert, new_assert)):
    if text.count(old) != 1:
        raise SystemExit(f"expected exactly one test marker, found {text.count(old)}: {old}")
    text = text.replace(old, new, 1)
path.write_text(text, encoding="utf-8")
Path(__file__).unlink()
