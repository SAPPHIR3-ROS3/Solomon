import importlib.util
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location('wiki', Path(__file__).resolve().parents[1] / 'scripts/wiki.py')
WIKI = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(WIKI)


class WikiTests(unittest.TestCase):
    def test_mirror(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / 'docs'
            (docs / 'guide').mkdir(parents=True)
            (docs / 'README.md').write_text('# Home\n[Guide](guide/README.md#setup)')
            (docs / 'guide/README.md').write_text('# Setup\n[Home](../README.md)\n[Code](../../code.go)\n![Image](../image.svg)\n[Remote](https://example.com)\n[Anchor](#setup)\n' + chr(96) * 3 + '\n[Example](../README.md)\n' + chr(96) * 3)
            (docs / 'image.svg').write_bytes(b'<svg/>')
            output = root / 'wiki'
            (output / '.git').mkdir(parents=True)
            (output / 'obsolete.md').write_text('old')
            self.assertEqual(WIKI.mirror(docs, output, 'owner/repo', 'abc'), 3)
            self.assertFalse((output / 'obsolete.md').exists())
            self.assertTrue((output / '.git').is_dir())
            self.assertEqual((output / 'image.svg').read_bytes(), b'<svg/>')
            self.assertIn('(guide--README#setup)', (output / 'Home.md').read_text())
            text = (output / 'guide--README.md').read_text()
            for expected in ['[Home](Home)', '/blob/abc/code.go', '(image.svg)', '(https://example.com)', '(#setup)', '[Example](../README.md)']:
                self.assertIn(expected, text)
            WIKI.mirror(docs, output, 'owner/repo', 'abc')
            self.assertEqual(text, (output / 'guide--README.md').read_text())

    def test_collision_and_overlap(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / 'docs'
            docs.mkdir()
            (docs / 'README.md').write_text('# Index')
            (docs / 'Home.md').write_text('# Collision')
            output = root / 'wiki'
            output.mkdir()
            marker = output / 'keep.md'
            marker.write_text('keep')
            with self.assertRaises(ValueError):
                WIKI.mirror(docs, output, 'owner/repo', 'main')
            self.assertTrue(marker.exists())
            with self.assertRaises(ValueError):
                WIKI.mirror(docs, docs / 'out', 'owner/repo', 'main')


if __name__ == '__main__':
    unittest.main()
