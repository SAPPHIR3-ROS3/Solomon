import base64
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


class NotificationTests(MouseTests):
    def test_notification_details(self):
        with tempfile.TemporaryDirectory(prefix="solomon-notification-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                self.update_screen(master, screen, 0.6)
                self.click(master, screen, "Notification")
                self.assertIn("Task completed", screen.text())
                x, y = screen.find("Task completed")
                self.assertGreater(x, 40)
                self.assertIn("System", screen.text())
                self.assertIn("10/25/2026", screen.text())
                x, y = screen.find("⧉")
                os.write(master, f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m".encode())
                copied = self.update_screen(master, screen)
                expected = base64.b64encode(b"Task completed | No files were changed while displaying this gallery.")
                self.assertIn(b"\x1b]52;c;" + expected + b"\x07", copied)
                self.assertNotIn("No files were changed", screen.text())
                self.click(master, screen, "Task completed")
                self.assertIn("No files were changed", screen.text())
                self.click(master, screen, "Task completed")
                self.assertNotIn("No files were changed", screen.text())
            finally:
                process.kill()
                process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
