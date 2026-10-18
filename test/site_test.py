import os
from pathlib import Path
import subprocess
import tempfile
import unittest
import xml.etree.ElementTree as ET
from html.parser import HTMLParser
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[1]


class Links(HTMLParser):
    def __init__(self, text):
        super().__init__()
        self.targets = []
        self.ids = set()
        self.feed(text)

    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        if "id" in values:
            self.ids.add(values["id"])
        for key in ("href", "src"):
            if key in values:
                self.targets.append(values[key])


class SiteTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.scratch = tempfile.TemporaryDirectory()
        cls.binary = Path(cls.scratch.name) / "renderer"
        cls.environment = dict(os.environ, GOPROXY="off", GOTOOLCHAIN="local", GOSUMDB="off")
        subprocess.run(["go", "build", "-mod=readonly", "-o", str(cls.binary),
                        "./scripts/site"], cwd=ROOT, env=cls.environment, check=True)

    @classmethod
    def tearDownClass(cls):
        cls.scratch.cleanup()

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.docs = self.root / "docs"
        self.output = self.root / "output"
        self.docs.mkdir()

    def write(self, name, content):
        target = self.docs / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")

    def generate(self, success=True):
        result = subprocess.run(
            [str(self.binary), "--docs", str(self.docs), "--output", str(self.output),
             "--repository", "owner/project", "--revision", "abc123",
             "--base-url", "https://example.com/project/"],
            cwd=ROOT, text=True, capture_output=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def test_gfm_links_assets_and_anchors(self):
        fence = chr(96) * 3
        self.write("README.md", "# Wiki & Guide\n\n[Portal](portal/README.md)\n[Page](portal/page.md?q=one#hello-world)\n[Directory](portal/)\n[Code](../cmd/main.go#L2)\n[External](https://example.com/a.md)\n![Image](images/a%20b.svg)\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n- [x] Done\n\n~~Removed~~\n\n" + fence + "mermaid\ngraph TD; A-->B\n[Not a link](portal/page.md)\n" + fence + "\n")
        self.write("portal/README.md", "# Portal\n\n[Home](../README.md)\n")
        self.write("portal/page.md", "# Page\n\n## Hello *World*\n\n## Hello World\n\n## Café _name_\n\n[Self](#hello-world)\n[Asset](../images/a%20b.svg)\n[Source](../../go.mod)\n")
        self.write("images/a b.svg", "<svg/>")
        self.generate()
        html = (self.output / "index.html").read_text()
        for expected in ("<table>", "<del>Removed</del>", 'type="checkbox"',
                         'class="language-mermaid"', "[Not a link](portal/page.md)",
                         'href="portal/page.html?q=one#hello-world"',
                         'href="https://github.com/owner/project/blob/abc123/cmd/main.go#L2"',
                         'href="https://example.com/a.md"',
                         '<title>Wiki &amp; Guide — Solomon Wiki</title>',
                         'href="https://example.com/project/wiki/index.html"',
                         'aria-label="Wiki pages"', 'aria-current="page"'):
            self.assertIn(expected, html)
        self.assertEqual((self.output / "images/a b.svg").read_text(), "<svg/>")
        page = (self.output / "portal/page.html").read_text()
        self.assertIn('id="hello-world-1"', page)
        self.assertIn('id="café-name"', page)
        self.assertIn('href="https://github.com/owner/project/blob/abc123/go.mod"', page)
        for document in self.output.rglob("*.html"):
            for raw in Links(document.read_text()).targets:
                link = urlsplit(raw)
                if link.scheme or link.netloc:
                    continue
                target = (document.parent / unquote(link.path)).resolve() if link.path else document
                if target.name == "index.html" and not target.is_relative_to(self.output):
                    continue
                self.assertTrue(target.is_file(), (document, raw))
                if link.fragment and target.suffix == ".html":
                    self.assertIn(unquote(link.fragment), Links(target.read_text()).ids)

    def test_modification_and_deletion(self):
        self.write("README.md", "# Before\n")
        self.write("old.md", "# Old\n")
        self.write("asset.txt", "old")
        self.generate()
        self.write("README.md", "# After\n")
        (self.docs / "old.md").unlink()
        (self.docs / "asset.txt").unlink()
        self.generate()
        self.assertFalse((self.output / "old.html").exists())
        self.assertFalse((self.output / "asset.txt").exists())
        html = (self.output / "index.html").read_text()
        self.assertIn("After — Solomon Wiki", html)
        self.assertNotIn("old.html", html)

    def test_collision_preserves_existing_output(self):
        self.write("README.md", "# Index\n")
        self.generate()
        self.write("index.html", "collision")
        self.generate(success=False)
        self.assertIn("Index — Solomon Wiki", (self.output / "index.html").read_text())

    def test_overlapping_source_rejected(self):
        self.write("README.md", "# Safe\n")
        self.output = self.docs / "nested"
        self.generate(success=False)
        self.assertTrue((self.docs / "README.md").exists())

    def test_builder_rejects_symlink_output(self):
        outside = self.root / "outside"
        outside.mkdir()
        output = self.root / "linked"
        output.symlink_to(outside, target_is_directory=True)
        result = subprocess.run(
            ["python3", "scripts/build_site.py", "--output", str(output)],
            cwd=ROOT, env=self.environment, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(list(outside.iterdir()), [])

    def test_actual_site_build(self):
        output = self.root / "site"
        subprocess.run(["python3", "scripts/build_site.py", "--output", str(output)],
                       cwd=ROOT, env=self.environment, check=True, capture_output=True)
        self.assertIn("<style>", (output / "index.html").read_text())
        self.assertTrue((output / "getting-started/index.html").is_file())
        namespace = {"sm": "http://www.sitemaps.org/schemas/sitemap/0.9"}
        locations = [entry.text for entry in ET.parse(output / "sitemap.xml").findall(
            "sm:url/sm:loc", namespace)]
        expected = {
            "https://sapphir3-ros3.github.io/Solomon/",
            "https://sapphir3-ros3.github.io/Solomon/getting-started/",
        }
        for source in (ROOT / "docs").rglob("*.md"):
            relative = source.relative_to(ROOT / "docs")
            relative = (relative.with_name("index.html") if source.name == "README.md"
                        else relative.with_suffix(".html"))
            expected.add("https://sapphir3-ros3.github.io/Solomon/wiki/" + relative.as_posix())
        self.assertEqual(set(locations), expected)
        self.assertEqual(len(locations), len(expected))
        for document in (output / "wiki").rglob("*.html"):
            for raw in Links(document.read_text()).targets:
                link = urlsplit(raw)
                if link.scheme or link.netloc:
                    continue
                target = (document.parent / unquote(link.path)).resolve() if link.path else document
                self.assertTrue(target.is_file(), (document, raw))
                if link.fragment and target.suffix == ".html":
                    self.assertIn(unquote(link.fragment), Links(target.read_text()).ids,
                                  (document, raw))
        for source in (ROOT / "docs").rglob("*"):
            if not source.is_file():
                continue
            relative = source.relative_to(ROOT / "docs")
            if source.name == "README.md":
                relative = relative.with_name("index.html")
            elif source.suffix.lower() == ".md":
                relative = relative.with_suffix(".html")
            target = output / "wiki" / relative
            self.assertTrue(target.is_file(), str(relative))
            if source.suffix.lower() != ".md":
                self.assertEqual(target.read_bytes(), source.read_bytes())


if __name__ == "__main__":
    unittest.main()
