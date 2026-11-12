import fcntl
import json
import os
from pathlib import Path
import pty
import subprocess
import tempfile
import termios
import struct
import unittest
import terminal as terminal_support

ROOT = terminal_support.ROOT


class LiveTests(unittest.TestCase):
    drain = terminal_support.TerminalTests.drain

    def test_live_style_keys_and_invalid_recovery(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tui-live-") as folder:
            config = Path(folder) / "interface.json"
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(config), TERM="xterm-256color", COLORTERM="truecolor")
            subprocess.run([str(ROOT / "laboratory"), "--init-config"], env=env, check=True, capture_output=True)
            settings = json.loads(config.read_text())
            master, slave = pty.openpty()
            original = termios.tcgetattr(slave)
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission")], stdin=slave, stdout=slave, stderr=slave, env=env)
            try:
                self.drain(master, 0.5)
                pid = process.pid
                settings["text"]["gui_placeholder"] = "Live configuration applied"
                settings["text"]["workspace"] = "Live workspace"
                config.write_text(json.dumps(settings))
                output = self.drain(master, 0.6)
                self.assertIn(b"Live configuration applied", output)
                self.assertIn(b"Live workspace", output)
                os.write(master, b"DRAFT-ONE")
                self.drain(master)
                settings["keys"]["composer"]["send"] = ["ctrl+s"]
                settings["keys"]["composer"]["newline"] = ["ctrl+y"]
                settings["keys"]["global"]["new_thread"] = ["ctrl+t"]
                settings["layout"]["home_width"] = 76
                settings["layout"]["border"] = "heavy"
                config.write_text(json.dumps(settings))
                output = self.drain(master, 0.6)
                self.assertIn(b"DRAFT-ONE", output)
                self.assertIn("┏".encode(), output)
                self.assertEqual(process.pid, pid)
                self.assertIsNone(process.poll())
                os.write(master, b"\x19DRAFT-TWO\x13")
                output = self.drain(master, 0.4)
                self.assertIn(b"DRAFT-ONE", output)
                self.assertIn(b"DRAFT-TWO", output)
                self.assertIn(b"ctrl+s send", output)
                self.assertIn(b"ctrl+y newline", output)
                config.write_text("{")
                output = self.drain(master, 0.5)
                self.assertIn(b"Configuration rejected", output)
                os.write(master, b"\x14")
                output = self.drain(master)
                self.assertIn(b"Live workspace", output)
                config.write_text(json.dumps(settings))
                self.drain(master, 0.5)
                os.write(master, b"\x1b[18~")
                output = self.drain(master, 0.3)
                self.assertIn(b"COMPONENT MAP", output)
                os.write(master, b"\x1b[B\t")
                self.drain(master)
                os.write(master, b"\x1b")
                self.drain(master)
                os.write(master, b"\x03")
                self.drain(master)
                process.wait(timeout=3)
                self.assertEqual(process.returncode, 0)
                self.assertEqual(termios.tcgetattr(slave), original)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
