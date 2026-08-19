from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

interface = ROOT / "tests/test_interface_details.py"
text = interface.read_text(encoding="utf-8")
old_setup = '''        cls.server = (ROOT / "server.go").read_text(encoding="utf-8")
'''
new_setup = '''        cls.server = (ROOT / "server.go").read_text(encoding="utf-8")
        cls.version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
'''
if text.count(old_setup) != 1:
    raise SystemExit(f"interface setup marker count={text.count(old_setup)}")
text = text.replace(old_setup, new_setup, 1)
old_visible = '        self.assertIn("Версия 2.0.0", self.html)'
new_visible = '        self.assertIn(f"Версия {self.version}", self.html)'
if text.count(old_visible) != 2:
    raise SystemExit(f"interface visible version marker count={text.count(old_visible)}")
text = text.replace(old_visible, new_visible)
old_app = '        self.assertIn("const APP_VERSION = \'2.0.0\'", self.script)'
new_app = '        self.assertIn(f"const APP_VERSION = \'{self.version}\'", self.script)'
if text.count(old_app) != 1:
    raise SystemExit(f"interface app version marker count={text.count(old_app)}")
text = text.replace(old_app, new_app, 1)
interface.write_text(text, encoding="utf-8")

helpers = ROOT / "tests/test_release_helpers.py"
text = helpers.read_text(encoding="utf-8")
old_block = '''    def test_version_is_coherent_across_runtime_and_release_metadata(self):
        self.assertEqual(
            (ROOT / "VERSION").read_text(encoding="utf-8").strip(), "2.0.0"
        )
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        html = (ROOT / "web" / "index.html").read_text(encoding="utf-8")
        script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        wails = json.loads((ROOT / "wails.json").read_text(encoding="utf-8"))
        resources = json.loads(
            (ROOT / "build" / "windows" / "info.json").read_text(encoding="utf-8")
        )
        self.assertIn('var appVersion = "2.0.0"', server)
        self.assertIn("Версия 2.0.0", html)
        self.assertIn("const APP_VERSION = '2.0.0'", script)
        self.assertEqual(wails["info"]["productVersion"], "2.0.0")
        self.assertEqual(resources["fixed"]["product_version"], "2.0.0.0")
        self.assertEqual(resources["info"]["0419"]["ProductVersion"], "2.0.0")
'''
new_block = '''    def test_version_is_coherent_across_runtime_and_release_metadata(self):
        version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
        self.assertRegex(version, r"^\\d+\\.\\d+\\.\\d+$")
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        html = (ROOT / "web" / "index.html").read_text(encoding="utf-8")
        script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        wails = json.loads((ROOT / "wails.json").read_text(encoding="utf-8"))
        resources = json.loads(
            (ROOT / "build" / "windows" / "info.json").read_text(encoding="utf-8")
        )
        self.assertIn(f'var appVersion = "{version}"', server)
        self.assertIn(f"Версия {version}", html)
        self.assertIn(f"const APP_VERSION = '{version}'", script)
        self.assertEqual(wails["info"]["productVersion"], version)
        self.assertEqual(resources["fixed"]["product_version"], f"{version}.0")
        self.assertEqual(resources["info"]["0419"]["ProductVersion"], version)
'''
if text.count(old_block) != 1:
    raise SystemExit(f"release helper current-version block count={text.count(old_block)}")
helpers.write_text(text.replace(old_block, new_block, 1), encoding="utf-8")

Path(__file__).unlink()
