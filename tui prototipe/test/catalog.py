import fcntl
import os
import pty
import struct
import subprocess
import tempfile
import termios
from pathlib import Path
import unittest
from mouse import MouseTests, ROOT
from screen import Screen


class CatalogTests(MouseTests):
    def test_added_components(self):
        with tempfile.TemporaryDirectory(prefix="solomon-catalog-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                self.update_screen(master, screen, 0.6)
                for label, expected in (("File summary", "3 changed files"), ("Compaction", "Context compacted"), ("Reasoning", "thought for"), ("Worked for", "worked for 12s"), ("Subagent chat", "Inspect image preview autosave behavior"), ("Subagent activity", "2 subagents working"), ("Rate limit", "Usage limit reached · Resets in 30s"), ("Interrupted generation", "Generation interrupted"), ("Mode switch", "Switching to Agent"), ("Edit diff", "EditorPage.tsx"), ("Message actions", "Regenerate")):
                    self.click(master, screen, label)
                    self.assertIn(expected, screen.text())
                    if label == "Subagent activity":
                        self.assertIn("Verify editor regressions", screen.text())
                        self.click(master, screen, "subagents working")
                        self.assertNotIn("Verify editor regressions", screen.text())
                        self.click(master, screen, "subagents working")
                        self.assertIn("Verify editor regressions", screen.text())
                        self.click(master, screen, "Review image accessibility")
                        self.assertIn("Subagent chat", screen.text())
                        self.assertIn("Inspect image accessibility", screen.text())
                        self.assertNotIn("subagents working", screen.text())
                        for _ in range(3):
                            os.write(master, b"\x1b[<65;60;8M")
                            self.update_screen(master, screen)
                        self.assertIn("keyboard navigation", screen.text())
                        self.assertIn("Subagent chat", screen.text())
                        os.write(master, b"\x1b[<64;60;8M")
                        self.update_screen(master, screen)
                        self.click(master, screen, "[ × ]")
                        self.assertIn("subagents working", screen.text())
                        self.click(master, screen, "Verify editor regressions")
                        self.assertIn("Inspect image preview autosave behavior", screen.text())
                        self.assertNotIn("subagents working", screen.text())
                        self.click(master, screen, "[ × ]")
                        self.assertIn("subagents working", screen.text())
                self.assertNotIn("F1 Atelier", screen.text())
                self.assertNotIn("F3 Paper", screen.text())
            finally:
                process.kill()
                process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
