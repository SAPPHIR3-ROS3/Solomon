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


class ToolGroupTests(MouseTests):
    def test_group_collapse(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tool-group-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen(120, 50)
            try:
                self.update_screen(master, screen, 0.6)
                self.click(master, screen, "Transcript")
                os.write(master, b"\x1b[F")
                self.update_screen(master, screen, 0.4)
                self.assertIn("Collapse tool calls", screen.text())
                self.click(master, screen, "Collapse tool calls")
                self.update_screen(master, screen, 2.0)
                self.assertIn("Show 6 tool calls", screen.text())
                self.click(master, screen, "Show 6 tool calls")
                self.assertIn("Collapse tool calls", screen.text())
            finally:
                process.kill()
                process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
