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


class ModeTests(MouseTests):
    def test_mode_preview(self):
        with tempfile.TemporaryDirectory(prefix="solomon-mode-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                self.update_screen(master, screen, 0.5)
                self.click(master, screen, "Mode switch")
                self.assertIn("Switching to Agent mode", screen.text())
                before = screen.text()
                self.update_screen(master, screen, 0.6)
                self.assertNotEqual(before, screen.text())
                self.click(master, screen, "Stay in Chat")
                self.assertIn("Stayed in Chat", screen.text())
            finally:
                process.kill()
                process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
