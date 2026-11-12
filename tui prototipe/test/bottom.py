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


class BottomTests(MouseTests):
    def test_bottom_button(self):
        with tempfile.TemporaryDirectory(prefix="solomon-bottom-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                self.update_screen(master, screen, 0.4)
                self.click(master, screen, "Transcript")
                self.assertIn("│ ↓ │", screen.text())
                self.click(master, screen, "│ ↓ │")
                self.assertNotIn("│ ↓ │", screen.text())
                os.write(master, b"\x1b[<64;40;15M")
                self.update_screen(master, screen)
                self.assertIn("│ ↓ │", screen.text())
                self.click(master, screen, "│ ↓ │")
                self.assertNotIn("│ ↓ │", screen.text())
            finally:
                process.kill()
                process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
